# Content HTTP API — MR 1

Base path: `/api/v1`. JSON keys are snake_case. All timestamps are RFC3339;
authorization compares absolute UTC instants. Nullable metadata stays `null`.
Statements are plain source text, not trusted HTML.

## Session and request protection

Use the existing `/login` and `/logout` forms and the `session_token` cookie.
`GET /api/v1/csrf` works anonymously or authenticated and returns
`{"csrf_token":"..."}`, setting the CSRF cookie when necessary. Obtain a new
token after login/logout, session rotation or server restart. Tokens are HMAC
signed with a per-process secret and bound to the presented session cookie.

Send both cookies and `X-CSRF-Token` on **every mutation**. JSON bodies require
`Content-Type: application/json`; unknown fields and additional JSON values fail.
PDF/ZIP uploads require `multipart/form-data` with exactly one `file` part and
the CSRF header. The header avoids parsing multipart before authorization.
Same-origin requests may omit Origin; an explicitly different Origin is rejected.
`PUBLIC_URL` is the authoritative origin behind a proxy; forwarded headers are
not trusted for authorization or rate-limit addresses.

## Responses

Reads and creations return 200 plus an object/array; bodyless mutations return
204. Lists are arrays with `limit=1..100` (default 50), `offset=0..` and stable
ordering: descending ID for contests/problems, ascending position for series,
assignments/tests, descending number for revisions, ascending ID for members.
Related ORM objects, password hashes and private storage keys are never serialized.

Errors have this form:

```json
{"error":{"code":"forbidden","message":"Access denied"}}
```

| Status | Code | Meaning |
| --- | --- | --- |
| 400 | validation | Invalid fields, JSON, ZIP or pagination |
| 401 | unauthorized | Session required |
| 403 | forbidden | Missing permission or invalid CSRF/origin |
| 404 | not_found | Missing resource or mismatched parent ID |
| 409 | conflict | Immutable revision, duplicate identity/position, expired import or storage quota |
| 413 | too_large | Request or individual artifact exceeds its byte limit |
| 500 | internal | Operational failure, details only in server log |

## Inputs and routes

PUT replaces the listed editable fields; omitted optional fields become null or
empty. IDs, ownership, revision state/number and file references are server-owned.
They cannot be changed by adding JSON fields.

| Routes | Input / response |
| --- | --- |
| `GET, POST /contests` | POST: `title, description?, visibility` (`draft`/`published`), `start_at?, end_at?`. Admin creates. Returns contest. |
| `GET, PUT, DELETE /contests/{id}` | PUT: same fields. Manager updates, admin deletes. Discovery of published schedules is public. |
| `GET, PUT /contests/{id}/members` | PUT: `user_id, role` (`organizer`/`participant`). Returns 204. Only admin changes organizer membership. |
| `DELETE /contests/{id}/members/{userID}` | Revoke membership, 204. |
| `GET, POST /contests/{id}/series` | POST: `title, position, start_at?, end_at?, release_at?`. Returns series. Public list contains schedule metadata only. |
| `PUT /contests/{id}/series/order` | `{"ids":[3,1,2]}`: complete set exactly once, 204. |
| `PUT, DELETE /series/{id}` | PUT: same fields as series creation. |
| `GET, POST /series/{id}/problems` | POST: `revision_id, code, position`. Returns assignment. GET requires released-content access or management. |
| `DELETE /series/{id}/problems/{assignmentID}` | Remove only the assignment belonging to this series. |
| `PUT /series/{id}/problems/order` | Complete assignment IDs exactly once, 204. |
| `GET, POST /problems` | POST: `title, code, statement?, languages?, checker?`. Returns `{problem, revision}`. Library list is scoped to the author; admin sees all. |
| `GET, PUT /problems/{id}` | PUT: `title, code`. No ownership transfer. |
| `GET, POST /problems/{id}/revisions` | POST without body creates the next draft, copying latest metadata and test references. Returns revision. |
| `GET, PUT /revisions/{id}` | PUT: `statement, languages, checker?, time_limit_millis?, memory_limit_kib?`. Returns revision. |
| `POST /revisions/{id}/publish` | No body, 204. Requires nonempty statement or PDF. |
| `GET, POST /revisions/{id}/tests` | POST: `position, is_sample?, input, output, points?, time_limit_override_millis?`. Returns test with private artifact IDs. |
| `PUT, DELETE /revisions/{id}/tests/{testID}` | PUT: same test fields; replacement bytes get new immutable artifacts. |
| `POST /revisions/{id}/pdf` | Multipart PDF upload/replacement; returns artifact metadata. |
| `POST /revisions/{id}/imports` | Multipart flat `.in`/`.out` ZIP; returns `{id, expires_at, tests:[{name,input_bytes,output_bytes}]}` without inserting tests. |
| `POST /revisions/{id}/imports/{previewID}/commit` | No body, 204. Appends hidden tests in filename order. Repeating a committed preview on an editable revision succeeds without duplicates. |
| `GET /assignments/{id}` | Participant statement projection, described below. |
| `GET /artifacts/{id}` | Authorized attachment, GET range requests, `private, no-store`, `nosniff`. |
| `POST /artifacts/gc` | Admin only, returns `{"removed":N}`. |

