# Code and legacy review

Review date: 2026-09-08.

Cotests reviewed at `60e51c524c4cd643f9cc6b2d9424e492e8b510ca`; original ZawodyWeb inspected at `0d1e68e4cda77b539774f95ce87808bf0966cb17`.

The working tree was clean before the original review. The follow-up branch refine-plan-fixes now contains planning updates, the empty judge interface/Noop, and the minor fixes marked below. Bootstrap, throttling and broader architecture changes remain pending. Original line references refer to the reviewed commit above.

## Findings, in priority order

### P1 — SQLite foreign keys disappear after connection replacement

**Fixed on refine-plan-fixes.** The required pragma is appended to the DSN for every connection, preserving caller options. [connection_test.go](internal/db/connection_test.go) covers replacement, cascading deletion, invalid parents and preserved caller settings.

Location: [internal/db/db.go](internal/db/db.go), lines 39–46.

`PRAGMA foreign_keys = ON` is executed once, while `SetConnMaxIdleTime(5 * time.Minute)` lets the pool discard that connection. A replacement uses the driver's default with foreign-key enforcement disabled. Deleting a contest can then leave orphaned series; invalid parent references can also be accepted.

Verified with a temporary file-backed database using the actual `db.Open` and migrations: create a contest and series, close the idle connection through the pool, query the pragma, then delete the contest.

```text
foreign_keys before=1 after=0; orphan series after contest deletion=1
```

