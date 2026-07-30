# Migration Guide — Database v4 / Public Photo Stream

This release promotes the **photo stream album id** from the client-owned config
blob to a typed server column, and makes `GET /api/photos` a public endpoint that
serves the photostream to guests and the full library to the owner. It also adds
equipment filtering, paging and ordering to that endpoint.

It has two audiences:

- **Deploying the server** → [Server / deployment](#server--deployment). There is a
  required database migration; the server does **not** run it for you.
- **Updating a UI app** (`mphotos-ui`, `mphotos-svelte`) → [UI apps](#ui-apps).

> **Nothing breaks immediately.** The currently deployed UI keeps working against
> the migrated backend without any change (see [Backward compatibility](#backward-compatibility)).
> The UI changes below are how you *adopt* the new server-side behavior; they are
> not required just to keep the site running.

---

## Server / deployment

`DbVersion` goes from **3 → 4**. The migration adds one column
(`usert.photostreamalbumid`) and seeds it once from the existing config blob.

The running server neither auto-migrates nor version-gates on startup, and the new
binary's user queries select the new column — so a **new binary against an
un-migrated (v3) database will error on every user query**. Run the migration
before serving traffic.

### Steps

```sh
# 1. (recommended) back up first — any schema change deserves a dump
pg_dump ... > mphotos-pre-v4.sql

# 2. deploy the new (v4) binary, but do not start serving yet

# 3. run the migration with the NEW binary
./mphotos db upgrade      # v3 → v4: adds usert.photostreamalbumid, seeds from config

# 4. confirm
./mphotos db version      # should report 4

# 5. (re)start the service
```

Notes:

- The upgrade is guarded to a single step (v3 → v4 only) and is a single `ALTER
  TABLE` plus one `UPDATE`. It is fast.
- If the old config blob has no `photoStreamAlbumId`, the column is simply left
  empty — the migration does not fail.
- A fresh install (`db create`) already creates the v4 schema; no upgrade needed.

---

## UI apps

### What changed on the API

| Endpoint | Change |
| --- | --- |
| `GET /api/photos` | Now **public** (was owner-only). Logged-in owner → all photos; guest → photostream members (empty if unset). Honors paging + ordering. |
| `GET /api/user` | Now returns `photoStreamAlbumId` (owner only — blank for anonymous callers). |
| `PUT /api/user/photostream` | **New** owner-only setter for the photostream album id. |
| `GET /api/user/config` / `PUT /api/user/config` | Unchanged. The config blob still round-trips `photoStreamAlbumId`, but the server no longer reads it from there. |

### Backward compatibility

The deployed UI needs no immediate change because:

- `GET /api/user/config` still returns `photoStreamAlbumId` in the blob, so the old
  UI reads it exactly as before.
- The old UI only calls `GET /api/photos` while logged in as the owner, where the
  response is unchanged (all photos). Making the route public is a widening, not a
  break.
- `photoStreamAlbumId` on `GET /api/user` is a new field older parsers ignore.

### Required changes to adopt the new model

1. **Read** the photostream id from `GET /api/user` (`user.photoStreamAlbumId`)
   instead of from the config blob.
2. **Write** it via `PUT /api/user/photostream` (body `{ "photoStreamAlbumId":
   "<album-uuid>" }`; an empty string clears it) — point the UX-config album picker
   here instead of stuffing it into the config blob.
3. **Remove** `photoStreamAlbumId` from the `UXConfig` type / defaults.
4. **Fetch photos** through `GET /api/photos` for both owner and guest, dropping the
   `isUser` branch that chose between `getPhotos()` and `getAlbumPhotos(streamId)`.

### Transition caveat (dual source of truth)

Between deploying the v4 backend and shipping the UI change, the id lives in **two**
places: the config blob (old UI reads/writes) and the new column (server reads).
The migration seeds them in sync. If you change the photostream from the **old** UI
after deploy, it updates the blob but not the column — harmless today (nothing yet
uses the guest `/photos` path), but re-set it via `PUT /api/user/photostream` after
the UI is updated, or just deploy backend + UI together. Once every client reads
from `/api/user`, the `photoStreamAlbumId` key can be dropped from the config blob
(optional cleanup).

---

## `GET /api/photos` — endpoint reference

Public (session cookie optional).

- **Owner** (logged in): every photo.
- **Guest** (anonymous): members of the photostream album, or an empty list if no
  photostream is set.

### Query parameters

Decoded from the query string; keys are case-insensitive.

| Param | Type | Default | Meaning |
| --- | --- | --- | --- |
| `limit` | int | `0` | Page size. **`0` means no limit** (return everything) — this preserves the old behavior. |
| `offset` | int | `0` | Rows to skip. Negative is treated as `0`. |
| `orderBy` | int enum | `0` | Sort order (see below). |
| `cameraModel` | string | — | Exact-match equipment filter, applied only when non-empty. |
| `cameraMake` | string | — | Exact-match equipment filter. |
| `lensModel` | string | — | Exact-match equipment filter. |
| `lensMake` | string | — | Exact-match equipment filter. |

Multiple filters combine with AND. On the guest view they apply within the
photostream (e.g. `?cameraModel=Leica%20M11` returns only photostream photos shot
with that model).

`orderBy` values (the `PhotoOrder` enum):

| Value | Order |
| --- | --- |
| `0` | None → defaults to newest upload first |
| `1` | Upload date, newest first |
| `2` | Original (capture) date, newest first |
| `3` | Manual album order → not meaningful here; treated as upload date |

Ordering always applies a stable `id` tie-breaker, so `limit`/`offset` paging never
drops or repeats a row across pages.

> **Only send parameters this backend version declares.** Unknown query keys are
> rejected with a 400, so send only the parameters listed above.

### Response

Standard `{ data }` / `{ error }` envelope (always transport-200; the real status is
in `error.code`). On success:

```json
{ "data": { "length": 12, "photos": [ { "id": "...", "title": "...", ... } ] } }
```

`length` is the size of the returned page, **not** a grand total — there is no total
count yet, so compute "has more" from whether a full page came back.

---

## Server-side filtering

Equipment filters (`cameraModel`, `cameraMake`, `lensModel`, `lensMake`) are live —
see the query-parameter table above. They are exact-match, combine with AND, and
apply on both the owner and guest views (a guest filter is scoped to the
photostream).

For the UI, this means the camera page can call
`GET /api/photos?cameraModel=<model>` for both audiences and drop the client-side
`.filter(p => p.cameraModel === model)` along with the `isUser` fetch branch.

---

# Migration Guide — Database v5 / Guest Auth Redesign

This release hardens and enriches the guest flow. Unlike the v4 change, the guest
API **is breaking**: name+email login is gone, replaced by email one-time-code
login and an expiring signup link. The currently deployed `mphotos-ui` guest
sign-in **will stop working** once this is deployed — that is expected and
accepted; guest sign-in returns when `mphotos-svelte` adopts the new flow.
Comments and likes stay readable throughout.

## Server / deployment

`DbVersion` 4 → 5. As before, the server does **not** auto-migrate.

```sh
pg_dump ... > mphotos-pre-v5.sql     # habit before any schema change
./mphotos db upgrade                  # v4 -> v5
./mphotos db version                  # should report 5
```

`upgradeToV5` adds `fullname`/`description` to `guest`, creates the `guestcode`
table, and **grandfathers every existing guest to `verified=true`**. That last
step is essential: under the old flow the verified flag was unenforced, so legacy
guests were active regardless. It keeps every guest, lets them all log in via the
new email→code flow, and stops the new reaper from deleting them.

**Data preservation:** existing guest rows, comments and likes are untouched;
existing guest cookies keep working (same keys, same 30-day codec window, and
grandfathered guests pass the new verified check).

**New config key:** `server.secureCookies` (bool). Set it **`true` in production**
(cookies are served over HTTPS behind nginx); `false` for local http dev. It adds
the `Secure` flag to all session cookies. `SameSite=Lax` is now set in code.

## Guest API — what changed

| Endpoint | Change |
| --- | --- |
| `PUT /api/guest` | Signup only. Body `{email, name, fullName?, description?}`. Creates a **pending** guest and emails a verification **link**; **no cookie** until it is clicked. Returning verified email → "please log in". |
| `GET /api/guest/verify?code=<token>` | Activates from the emailed single-use token (was the guest's uuid), then signs in. |
| `PUT/POST /api/guest/login` | **New.** Body `{email}`. Emails a 6-digit code to a verified guest; always returns a neutral message (never leaks whether the email exists). |
| `PUT/POST /api/guest/login/verify` | **New.** Body `{email, code}`. Consumes the code (single use) and signs in. |
| `PUT /api/guest/update` | Body is now `{name, fullName?, description?}`. Email is immutable (it's the login identity). |
| `GET /api/guests` | **New, owner-only.** Full guest records incl. `fullName`/`email`. |
| Comments / likes responses | Now carry the guest `description` alongside `name` (public). `fullName` and `email` are never in public responses. |

Cookie: guests now expire after **30 days** and must re-login (email→code). The
read path enforces `verified`. Login/signup email delivery needs the owner's Gmail
OAuth token (same mechanism as before).

## UI adoption (`mphotos-svelte`)

The new guest experience:
- **Signup:** collect email + nickname (+ optional full name / description) → `PUT /api/guest` → show "check your email" → the link lands on the verify page which calls `GET /api/guest/verify?code=…`.
- **Login (returning guest):** email → `PUT /api/guest/login` → prompt for the 6-digit code → `PUT /api/guest/login/verify`. No name needed.
- **Profile:** `full name` + `description` fields; `GET /api/guest` returns them for the guest's own view. Show a guest's `description` next to their comments/likes (now in those responses).
- **Session:** expect a 30-day expiry — handle the `guestOnly` 401 by prompting a fresh code login.

`mphotos-ui` is intentionally left on the old (now-broken) guest flow.