`languages` is an array of up to 100 distinct nonempty identifiers (up to 100 bytes
each). Revision responses expose `language_metadata` as its stored JSON **string**
for compatibility with the existing model; e.g. `"[\"cpp\",\"python\"]"`.
The checker identifier is metadata only (up to 200 bytes). Limits must be positive,
points non-negative; positions are zero-based non-negative integers.
Titles: up to 300 bytes; problem/assignment codes: up to 64; statement: 512 KiB.

Example participant response:

```json
{
  "assignment_id": 1,
  "revision_id": 2,
  "title": "Sum",
  "code": "A",
  "statement": "Add two integers.",
  "pdf_artifact_id": null,
  "time_limit_millis": 1000,
  "memory_limit_kib": 65536,
  "language_metadata": "[\"cpp\"]",
  "samples": [{"position":0,"input_artifact_id":3,"output_artifact_id":4}]
}
```

This projection has no hidden-test IDs, checker configuration, scores or verdicts.
Artifact authorization is checked again when downloading; removal of membership
or a later release date immediately removes participant access to file URLs.

## Persistence and deployment

Migration 6 adds memberships, series dates and persistent import previews.
Migration 7 scopes assignment-code uniqueness to each series (different rounds
can both use `A`) without changing content. Existing series dates are
null; contest start still gates participant statement release.

`MAX_ARTIFACT_BYTES` defaults to 10 MiB; `MAX_IMPORT_BYTES` to 50 MiB compressed
and decompressed; `MAX_IMPORT_FILES` to 100 archive entries; `MAX_STORAGE_BYTES`
to 1 GiB. Upload limits cannot exceed 512 MiB or 10000 entries. JSON/form requests
have a 1 MiB cap; multipart envelopes get 64 KiB of overhead above their payload
limit. Storage quotas count committed immutable files, including retained previews
and orphans pending collection. Deploy one web process per artifact directory;
quota admission is serialized within that process.

ZIP previews belong to one actor and revision and expire after one hour. A commit
is a single DB transaction; failed operations compensate new files. A crash may
leave unreferenced files. Run the admin GC operation periodically: it expires old
preview references and removes generated files/metadata unreferenced by any
revision, test or retained preview, with a one-hour minimum grace period. It
never removes content merely because its owner account lost membership.

`GET /live` is DB-independent liveness; existing `GET /health` checks DB readiness.
Keep both data and artifacts in backups; the restore rehearsal is MR 2/B8.

## Verification

`internal/server/api_test.go` exercises the complete HTTP author/participant flow.
`internal/content/service_test.go` runs the policy, revision, concurrency, quota
and import contract on SQLite and, with `COTESTS_TEST_POSTGRES_DSN`, PostgreSQL.
CI provisions a disposable PostgreSQL service. The same DSN enables the existing
migration checks; each integration test creates/drops only its random schema.
