# Project Context

Go backend for the **mphotos** photo service. Exposes a JSON API under `/api` (session-cookie
auth, owner + guest) and serves the generated image files. Photos originate in a Google Drive
folder: they are downloaded, EXIF-parsed and thumbnailed via
[mimage](https://github.com/msvens/mimage), then stored in Postgres + local disk.

Frontends live in sibling repos and talk to this over `/api`: `../mphotos-svelte` (SvelteKit,
active) and `../mphotos-ui` (Next.js, the original). Neither is served by this process — nginx
routes `/` to the frontend and `/api` here (:8050). There is no CORS handling; same-origin is
assumed.

# Commands

- Gate (run before committing): `make ci` # lint + build + vet + test
- Individually: `make lint` (golangci-lint) · `make build` · `make vet` · `make test`
- Run the server: `go run . service` (or bare `go run .` — the root command starts the server)
- DB lifecycle: `go run . db create` · `db upgrade` · `db version` · `db dump` · `db delete` (destructive)
- Image maintenance: `go run . photo generate` (regenerate crops/thumbs) · `photo clean` (drop
  image files no longer referenced in the DB)

**Tests need a live Postgres** — there is no skip guard, they fail if the DB is unreachable.
`config_test.yaml` expects `mphotos_test`/`mphotos_test`/`mphotos-test` on `localhost:5432` (CI
spins up exactly that). DAO tests drop and recreate all tables, so they are destructive and not
parallel-safe. `MPHOTOS_DB_HOST`, `MPHOTOS_DB_USER`, … override the file.

# Conventions

- **Routing**: stdlib `net/http.ServeMux` (Go 1.22 method+pattern routing). All routes are
  declared in one place, `routes()` in `internal/server/routes.go`. Register via the `mGET` /
  `mPUT` / `mDELETE` helpers, which auto-prefix `/api` — write patterns without the prefix
  (`s.mGET("/albums/{albumid}", ...)`). Note `mPUT` registers both PUT and POST. Read path
  params with `Var(r, "albumid")`; use `uid(r, "photoid", &id)` for UUIDs.
- **Handlers** return `(interface{}, error)` and are wrapped in exactly one middleware adapter
  (`internal/server/middelware.go`): `authOnly` (owner), `guestOnly` (passes guest uuid),
  `loginInfo` (public, passes a `loggedIn` flag so the handler can filter private data),
  `mResponse` (public, gets `w` — for cookies). Raw `http.HandlerFunc` only for binary/redirect
  responses (images, downloads, OAuth callbacks); those must check `ctxLoggedIn(r.Context())`
  themselves.
- **Responses** are always transport-200 with a `{data}` / `{error}` envelope; the real status
  lives in `error.code`. Never return `(nil, nil)` — it yields a 500. Build errors with the
  constructors in `errors.go` (`NotFoundError`, `BadRequestError`, …); raw DAO errors can be
  returned directly since `ResolveError` maps `sql.ErrNoRows` → 404 and `*googleapi.Error`.
- **Request decoding**: declare a local `type request struct` with `json:"x" schema:"x"` tags and
  call `decodeRequest(r, &params)` (picks JSON vs form by Content-Type). Multipart uploads
  bypass it.
- **Auth**: `gorilla/sessions` cookie store, `Path: /api`. Owner login is either a password or
  Google OAuth, selected by `config.AuthMethod()`; the frontend discovers which via
  `GET /api/auth/method`. Guests sign up by email and verify via an emailed code. Separate from
  both: the owner-only Drive/Gmail OAuth (`/api/drive/auth` → `/api/auth/callback`), whose token
  is persisted to `token.json` — that is infrastructure, not a login mechanism.
- **DAO** (`internal/dao/`): `sqlx` over `pgx/v5`, no ORM. `PGDB` holds one interface field per
  entity, each implemented by a `*XxxPG` in its own file. Structs live in `types.go`; column
  names are derived by lowercasing Go field names (`getStructFields` in `sqlx.go`), so field and
  column names must match case-insensitively. Build statements with `buildInsertNamed` /
  `buildUpdateNamed2`; ad-hoc queries are plain SQL with `$1` placeholders.
- **Schema changes are hand-rolled migrations** — no framework, but multi-step. To add version N:
  bump `DbVersion`/`DbDescription` in `types.go`; add `schemaV(N-1)toVN` (the ALTER/DDL delta) and
  rename the full-schema/teardown consts to `schemaVN`/`deleteSchemaVN` in `schema.go` (updating
  their references in `dao.go`'s `CreateTables`/`DeleteTables`); write `upgradeToVN(pgdb)` that
  applies only the delta — it must **not** stamp the version; and **append** `N: upgradeToVN` to the
  `migrations` map in `migrate.go` (never delete older entries — the whole chain is kept). `UpgradeDb`
  loops from the db's current version up to `DbVersion`, applying each registered step and stamping
  the version after each via `Version.Set`, so any older db walks forward one step at a time.
- **Config**: viper + YAML, accessed through free functions in `internal/config`. Searched in
  `$HOME/.mphotos`, `/etc/mphotos`, `.` (tests also `../..`). `config_example.yaml` documents
  every section — and is asserted by `internal/config/config_test.go`, so keep it in sync.
- **Logging**: zap, structured. No `fmt.Println` / `log.Printf`.
- Drive imports run async through a package-level worker (`jobChan` in `server.go`); schedule via
  `PUT /api/drive/job/schedule`, poll `GET /api/drive/job/{jobid}`.
- Known backend bugs found while porting the frontend are logged in `discovered-bugs.md`.

# Behavior Rules

- Ask before assuming when requirements are ambiguous
- Write minimum code to solve the stated problem — no preemptive abstraction
- Only modify files and functions directly involved in the current task
- Say "I'm not sure" when uncertain rather than confabulating
