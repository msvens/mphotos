# mphotos

mphotos is the backend service for a personal photo site. It exposes a JSON API for browsing and
managing a photo collection, and is tightly integrated with Google Drive.

**Goal**: *Once your images have been uploaded to your Google Drive they should be accessible
through your website.*

You point mphotos at a Drive folder; it downloads new images, extracts their metadata, generates
the derived sizes, and serves everything over `/api`. Frontends are separate projects —
[mphotos-svelte](https://github.com/msvens/mphotos-svelte) (SvelteKit, current) and
[mphotos-ui](https://github.com/msvens/mphotos-ui) (Next.js, the original). Neither is served by
this process; in production a reverse proxy routes `/` to the frontend and `/api` here.

## Features

- Long-lived connection to a remote Google Drive folder containing pictures
- (Automatic) download of new images to local storage; images can also be uploaded directly
- Full automation of
  - Image information extraction using [mimage](https://github.com/msvens/mimage)
  - Thumbnail and derived-size creation using [mimage](https://github.com/msvens/mimage)
    (thumb, square, portrait, landscape, resize — served alongside the original)
- Albums, cameras (auto-derived from EXIF, with editable metadata and a camera image)
- Guest accounts: visitors sign up with an email, verify via a mailed link, and can then like and
  comment on photos
- Owner login by password or Google OAuth (with an email allowlist), configurable per deployment

## Requirements

- Go 1.25+
- PostgreSQL
- A Google Cloud OAuth client (Drive + Gmail scopes) if you want Drive sync and guest
  verification emails

## Getting started

1. **Configure.** Copy `config_example.yaml` to `config.yaml` and edit it. mphotos looks for the
   file in `$HOME/.mphotos`, `/etc/mphotos`, and the current directory. Any value can be
   overridden by an environment variable, e.g. `MPHOTOS_DB_HOST`, `MPHOTOS_DB_PASSWORD`.

   The sections are `server` (api prefix, port, host), `service` (where images are stored),
   `auth` (`method: password` or `google`), `session` (cookie keys and name), `db`, and `google`
   (OAuth client for Drive/Gmail).

2. **Create the database.**

   ```sh
   go run . db create
   ```

3. **Run the server.**

   ```sh
   go run . service     # or just: go run .
   ```

   It listens on `server.port` (8050 by default) and serves everything under `server.prefix`
   (`/api`).

4. **Connect Google Drive.** With the owner logged in, visit `/api/drive/auth` to run the OAuth
   consent flow. The resulting token is persisted under the service root as `token.json` and
   reloaded on startup, so this is a one-time step. Then set the Drive folder on the user and use
   `/api/drive/upload` or schedule a background import job via `/api/drive/job/schedule`.

## CLI

The binary is a cobra app; running it with no subcommand starts the server.

| Command | Description |
| --- | --- |
| `service` | Start the API server |
| `db create` | Create the schema |
| `db upgrade` | Migrate an existing database to the current schema version |
| `db version` | Force the stored schema version to the current one |
| `db dump` | Dump photos or exif data as JSON |
| `db delete` | Drop all tables (destructive) |
| `photo generate` | Regenerate thumbnails and derived sizes for every photo |
| `photo clean` | Delete image files that are no longer referenced in the database |

## Development

```sh
make ci      # lint + build + vet + test — run this before committing
make lint    # golangci-lint
make test    # go test ./... -count=1
```

Tests require a running PostgreSQL — the DAO tests drop and recreate the schema, so point them at
a throwaway database. `config_test.yaml` expects user/password `mphotos_test` and database
`mphotos-test` on `localhost:5432`; CI provisions exactly that. The same values can be overridden
with the `MPHOTOS_DB_*` environment variables.

See `CLAUDE.md` for the internal architecture and code conventions.

## License

Apache 2.0 — see [LICENSE](LICENSE).
