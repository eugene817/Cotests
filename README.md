# Cotests

[![CI](https://github.com/eugene817/Cotests/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/eugene817/Cotests/actions/workflows/ci.yml)

Cotests is a remake of ZawodyWeb with additional functionality, built with Go,
chi, HTMX, GORM, and SQLite (with optional PostgreSQL). The web application is
packaged as a single binary.

Currently implemented: accounts, sessions, contest/series administration,
published contest browsing, scoped organizer/participant memberships and series
schedules. The content backend provides revisioned authoring, private tests,
PDF delivery, task assignments, participant statement/sample access and bounded
ZIP preview/commit through `/api/v1`. See [HTTP_API.md](HTTP_API.md) for contracts;
the corresponding frontend screens remain a separate workstream.
The judge has only an interface
and a no-op implementation that returns “unavailable”; it executes nothing and
produces no results. The engine will be designed with the professor.
Submission persistence, rankings, and quizzes are planned.

## Quick start

```bash
go build -o cotests .
./cotests
```

The server starts on `http://localhost:3000`.

Create an administrator locally before exposing a fresh installation. The
password is requested without echoing it and is never passed as a command-line
argument.

```bash
./cotests admin create --email admin@example.com --name "Administrator"
```

Runtime state is explicit: `DATA_DIR` defaults to the current directory and
contains `cotests.db` when `DATABASE_URL` is absent. `ARTIFACT_DIR` defaults to
`DATA_DIR/artifacts`; these private files are never served from `/static`.
`MAX_ARTIFACT_BYTES`, `MAX_IMPORT_BYTES`, and `MAX_IMPORT_FILES` bound content
uploads/imports. `MAX_STORAGE_BYTES` defaults to 1 GiB for the private artifact
directory; deploy one web process per directory. Admins can invoke
`POST /api/v1/artifacts/gc` to collect old unreferenced files. `LISTEN_ADDR` defaults to
`:3000`. `PUBLIC_URL`, when set, must be an `http` or `https` origin and makes
cookies secure automatically for an HTTPS origin. Set `DATABASE_URL` to a
PostgreSQL DSN to use PostgreSQL instead of local SQLite.

```bash
DATABASE_URL="postgres://user:pass@localhost:5432/cotests" SECURE_COOKIES=true ./cotests
```

Schema upgrades run on startup and are recorded in `schema_migrations`. Before
upgrading a production copy, back it up and run the binary against a copy first.
The application refuses an upgrade that would add the session foreign key while
orphaned sessions exist, rather than deleting them.

## Verification

```bash
go test ./...
go vet ./...
go build -o cotests .
```

For PostgreSQL integration checks, provide a disposable database with permission
to create schemas. Tests create and drop only random `cotests_test_*` schemas.
CI provisions PostgreSQL 17; the full race suite has also passed locally against
PostgreSQL 17 through OrbStack's Docker-compatible API.

```bash
COTESTS_TEST_POSTGRES_DSN="postgres://user:pass@localhost:5432/cotests_test" go test -race ./...
```

## Stack

- **Go** 1.25+ — backend and single-binary packaging
- **chi** — HTTP router
- **HTMX** — frontend interactivity
- **GORM** — ORM and query layer
- **SQLite** (`github.com/glebarez/sqlite`) — default pure-Go database
- **PostgreSQL** (`gorm.io/driver/postgres`) — optional via `DATABASE_URL`
- **bcrypt** — password hashing

## Security

- Session identifiers are random 256-bit values. Only their SHA-256 hashes are stored in the database.
- Session cookies are HTTP-only, `SameSite=Lax`, and can be marked `Secure` with `SECURE_COOKIES=true`.
- All state-changing requests require a session-bound, HMAC-signed CSRF token;
  JSON and multipart requests send it in `X-CSRF-Token`.
- Public registration always creates a `user` account. Administrators are created locally with `cotests admin create`.

SQLite connection handling, operator-controlled administrator bootstrap,
bounded per-IP/account authentication attempts and a global bcrypt concurrency
budget are implemented. Static assets and liveness skip session loading;
embedded templates are excluded from public asset delivery.

## Design

Visual design follows *The Digital Atelier* system. Tokens are defined in `static/css/style.css`.

## Code structure and access rules

- `main.go`: configuration, dependency wiring, embedded UI and admin CLI.
- `internal/server`: HTML/HTMX and JSON handlers, sessions and request middleware.
- `internal/content`: scoped permissions, schedules and file/import transactions.
- `internal/db`: GORM models, queries and ordered, immutable migrations.
- `internal/artifact` / `internal/importer`: private files and bounded test ZIPs.
- `internal/auth`, `internal/config`, `internal/judge`: credentials, runtime
  configuration and the provisional judge interface/Noop.
- `static`: embedded templates and assets; only CSS/JS are publicly served.

Admins manage all contests and grant organizer roles. Organizers manage assigned
contests and their own problem libraries; participants read released content in
contests they belong to. Public schedules do not expose unreleased statements.
File downloads repeat the same resource checks. Statement release is separate
from submission eligibility; submission windows use UTC half-open intervals.

Published revisions and file bytes are immutable. Editing creates a new draft;
assignments pin a published revision. Future submissions must retain that revision
and ownership, and prevent destructive cascades. Add schema changes as new
migrations, preserving existing data. Backups must include both DB and artifacts.

## Project documents

- [ROADMAP.md](ROADMAP.md): remaining backend work, release checks and legacy/judge scope.
- [HTTP_API.md](HTTP_API.md): implemented routes, payloads, permissions and limits.
- [FRONTEND_TODO.md](FRONTEND_TODO.md): UI backlog and browser walkthrough checklist.
