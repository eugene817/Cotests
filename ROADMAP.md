# Cotests Roadmap

A single-binary programming contest and quiz platform built with Go, chi, HTMX, GORM, and SQLite (with optional PostgreSQL).

## Legend

- `[x]` Completed
- `[-]` In progress
- `[ ]` Pending

---

## Phase 1 — Foundation & Database (Weeks 1-2)

Goal: application skeleton, router, database, authentication, and base UI.

`[x]` **Session 1 — Project skeleton**
- Initialize Go module (`go mod init cotests`).
- Install `chi` router (`github.com/go-chi/chi/v5`).
- Single HTTP server listening on `:3000`.
- Serve CSS, JS, and HTMX via `//go:embed` so everything lives in one binary.

`[x]` **Session 2 — Database setup**
- Connect to SQLite using `github.com/glebarez/sqlite` (pure Go, no CGO).
- Optionally switch to PostgreSQL via `DATABASE_URL` using `gorm.io/driver/postgres`.
- Implement `db.Open(dsn)` auto-detection and GORM `AutoMigrate` plumbing.

`[x]` **Session 3-4 — Users and sessions**
- Create `User` and `Session` models.
- Add password hashing with `golang.org/x/crypto/bcrypt`.

`[x]` **Session 5-6 — Authentication & authorization**

> Detailed plan: [AUTH.md](./AUTH.md)

- Backend registration, login, logout, CSRF validation, and session cleanup.
- Issue HTTP-only, SameSite session cookies and store token hashes only.
- Add `Role` field to `User` (`admin` / `user`) and promote the first user to `admin`.
- Implement chi middleware: load identity, require login, and require roles.

`[x]` **Session 7 — Base UI**
- Shared `layout.html` with header and navigation.
- Role-aware navigation links and an admin dashboard placeholder.
- Base `home.html`, `auth_form.html`, and HTMX-compatible access-denied fragments.

`[x]` **Phase 1 verification**
- Shared test helpers provide isolated SQLite databases, users, sessions, HTTP forms, and CSRF cookies.
- Automated checks: `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go build`.
- GitHub Actions runs `go mod download`, `go vet ./...`, and `go test -race ./...` for pull requests and pushes to `main`.
- Manual check on a clean SQLite database: registration, first-user admin promotion, guest redirect from `/admin`, and admin dashboard access.

---

## Phase 2 — Interface & Content Draft (Weeks 3-5)

Goal: a usable admin and participant interface for contest content. This phase does
not execute submissions or store test cases; the submit action shows an explicit
"judging is not available yet" placeholder.

`[ ]` **Session 8-9 — Contests and Series**
- `Contest` model (title, description, start/end dates, visibility).
- `Series` model (title, order) belonging to a contest.
- Set up hierarchy and cascade deletes.
- Add SQLite-backed model and migration tests for constraints, ordering, and cascade deletion.
- Exit criterion: an admin can create a contest with ordered series through the data layer; non-admin access remains denied.

`[ ]` **Session 10-11 — Admin HTMX CRUD for Contests & Series**
- List views and create/edit/delete forms using HTMX fragments.
- Admin-only routes protected by middleware.

`[ ]` **Session 12-13 — Tasks**
- `Task` model with metadata: RAM/CPU limits, allowed languages, points.
- Belongs to a `Series`.

`[ ]` **Session 14-15 — Task creation UI**
- HTMX form for creating/editing tasks.
- PDF statement upload stored as a BLOB column in SQLite.
- Dedicated route to serve the PDF back to the browser.

`[ ]` **Session 16-17 — Participant interface and judge placeholder**
- Public contest and series lists respecting visibility and active dates.
- Task page with statement, limits, points, and available languages.
- Submit form with a language selector and source-code textarea.
- A CSRF-protected HTMX endpoint returns a clear placeholder: the solution is not saved or judged until Phase 3.
- Exit criterion: both roles can complete their content-browsing workflows, while no user can mistake the placeholder for a verdict.

---

## Phase 3 — MainJudge Engine (Weeks 6-9)

Goal: compile, execute, and judge user submissions safely.

`[ ]` **Session 18-19 — Tests, submissions, and worker pool**
- `Test` model: input, expected output, points, per-test time/memory limits.
- Admin-only UI for adding and editing test cases.
- `Submission` model: code, language, status, runtime, memory, score.
- Goroutine + channel worker pool for processing the submission queue.

`[ ]` **Session 20-22 — Sandbox**
- Compile and run submissions with `os/exec`.
- Use `context.WithTimeout` for execution timeout.
- Support compilers/interpreters like `g++` and `python3`.

`[ ]` **Session 23-24 — Metrics**
- Measure execution time (wall clock).
- Measure peak memory via `rusage` / `MaxRSS`.

`[ ]` **Session 25-27 — Checkers**
- Define `CheckerInterface`.
- Implement `ExactDiff` (byte-exact output comparison).
- Implement `NormalDiff` (ignore leading/trailing whitespace).

`[ ]` **Session 28-29 — End-to-end pipeline**
- Compile → run on each test input → pass output to checker → write verdict to DB.
- Verdicts: Pending, Running, Accepted, Wrong Answer, Time Limit Exceeded, Memory Limit Exceeded, Runtime Error, Compilation Error.

---

## Phase 4 — Submission Experience (Weeks 10-12)

Goal: replace the Phase 2 placeholder with a complete participant submission experience.

`[ ]` **Session 30-31 — Connect submissions to the judge**
- Replace the placeholder endpoint with submission persistence and queueing.
- Preserve the Phase 2 task page and submit form.

`[ ]` **Session 32-33 — Submission history**
- Show a participant's previous submissions, verdicts, score, time, and memory.

`[ ]` **Session 36-38 — Live status updates**
- HTMX polling (`hx-trigger="every 2s"`) or Server-Sent Events (SSE) in Go.
- Show status transitions: Pending → Running → Accepted / Wrong Answer / etc.

---

## Phase 5 — Scoring & Quizzes (Weeks 13-14)

Goal: aggregate scores and support non-code quiz tasks.

`[ ]` **Session 39-40 — Score aggregation**
- SQL queries that compute the maximum score per task per user.
- Cache or compute total contest/series scores.

`[ ]` **Session 41-42 — Leaderboards**
- Global rating page.
- Per-contest and per-series rating pages.

`[ ]` **Session 43-44 — Quiz task type**
- New task kind that accepts a text answer instead of source code.
- Special `QuizDiff` checker supporting exact match or regex match.
- Adjusted submission UI for quiz answers.

---

## Phase 6 — Admin Bulk Operations (Weeks 15+)

Goal: reduce repetitive admin work via import, clone, and re-judge.

`[ ]` **Session 45-47 — ZIP import**
- Parse ZIP archives using `archive/zip`.
- Infer folder structure (`tests/1.in`, `tests/1.out`, statements, etc.).
- Auto-create tasks, tests, and file attachments.

`[ ]` **Session 48 — Task cloning**
- Deep copy a task record including its tests and statement blob.

`[ ]` **Session 49 — Re-judge**
- Admin button to reset submission statuses and re-enqueue them in the worker pool.

---

## Notes

- All static assets are embedded; the final deliverable is a single `cotests` binary.
- SQLite is the default database for simplicity; PostgreSQL is available via `DATABASE_URL`.
- Design system: *The Digital Atelier* — tokens live in `static/css/style.css`.
