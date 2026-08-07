# Changelog

All notable changes to the mphotos backend are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
(pre-1.0: a minor bump carries features and breaking changes, a patch bump carries
fixes). Entries call out **Frontend impact** whenever a change needs matching work
in the `mphotos-svelte` / `mphotos-ui` frontends.

## [Unreleased]

### Added

- Multi-format photo import: accept JPEG, TIFF, PNG, GIF and BMP on both Drive and
  local upload. Non-JPEG sources are converted to JPEG, preserving embedded
  EXIF/XMP/IPTC where present (e.g. TIFF). The format is detected by content, not
  by the reported mime type or file extension.
  - **Frontend impact:** upload UIs can offer the new formats; worth messaging that
    non-JPEG originals are stored as JPEG.
- "No Camera" handling: photos with no camera EXIF (PNG/GIF/BMP, or a stripped
  JPEG) resolve to a single "No Camera" camera instead of a blank camera row, and
  are filterable like any other camera. A v7→v8 database migration backfills
  existing blank-camera data.

### Changed

- **Guest avatar URL** moved from `/api/guest/{guestid}/avatar[/{size}]` to
  `/api/guest/avatar/{guestid}[/{size}]`.
  - **Frontend impact:** update the avatar image `src` to the new path. Not yet
    consumed by either frontend, so no live breakage.
- `photo.sourceOther` now records the original image format (`jpeg`, `tiff`, …);
  previously it was unused.

### Fixed

- Server no longer panics at startup from a route-pattern conflict between the
  guest avatar and guest likes routes.

## [0.3.0] - 2026-08-07

Baseline release — changelog tracking starts here. This summarizes the major work
merged since v0.2.0 (2021); see the git history for the full detail.

### Added

- Guest accounts redesign: email one-time-code login, expiring signup,
  configurable session lifetime, and richer guest profiles with avatars.
- Server-side photo filtering on `/api/photos` and a public photostream.
- Multi-step, forward-only database migration framework.

### Fixed

- Guest email addresses are no longer leaked from the likes endpoint.
- Several camera handler defects and a swallowed DAO error.

[Unreleased]: https://github.com/msvens/mphotos/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/msvens/mphotos/releases/tag/v0.3.0
