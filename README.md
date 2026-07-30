# Cotests

Single-binary programming contest and quiz platform built with Go, chi, HTMX, GORM, and SQLite (with optional PostgreSQL).

## Quick start

```bash
go build -o cotests .
./cotests
```

The server starts on `http://localhost:3000`.

## Stack

- **Go** 1.25+ — backend and single-binary packaging
- **chi** — HTTP router
- **HTMX** — frontend interactivity
- **GORM** — ORM and migrations
- **SQLite** (`github.com/glebarez/sqlite`) — default pure-Go database
- **PostgreSQL** (`gorm.io/driver/postgres`) — optional via `DATABASE_URL`
- **bcrypt** — password hashing

## Design

Visual design follows *The Digital Atelier* system. Tokens are defined in `static/css/style.css`.

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the full development plan.
