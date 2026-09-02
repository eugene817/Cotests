# Cotests

[![CI](https://github.com/eugene817/Cotests/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/eugene817/Cotests/actions/workflows/ci.yml)

Single-binary programming contest and quiz platform built with Go, chi, HTMX, GORM, and SQLite (with optional PostgreSQL).

## Quick start

```bash
go build -o cotests .
./cotests
```

The server starts on `http://localhost:3000`.

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
- The first registered account is an `admin`; subsequent accounts receive the `user` role.

## Design

Visual design follows *The Digital Atelier* system. Tokens are defined in `static/css/style.css`.

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full development plan.
