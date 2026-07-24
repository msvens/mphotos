# Discovered backend bugs

A running log of backend bugs and limitations found while porting the frontend
(`../mphotos-svelte`) from the Next.js app. Each entry notes where it is, what goes wrong, and the
fix. Newest issues can be appended at the end.

## Status

| # | Issue | Status |
| --- | --- | --- |
| 1 | `handleCamera` reads the wrong path variable | **fixed** |
| 2 | `GET /api/photos` ignores filter and paging | **fixed** |
| 3 | `handlePhotos` swallows DAO errors | **fixed** |
| 4 | `GET /api/likes/{photoid}` leaks guest emails | **fixed** |
| 5 | `handleCameraImage` writes the response twice | **fixed** |
| 6 | Missing `return` after an error response in the image editor | **fixed** (PR #25) |
| 7 | Update handlers ignore the id in the URL | **fixed** |
| 8 | Photos in code-protected albums reachable by direct id | **closed — by design** |

All eight are resolved.

---

## 1. `handleCamera` reads the wrong path variable — FIXED

- **Where:** `internal/server/cameras.go:20` (`handleCamera`), route registered at
  `internal/server/routes.go:24` as `/cameras/{cameraid}`.
- **Symptom:** the handler calls `Var(r, "id")`, but the path parameter is named `cameraid`. `Var`
  is just `r.PathValue(name)` (`internal/server/middelware.go:141`), so it returns `""` and the
  handler runs `Camera.Get("")`. `GET /api/cameras/{id}` therefore never returns a single camera.
  (The image handlers correctly use `PathValue("cameraid")`.)
- **Fixed:** `id := Var(r, "cameraid")`.
- **Frontend impact:** the camera detail page works around this by finding the camera in the
  `GET /api/cameras` list instead of calling the single-camera endpoint.

## 2. `GET /api/photos` ignores its filter and paging — FIXED

- **Where:** `internal/server/photos.go:203` (`handlePhotos`).
- **Symptom:** it decoded a `{Limit, Offset}` request, but the `dao.Range` line was commented out
  and it called `s.pg.Photo.List()`, returning **all** photos regardless of `Limit`/`Offset`. It
  also never read a `cameraModel` (or any other) filter. The route was `authOnly`, so guests
  couldn't call it at all.
- **Fixed** across two PRs, which also reshaped the endpoint:
  - The route is now `loginInfo` (public): the owner gets all photos, a guest gets only the
    photostream album's members. This required promoting the photostream album id out of the client
    config blob into a typed `usert` column (`DbVersion` 4). See `MIGRATION.md`.
  - A new `PhotoPG.Select(filter, range, order)` (`internal/dao/photo.go`) honors `Limit`/`Offset`
    (with a stable `id` tie-breaker so paging can't drop/repeat rows), an `orderBy`, and equipment
    filters `cameraModel`/`cameraMake`/`lensModel`/`lensMake` (exact match, AND-combined). Built as
    a small dynamic-predicate helper rather than copying `AlbumPG.SelectPhotos`' two-places-in-sync
    pattern.
- **Frontend `getPhotosByCameraModel`** (which hits `/api/photos?cameraModel=`) is no longer dead —
  it now works for both owner and guest, so the client-side `.filter` on the camera page can go.
- **Also in the same statement:** #3 (swallowed `List()` error) — obsolete now, `List()` is no
  longer called here.

---

The following were found by a sweep of all handlers in `internal/server/`, prompted by the two
issues above. Every route pattern in `routes.go` was cross-referenced against the param names its
handler actually reads; #1 is the only path-variable mismatch in the package.

## 3. `handlePhotos` swallows DAO errors (shadowed error variable) — FIXED

- **Where:** `internal/server/photos.go:213`.
- **Symptom:** `if photos, e1 := s.pg.Photo.List(); err != nil` tests the *outer* `err`, not `e1`.
  The enclosing `if err := decodeRequest(...); err != nil` already returned on the error path, so
  `err` is provably nil here and the error branch is dead. A failing `Photo.List()` therefore
  falls through to the `else` and returns `&PhotoFiles{Length: 0, Photos: nil}` — a 200 with an
  empty collection instead of an error.
- **Fixed:** the condition now tests `e1`.

## 4. `GET /api/likes/{photoid}` leaks guest email addresses — FIXED

- **Where:** `internal/server/guest.go:150` (`handlePhotoLikes`), route `routes.go:83`.
- **Symptom:** the route is `loginInfo` (public), and the handler returns
  `Reaction.ListByPhoto()` verbatim. That returns `[]*GuestReaction`
  (`internal/dao/reaction.go:51`), and `GuestReaction` carries
  `Email string \`json:"email"\`` (`internal/dao/types.go:79`). The SQL explicitly selects
  `name,email,kind`. So **any anonymous caller can enumerate the email address of every guest who
  liked a photo.**
- **That this is unintended** is clear from the neighbouring comments handler
  (`guest.go:98`), which builds a deliberate local projection with only `Id/Name/PhotoId/Time/Body`
  precisely to keep the email out of the response.
- **Fixed** by removing `Email` from `GuestReaction` (`internal/dao/types.go`) and from the SELECT
  in `Reaction.ListByPhoto` (`internal/dao/reaction.go`). Chosen over the handler-level projection
  because `ListByPhoto` has exactly one caller and `GuestReaction` has no other consumer — it is a
  query-result DTO, not a domain entity. Removing the field makes any future attempt to serialize a
  guest email from this path a compile error.
- **Storage was not touched.** `GuestReaction` is not a table; the `guest` table keeps its
  `email TEXT NOT NULL UNIQUE` column, so `GetByEmail`/`HasByEmail` still resolve a returning guest
  to their original UUID and their comments and likes reattach as before.

## 5. `handleCameraImage` writes the response twice — FIXED

- **Where:** `internal/server/cameras.go:64-66`.
- **Symptom:** two `http.ServeFile` calls on the same `ResponseWriter`. The second uses
  `imgPath`, declared at `cameras.go:41` and never assigned — so it is `http.ServeFile(w, r, "")`
  after the real image has already been written. Produces superfluous-WriteHeader and duplicate
  header churn in the net/http logs.
- **Fixed:** removed the second `ServeFile` call and the unused `imgPath` declaration.

## 6. Missing `return` after an error response in the image editor — FIXED

- **Where:** `internal/server/img.go:98-101`.
- **Symptom:** `img.Open` failure writes a 500 via `http.Error` but does not return. Execution
  continues into `img.RotateImage` / `img.CropImage` / `imaging.Encode` with a nil `srcImage`,
  which will panic or write a second response body after the 500. Every other error branch in the
  same handler (`img.go:78,84,89,95,112`) returns correctly.
- **Fixed** in PR #25 (`Add missing return after http.Error in handleEditPreviewImage`), which was
  already open when this sweep independently rediscovered the bug.

## 7. Update handlers ignore the id in the URL — FIXED

- **Where:** `handleUpdateCamera` (`internal/server/cameras.go:30`, route `routes.go:25`),
  `handleUpdateAlbum` (`internal/server/albums.go:188`, route `routes.go:11`),
  `handleUpdatePhoto` (`internal/server/photos.go:224`, route `routes.go:68`).
- **Symptom:** all three took the target id solely from the decoded request body and never read
  the `{cameraid}` / `{albumid}` / `{photoid}` path variable. `PUT /api/cameras/A` with a body
  whose id is `B` updated `B` — the URL was decorative. The photos handler already carried an
  acknowledging comment at `photos.go:223`: *"add check that url path id is the same as the
  update id"*.
- **Impact:** these are all `authOnly`, so this is a correctness/API-contract issue rather than a
  privilege-escalation one.
- **Fixed** by making the path id authoritative rather than by comparing the two and erroring.
  Sending the id in both the path and the body is a normal thing for a client to do — it just
  serializes the object it already holds — so there is nothing to reject. The path names the
  resource, the body carries the new field values:
  - `handleUpdateCamera` overwrites `params.Id` with `Var(r, "cameraid")` after decoding.
  - `handleUpdateAlbum` fills `a.Id` via `uid(r, "albumid", &a.Id)`, so the existing `Has()`
    check and the update both run against the path id.
  - `handleUpdatePhoto` drops `Id` from its local `request` struct entirely and reads
    `uid(r, "photoid", &id)` — the body id is not merely ignored, it is never decoded.
- **API contract:** for all three, **the id in the request body is ignored**. Callers may keep
  sending it; it has no effect.
- **Frontend impact: none.** Both frontends were swept before the change. In `mphotos-ui` and
  `mphotos-svelte` alike, the album and photo update calls derive the path id and the body id
  from a single binding, so they could never disagree. Only `updateCamera(name, camera)`
  (`services/cameras.ts` in both) takes the two as independent arguments, and its only caller
  passes `camera.id` with `id` excluded from the editable fields. Every live caller therefore
  already sent a matching id, making this a no-op for them.
- **One narrowing worth knowing:** `decodeRequest` uses gorilla/schema for form-encoded bodies and
  the decoder does not set `IgnoreUnknownKeys`, so a **form-encoded** `PUT /api/photos/{id}`
  carrying an `id=` field is now rejected as an unknown key. Both frontends send
  `Content-Type: application/json`, where unknown fields are silently discarded, so nothing in
  practice hits this. Camera and album are unaffected — they decode into `dao.Camera` / `dao.Album`,
  which still have an `Id` field for the form decoder to bind.

## 8. Photos in code-protected albums are reachable by direct id — CLOSED, BY DESIGN

- **Where:** `/photos/{photoid}` (`routes.go:67`), `/photos/{photoid}/exif` (`routes.go:64`),
  `/photos/{photoid}/orig` (`routes.go:63`).
- **Symptom:** album access control is the `Album.Code` field (`internal/dao/types.go:17`; there
  is no `Photo.Private` — it is commented out at `types.go:112`). `/albums/{albumid}/photos`
  enforces the code (`albums.go:75`), but the direct per-photo endpoints are registered with
  `mResponse` / bare and do no album-code check. A photo in a code-protected album is therefore
  fetchable, including the original file, by anyone who knows or guesses its uuid.
- **Resolution: not a bug.** `Album.Code` was only ever meant to make certain images *a tad more
  difficult* to reach — never to hide them from public view. The same holds for photos in or out of
  the photostream, and for listing all images as a non-account user: you cannot enumerate them, but
  if you have the image URL you can always view it. That is the intended policy.
- Revisit only if the site later adopts genuinely private images. Even then the album code is an
  awkward mechanism for it, since it is just a shared code.
