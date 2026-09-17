# Cotests

[![CI](https://github.com/eugene817/Cotests/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/eugene817/Cotests/actions/workflows/ci.yml)

Cotests is a remake of ZawodyWeb with additional functionality, built with Go,
chi, HTMX, GORM, and SQLite (with optional PostgreSQL). The web application is
packaged as a single binary.

Currently implemented: accounts, sessions, contest/series administration, and
public browsing of active published contests. The judge has only an interface
and a no-op implementation that returns “unavailable”; it executes nothing and
produces no results. The engine will be designed with the professor.
Problem authoring, submission persistence, rankings, and quizzes are planned.

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

Set `DATABASE_URL` to a PostgreSQL DSN to use PostgreSQL instead of the local
`cotests.db` file. Set `SECURE_COOKIES=true` when serving the application over
HTTPS in production.

```bash
DATABASE_URL="postgres://user:pass@localhost:5432/cotests" SECURE_COOKIES=true ./cotests
```

## Verification

```bash
go test ./...
go vet ./...
go build -o cotests .
```

## Stack

- **Go** 1.25+ — backend and single-binary packaging
- **chi** — HTTP router
- **HTMX** — frontend interactivity
- **GORM** — ORM and migrations
- **SQLite** (`github.com/glebarez/sqlite`) — default pure-Go database
- **PostgreSQL** (`gorm.io/driver/postgres`) — optional via `DATABASE_URL`
- **bcrypt** — password hashing

## Security

- Session identifiers are random 256-bit values. Only their SHA-256 hashes are stored in the database.
- Session cookies are HTTP-only, `SameSite=Lax`, and can be marked `Secure` with `SECURE_COOKIES=true`.
- All state-changing forms require a CSRF token.
- Public registration always creates a `user` account. Administrators are created locally with `cotests admin create`.

SQLite connection handling and operator-controlled administrator bootstrap are
implemented. Authentication throttling remains pending; see [CODE_REVIEW.md](CODE_REVIEW.md).

## Design

Visual design follows *The Digital Atelier* system. Tokens are defined in `static/css/style.css`.

## Roadmap

Current phase: M0, partially complete. M2 will save submissions with the judge
disabled; M3 is the web MVP for authoring and submission collection. Full
replacement needs M4 web/content parity plus the deferred professor-led judge
work (J). See [MVP.md](MVP.md), [JUDGE.md](JUDGE.md), and
[FEATURE_PARITY.md](FEATURE_PARITY.md) for the scope and acceptance criteria.

See [ROADMAP.md](ROADMAP.md) for milestones and acceptance criteria,
[ARCHITECTURE.md](ARCHITECTURE.md) for the proposed stack and domain design,
[AUTH.md](AUTH.md) for account policies, and [CODE_REVIEW.md](CODE_REVIEW.md)
for verified findings and differences from the original ZawodyWeb.
