# MR 1: content backend (B0–B4)

Implemented server contracts and payloads: [HTTP_API.md](HTTP_API.md).
Frontend screens are the next workstream; MR 2 covers B5–B8.

## Contract

The existing HTML routes remain supported. `/api/v1` provides cookie-authenticated
JSON operations for the frontend. JSON mutations use `X-CSRF-Token`; obtain it
from `GET /api/v1/csrf`. Uploads use multipart `file` and the same header. Unknown
JSON fields and trailing values are rejected. Errors are `{ "error": { "code":
"validation|unauthorized|forbidden|not_found|conflict|too_large|internal", "message":
"..." } }`. Lists accept `limit` (1–100, default 50) and `offset` (0+).

## Policy

* Only platform admins create/delete contests and grant/revoke organizers.
* Admins and a contest's organizers manage its settings, series and participants.
* An organizer can create problems and manage their own library; being an
  organizer does not grant access to another author's private library.
* Assigning a revision requires both contest-management and problem-management
  authority. Only published revisions can be assigned.
* Guests discover published contests. Released content is for contest members;
  admins and assigned organizers can preview unreleased content.
* Statements open at the later of contest start and series release (series start
  when release is unset). They remain readable after the contest ends.
* Submission eligibility (used in MR 2) requires participant membership and both
  contest and series half-open UTC windows `[start,end)`.
* Private test artifacts are only for authorized problem authors/admins. A sample
  or PDF can also be read through a released assignment. IDs/hashes grant no access.
* Published revisions, test references and file bytes are immutable. A new draft
  copies metadata and test references. Source language/checker names are metadata.

## Operations

`/api/v1/contests`: list/create; `/{id}`: get/update/delete;
`/{id}/members`: list/upsert; `/{id}/members/{userID}`: delete;
`/{id}/series`: list/create; `/{id}/series/order`: reorder.

`/api/v1/series/{id}`: update/delete; `/{id}/problems`: list/assign;
`/{id}/problems/{assignmentID}`: delete; `/{id}/problems/order`: reorder.

`/api/v1/problems`: list/create; `/{id}`: get/update;
`/{id}/revisions`: list/create draft. `/api/v1/revisions/{id}`: get/update;
`/{id}/publish`: publish; `/{id}/tests`: list/create;
`/{id}/tests/{testID}`: update/delete; `/{id}/pdf`: upload/replace;
`/{id}/imports`: ZIP preview; `/{id}/imports/{previewID}/commit`: atomic import.

`/api/v1/assignments/{id}` returns a released statement and sample references,
never hidden tests or checker metadata. `/api/v1/artifacts/{id}` streams a file
after resource authorization. `/api/v1/artifacts/gc` is an admin-only cleanup.

Preview records are owner/revision-bound, expire after one hour and commit once.
Artifact storage has per-file and total-byte bounds; GC uses a one-hour grace
period and removes only generated storage keys unreferenced by content/imports.

## Delivery checks

Each stage includes migration/HTTP/service checks. MR 1 must cover access isolation,
UTC boundaries, immutable publications, revision cloning, collision-free reorder,
ZIP preview/retry/failure, protected downloads and storage cleanup. Run
`go test -race ./...`, `go vet ./...`, `go build`, and PostgreSQL integration in CI.
No submission, account-lifecycle or judge execution work is part of this MR.

## Local verification (2026-10-21)

* `go test -race -count=1 ./...` passed on SQLite, including the complete content
  HTTP flow, time/access boundaries, publication races and import retries.
* After the final staging-file cleanup change,
  `go test -race ./internal/content ./internal/artifact` also passed.
* `go vet ./...`, `go build ./...` and `git diff --check` passed.
* PostgreSQL contract/migration tests are opt-in locally and enabled in CI with
  a PostgreSQL 17 service. Local execution was unavailable because the Docker
  daemon was not running; the PostgreSQL acceptance gate still needs CI evidence.