The temporary diagnostic probe was removed after the original verification; the fix now includes a permanent file-backed regression test. Merely disabling idle expiry would not cover all replacement paths. The pinned driver documents per-connection configuration. [Driver foreign-key configuration](https://github.com/glebarez/sqlite#foreign-key-constraint-activation)

### P1 — Public registration can claim the first administrator account

Location: [internal/db/users.go](internal/db/users.go), lines 17–25; [internal/server/router.go](internal/server/router.go), lines 35–37.

On an empty installation, any visitor completing registration first receives full administrator access. The server binds to `:3000`, so a fresh deployment exposed before its owner registers can be taken over. This is current documented behavior, but it is an unsafe production bootstrap policy.

Replace it with an explicit operator-only bootstrap command or a one-use setup credential. Ordinary registration must always create an ordinary account. Preserve existing admins during migration. Serializable isolation addresses transaction ordering; it does not identify the intended operator.

### P2 — Authentication has no attempt or concurrency limits

Location: [internal/server/router.go](internal/server/router.go), lines 24–38; [internal/server/handlers.go](internal/server/handlers.go), `Register` and `Login`.

A client can obtain its own CSRF token and repeatedly call login/registration. Existing-account password attempts and registrations consume bcrypt work without application throttling; login attempts are also unlimited. No reverse-proxy protection is configured in this repository.

Add bounded per-account and per-IP attempts plus a global authentication work budget, with trusted-proxy handling and generic failure responses. Avoid account-only hard lockouts that let attackers deny access to other users. OWASP recommends login throttling and careful lockout design. [Authentication guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)

### P2 — Published contest discovery is tied to the submission window

Location: [internal/db/contests.go](internal/db/contests.go), lines 34–55.

The list and detail queries exclude both future and ended contests. A published future contest cannot advertise its schedule, and a bookmarked contest becomes 404 after it ends. This matches the existing roadmap's “active dates” behavior but conflicts with the broader requested schedule, history and progress workflows.

Separate publication/discovery from statement release, submission eligibility and result visibility. Keep upcoming and archived pages available according to policy; do not expose hidden statements merely by changing these queries.

### P3 — Creating the first contest leaves the empty-state message visible

**Fixed on refine-plan-fixes.** Successful creation renders the complete contest_list fragment with an innerHTML replacement. The initial page uses the same fragment, so the obsolete empty state is removed. HTTP regression expectations are updated; a live browser walkthrough remains separate verification.

Location: [internal/server/admin.go](internal/server/admin.go), `CreateContest`; [static/templates/admin.html](static/templates/admin.html), lines 42–49.

The initial list contains “No contests yet.” Successful HTMX creation prepends only a `contest_card`, so the empty-state element remains under the new contest. Replace the list fragment or explicitly remove the empty state after success. Cover this with a browser/DOM flow; the current response-fragment tests do not model the resulting page.

## Design and maintenance observations

- Positive foundation: password hashing, random session tokens with hashes at rest, cookie flags, CSRF checks on current POST handlers, admin route guards, series-parent ownership checks, form size limits, escaping templates, graceful shutdown, and meaningful HTTP/DB tests.
- The `template.HTML` conversion currently wraps output already rendered by html/template. That alone is not evidence of a user-content XSS bug. Future Markdown or imported HTML must pass a sanitization policy.
- CSRF uses an unsigned double-submit cookie and is repeated in individual handlers. Before production, centralize protection and adopt a reviewed signed/session-bound design with origin validation. Cookie injection is a deployment-dependent concern, not a demonstrated exploit here. [OWASP CSRF guidance](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)
- Authentication middleware also runs for static files and health requests, adding DB lookups for authenticated asset requests. Scope identity loading to routes that use it.
- Most tests use in-memory SQLite; the new connection regression also uses a file-backed database. There is no PostgreSQL integration job. Current “optional PostgreSQL” support has not established concurrent bootstrap, migration and constraint behavior on PostgreSQL.
- `Session.UserID` has an index but no declared GORM relationship/foreign key. Add referential integrity and session-expiry indexing as account management grows.
- Current hard deletion of contest/series is acceptable for empty drafts. Extending cascades to submissions and results would destroy grading history; set retention rules before adding those models.
- Position uniqueness prevents directly swapping two occupied series positions. Add a transactional reorder operation using collision-free temporary positions, rather than requiring organizers to invent intermediate values.
- Fixed the password form/error mismatch: both now describe the existing 8–72 UTF-8 byte rule, the conflicting browser character minimum is removed, and multibyte boundaries are covered. Existing hashes and backend validation semantics are unchanged. Labels/autocomplete are also corrected.
- Templates are embedded under `static/` and the whole subtree is served. They contain no identified secrets today, but should move outside the public asset root.
- `AUTH.md` previously described a placeholder admin page and middleware.go that no longer match the implementation. The AGENTS file structure also predates internal/server; update operational instructions when the corresponding implementation changes.
- `cotests.db` is relative to the process working directory, not inherently adjacent to the executable. Introduce an explicit data directory and document backup contents.
- Existing CSS already has mobile overrides, but some target inline style strings. Move layouts to named classes and verify actual browser flows; this review did not perform visual QA.

## Corrections to the legacy feature summary

The original has useful domain behavior, but its implementation should not be copied literally.

| Topic | What the inspected source does | Remake implication |
| --- | --- | --- |
| Checker contract | `check(Program, TestInput, TestOutput)`; built-ins call `program.runTest(input)` themselves | Separate execution and output comparison in the remake. |
| ExactDiff | Compares Java strings character by character | A new byte-exact checker needs explicit encoding/newline rules for imported tasks. |
| NormalDiff | Normalizes whitespace runs, including internal separators | Trimming only the ends, as the old Cotests roadmap proposed, is not equivalent. |
| TrailingDiff | Trims both ends of each line and permits extra trailing blank lines | Do not rename it to “ignore trailing spaces” and silently change acceptance. |
| QuizDiff | Parses numbered answers, lowercases keys/answers, awards proportional rounded integer points, uses ACC for any positive score | It is not a generic answer-regex matcher; preserve its partial-credit behavior where required. |
| qtype | Controls public/private visibility of contest questions/clarifications | Model clarifications separately from quiz task types. |
| Classes | Stores class filename, version, type and executable bytecode | It is an executable plugin registry, not educational categorization. |
| ZIP | Uses XML manifests including contest.xml, serie.xml, problem.xml and tests.xml, with referenced input/output files | A directory-only .in/.out importer cannot claim ZawodyWeb compatibility. |
| Series | Includes start/end, freeze/unfreeze and IP-related settings | Carry these into explicit schedule, ranking and access policies. |

Evidence: [checker contract](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/judge-commons/src/main/java/pl/umk/mat/zawodyweb/judge/commons/CheckerInterface.java), [ExactDiff](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/judge-classes/src/main/java/pl/umk/mat/zawodyweb/checker/classes/ExactDiff.java), [NormalDiff](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/judge-classes/src/main/java/pl/umk/mat/zawodyweb/checker/classes/NormalDiff.java), [TrailingDiff](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/judge-classes/src/main/java/pl/umk/mat/zawodyweb/checker/classes/TrailingDiff.java), [QuizDiff](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/judge-classes/src/main/java/pl/umk/mat/zawodyweb/checker/classes/QuizDiff.java), [clarification access](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/www/src/main/java/pl/umk/mat/zawodyweb/www/RequestBean.java#L477-L516), [Classes](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/database/src/main/java/pl/umk/mat/zawodyweb/database/pojo/Classes.java), [ZIP manifests](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/www/src/main/java/pl/umk/mat/zawodyweb/www/zip/ZipFile.java), [series import/export](https://github.com/faramir/ZawodyWeb/blob/0d1e68e4cda77b539774f95ce87808bf0966cb17/www/src/main/java/pl/umk/mat/zawodyweb/www/zip/ZipSerie.java).

## Plan assessment

The user's subsequent decision supersedes the original engine recommendations: **implement only an interface and a no-op judge; design the actual judge with the professor.** See [JUDGE.md](JUDGE.md).

1. Keep Go/chi/HTMX/GORM and improve existing foundations incrementally.
2. Build authoring and source persistence/history independently from evaluation.
3. Version published content so later integration can identify what was submitted.
4. Display saved code as unjudged; keep verdicts/scores absent and result actions unavailable.
5. Defer worker/sandbox/queue/result/checker/scoring design and implementation to the professor-led phase J.
6. Capture legacy import/metadata requirements early, without running bundled code.
7. Retain full feature parity as a future gate requiring both the web work and actual judge integration.
8. Use concrete acceptance checks instead of speculative calendar dates.

## Verification and limits

- Existing tests passed with an uncached `go test -race -count=1 ./...`; `go vet ./...` and `go build -o /private/tmp/cotests-review-build .` also completed successfully.
- The targeted file-backed connection-replacement probe reproduced the foreign-key defect.
- Follow-up minor fixes also passed uncached race tests, vet and build (output: /private/tmp/cotests-refine-plan-fixes). The permanent reconnection regression and updated HTTP/Unicode checks pass. The judge package contains only the interface/Noop and no engine integration.
- Inspected local application, templates, styles, tests, documentation and CI; compared relevant original Java models, checkers, main judge and ZIP code.
- No production data was modified. The original was cloned into a temporary directory for source inspection.
- No PostgreSQL server, deployed browser workflow, legacy Java runtime, or real judge sandbox was exercised. This is a source/design review with local Go verification, not a complete penetration test or a legacy compatibility certification.
