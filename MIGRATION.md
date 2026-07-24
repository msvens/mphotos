# Migration Guide — Database v4 / Public Photo Stream

This release promotes the **photo stream album id** from the client-owned config
blob to a typed server column, and makes `GET /api/photos` a public endpoint that
serves the photostream to guests and the full library to the owner. It also adds
paging and ordering to that endpoint.

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
> rejected with a 400. In particular, do **not** send `cameraModel` etc. yet — see
> the next section.

### Response

Standard `{ data }` / `{ error }` envelope (always transport-200; the real status is
in `error.code`). On success:

```json
{ "data": { "length": 12, "photos": [ { "id": "...", "title": "...", ... } ] } }
```

`length` is the size of the returned page, **not** a grand total — there is no total
count yet, so compute "has more" from whether a full page came back.

---

## Server-side filtering — next backend release

Equipment filters are **not in v4**. The next backend release adds these query
parameters to `GET /api/photos`, all exact-match:

- `cameraModel`
- `cameraMake`
- `lensModel`
- `lensMake`

They work on both the owner and guest views (a guest filter is applied within the
photostream). Until that release ships, sending any of them returns a 400, so keep
doing camera-model filtering client-side for now. This section will be updated to
"available" when the filtering release lands.
