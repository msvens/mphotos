# Discovered backend bugs

A running log of backend bugs and limitations found while porting the frontend
(`../mphotos-svelte`) from the Next.js app. Each entry notes where it is, what goes wrong, and the
fix. Newest issues can be appended at the end.

## Status

| # | Issue | Status |
| --- | --- | --- |
| 1 | `handleCamera` reads the wrong path variable | open |
| 2 | `GET /api/photos` ignores filter and paging | deferred — a feature, not a bugfix |
| 3 | `handlePhotos` swallows DAO errors | open |
| 4 | `GET /api/likes/{photoid}` leaks guest emails | **fixed** |
| 5 | `handleCameraImage` writes the response twice | open |
| 6 | Missing `return` after an error response in the image editor | **fixed** (PR #25) |
| 7 | Update handlers ignore the id in the URL | deferred — behavior change, needs frontend check |
| 8 | Photos in code-protected albums reachable by direct id | **closed — by design** |

Working order for the open items: #3 first (it is observability — until it is fixed, a silent
`/api/photos` cannot be trusted to mean health), then #1 and #5.

---

## 1. `handleCamera` reads the wrong path variable

- **Where:** `internal/server/cameras.go:20` (`handleCamera`), route registered at
  `internal/server/routes.go:24` as `/cameras/{cameraid}`.
- **Symptom:** the handler calls `Var(r, "id")`, but the path parameter is named `cameraid`. `Var`
  is just `r.PathValue(name)` (`internal/server/middelware.go:141`), so it returns `""` and the
  handler runs `Camera.Get("")`. `GET /api/cameras/{id}` therefore never returns a single camera.
  (The image handlers correctly use `PathValue("cameraid")`.)
- **Fix:** `id := Var(r, "cameraid")`.
- **Frontend impact:** the camera detail page works around this by finding the camera in the
  `GET /api/cameras` list instead of calling the single-camera endpoint.

## 2. `GET /api/photos` ignores its filter and paging

- **Where:** `internal/server/photos.go:203` (`handlePhotos`).
- **Symptom:** it decodes a `{Limit, Offset}` request body, but the `dao.Range` line is commented
  out and it calls `s.pg.Photo.List()`, returning **all** photos regardless of `Limit`/`Offset`. It
  also never reads a `cameraModel` (or any other) filter. The route is `authOnly`
  (`routes.go:56`), so guests can't call it at all.
- **Consequence:** there is no working "photos filtered by camera model" endpoint. Camera-model
  filtering must go through the album-photos endpoint (`/api/albums/{albumid}/photos`, which does
  honor a `CameraModel` filter) or be done client-side. The frontend's `getPhotosByCameraModel`
  (which hits `/api/photos?cameraModel=`) is effectively dead and unused.
- **Fix (if paging/filtering is wanted):** decode and apply `Limit`/`Offset` (and optionally a
  `CameraModel` filter) via `Photo.List` with a range/filter instead of the argument-less call.
  Note `Photo.List()` (`internal/dao/photo.go:178`) takes no arguments, so this needs a new DAO
  method — `AlbumPG.SelectPhotos` (`internal/dao/album.go:135`) is the template to copy.
- **Also in the same statement:** see #3 below — the error from `List()` is silently swallowed.

---

The following were found by a sweep of all handlers in `internal/server/`, prompted by the two
issues above. Every route pattern in `routes.go` was cross-referenced against the param names its
handler actually reads; #1 is the only path-variable mismatch in the package.

## 3. `handlePhotos` swallows DAO errors (shadowed error variable)

- **Where:** `internal/server/photos.go:213`.
- **Symptom:** `if photos, e1 := s.pg.Photo.List(); err != nil` tests the *outer* `err`, not `e1`.
  The enclosing `if err := decodeRequest(...); err != nil` already returned on the error path, so
  `err` is provably nil here and the error branch is dead. A failing `Photo.List()` therefore
  falls through to the `else` and returns `&PhotoFiles{Length: 0, Photos: nil}` — a 200 with an
  empty collection instead of an error.
- **Fix:** test `e1`.

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

## 5. `handleCameraImage` writes the response twice

- **Where:** `internal/server/cameras.go:64-66`.
- **Symptom:** two `http.ServeFile` calls on the same `ResponseWriter`. The second uses
  `imgPath`, declared at `cameras.go:41` and never assigned — so it is `http.ServeFile(w, r, "")`
  after the real image has already been written. Produces superfluous-WriteHeader and duplicate
  header churn in the net/http logs.
- **Fix:** delete line 66 and the unused `imgPath` declaration.

## 6. Missing `return` after an error response in the image editor — FIXED

- **Where:** `internal/server/img.go:98-101`.
- **Symptom:** `img.Open` failure writes a 500 via `http.Error` but does not return. Execution
  continues into `img.RotateImage` / `img.CropImage` / `imaging.Encode` with a nil `srcImage`,
  which will panic or write a second response body after the 500. Every other error branch in the
  same handler (`img.go:78,84,89,95,112`) returns correctly.
- **Fixed** in PR #25 (`Add missing return after http.Error in handleEditPreviewImage`), which was
  already open when this sweep independently rediscovered the bug.

## 7. Update handlers ignore the id in the URL

- **Where:** `handleUpdateCamera` (`internal/server/cameras.go:30`, route `routes.go:25`),
  `handleUpdateAlbum` (`internal/server/albums.go:188`, route `routes.go:11`),
  `handleUpdatePhoto` (`internal/server/photos.go:224`, route `routes.go:68`).
- **Symptom:** all three take the target id solely from the decoded request body and never read
  the `{cameraid}` / `{albumid}` / `{photoid}` path variable. `PUT /api/cameras/A` with a body
  whose id is `B` updates `B`. The photos handler already carries an acknowledging comment at
  `photos.go:223`: *"add check that url path id is the same as the update id"*.
- **Impact:** these are all `authOnly`, so this is a correctness/API-contract issue rather than a
  privilege-escalation one.
- **Fix:** compare the path id to the body id and return `BadRequestError` on mismatch.

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
