# Cotests roadmap

Updated: 2026-09-08. Cotests is a remake of ZawodyWeb with additional functionality.

**Current decision: the judge is a no-op placeholder with an interface. Its engine, protocol, checkers and scoring will be designed with the professor.** The web MVP must progress without implementing or selecting a judge.

- [MVP.md](MVP.md): web MVP scope and C01–C15 work items.
- [JUDGE.md](JUDGE.md): placeholder contract and deferred design decisions.
- [FEATURE_PARITY.md](FEATURE_PARITY.md): eventual ZawodyWeb baseline.
- [ARCHITECTURE.md](ARCHITECTURE.md): web application structure.
- [CODE_REVIEW.md](CODE_REVIEW.md): findings, fixed items and remaining work.
- [AUTH.md](AUTH.md): account and permission policies.

## Current phase

Original roadmap: Phase 1 recorded complete; Phase 2 partially implemented through contest/series management.

Revised roadmap: **M1, partially complete**. MR 1/B0–B4 supplies the content backend:
scoped memberships, schedules, revision/test authoring, assignments, participant
statement/sample/PDF access and ZIP preview/commit. Frontend flows remain partial.
There are still no submissions or judge routes. [HTTP_API.md](HTTP_API.md) records
the implemented contract; MR 2/B5–B8 is the next backend changeset.

- [x] Go/chi, embedded HTMX/templates/CSS, GORM and SQLite with a selectable PostgreSQL driver.
- [x] Registration/login/logout, bcrypt, hashed sessions, global admin/user guards and current POST CSRF checks.
- [x] Contest/series CRUD and published contest discovery, including upcoming and archived schedules.
- [x] SQLite foreign keys survive connection replacement; regression coverage includes existing DSN options.
- [x] HTMX contest creation replaces the list, removing its stale empty state.
- [x] Password form/error text follows the existing byte-length rule; associated labels and autocomplete added.
- [x] Public registration always creates a user; a local terminal command creates administrators after password confirmation.
- [x] Minimal judge interface and Noop implementation returning ErrUnavailable. No submission endpoint uses it yet.
- [x] Versioned migrations, explicit runtime storage configuration, request CSRF/origin checks and bounded in-process authentication attempts.
- [x] Draft/published problem revisions and manual private sample/hidden tests for administrators.
- [x] Backend scoped memberships, PDF/artifact delivery, assignments and ZIP preview/commit.
- [ ] Frontend completion, submission persistence/history and the rest of the web MVP.

## Release sequence

| Milestone | Outcome | Completion gate |
| --- | --- | --- |
| M0 | Foundation fixes | Current data survives upgrades; controlled bootstrap, request protection and supported DB checks pass. |
| M1 | Problem authoring | Publish statements/PDFs, samples and private tests with revisioned metadata; capture legacy import fixtures. |
| M2 | Submission alpha, judge disabled | Save source once, show own history/code, display “Saved — not judged”; no fake results or queued work. |
| M3 | **Web MVP** | Account/contest operations, permissions, clarifications, persistence, browser checks and restore rehearsal pass. |
| M4 | Remaining web/content parity | Full administration, cloning, imports, aliases and baseline views; engine-dependent actions remain visibly unavailable. |
| J | Professor-led judge design and later integration | Agree the contract, then separately implement and verify actual grading and dependent result workflows. |
| M5 | Additional functionality | Prioritized extensions with regression checks. |

**Full ZawodyWeb replacement requires both M4 and J.** The web MVP with a disabled judge is not a live graded-contest release. J has no scheduled implementation date and is not a prerequisite for M1–M3.

## M0 — Foundation

- [x] C01: fix SQLite connection integrity.
- [x] C02: introduce operator-controlled admin creation and preserve existing administrators.
- [ ] C03: versioned migrations and explicit data/public-URL configuration are implemented; run the opt-in PostgreSQL integration check against the intended deployment before marking complete.
- [x] C04 backend: signed session-bound CSRF/origin protection, authentication limits and scoped publication/access policies.
- [x] Fix the stale contest-list message and auth form byte-length mismatch.
- [x] Backend transactional series/task reordering, private template delivery, request contexts and DB-independent liveness.
- [ ] Release readiness/restore rehearsal (MR 2), responsive browser flows (frontend).

The bootstrap/auth/migration changes are separate implementation tasks. They are not included in this branch's minor fixes.

## M1 — Problem authoring

- [x] C05: provide only the provisional judge interface/Noop package.
- [x] C06: Problem, ProblemRevision, SeriesProblem, TestCase and language/checker metadata references.
- [x] C07 backend: private artifacts/PDF, samples, hidden tests and bounded ZIP preview/commit.
- [ ] C07 frontend: complete authoring/publication/participant UI and readable statement rendering.
- [x] Scoped organizer/participant backend access and validation of content metadata.
- [ ] C08: record original XML/checker/language identifiers and representative import fixtures without executing any checker.

Limits, language selections and checker names are metadata at this stage. Validate their structure; do not claim runtime support or enforce grading semantics through a new engine.

## M2 — Saved submissions with an empty judge

- [ ] C09: transactional source persistence, ownership, schedule validation and idempotency.
- [ ] C10: inject Noop at the submission service boundary; handle ErrUnavailable as unjudged.
- [ ] C11: participant status/history/source views; verdict, score, runtime and memory remain absent.
- [ ] C12: ranking and rejudge surfaces clearly state that judging is unavailable; no fabricated standings or result mutation.

**Exit:** a source submission survives restart, a retried request creates no duplicate, another participant cannot read it, and the UI clearly distinguishes saved source from evaluated code. No background jobs, worker process, compiler, checker or executor is introduced.

## M3 — Web MVP

- [ ] C13: profile/password/recovery/disable flows, scoped membership, series schedules, archives and clarifications.
- [ ] C14: administration of saved submissions, submission/storage quotas and authoring operations.
- [ ] C15: rehearse the [MVP release checklist](MVP.md#mvp-release-checklist), including a clean restore and disabled-judge behavior.
- [ ] Verify core mobile/keyboard and ordinary/HTMX form flows; document setup and remaining blockers.

**Exit:** the platform supports authoring, participation and submission collection without direct database editing. It does not grade or award scores.

## M4 — Remaining web/content parity

- [ ] Full user administration, aliases, necessary permissions and account/history inventory.
- [ ] Problem cloning and bounded native/legacy problem/series/contest/test imports with preview.
- [ ] Legacy test/configuration metadata, language/compiler/checker catalogues and quiz answer collection.
- [ ] Remaining ranking/filter/freeze and rejudge administration surfaces; keep execution and scored output unavailable until J.
- [ ] Optional series IP restrictions with explicit trusted-proxy policy.
- [ ] Historical data migration with provenance; imported historical results, if supported, must not be presented as new judge results.

Rows in FEATURE_PARITY.md that require actual grading remain incomplete until J. Building a view or importing a checker name does not complete those rows.

## J — Professor-led work, deferred

Use [JUDGE.md](JUDGE.md) as the discussion boundary. Agree execution isolation, language support, request/result delivery, retries, verdicts, metrics, checker compatibility, scoring and rejudging semantics together.

Only after that design is agreed, create a separate implementation backlog and verify required legacy behavior. Previous Linux worker, Isolate, job lease and C++/Python execution proposals are not selected requirements for this branch or web MVP.

## Backend delivery sequence

1. MR 1/B0–B4: content/access/file backend implemented; verify PostgreSQL CI gate.
2. MR 2/B5–B8: source persistence/history, account lifecycle, clarifications,
   administration and operational release checks (C09–C15 backend).
3. Frontend: complete authoring/participant forms and browser release checks.
4. Continue C08 representative institutional fixtures and M4 parity while the
   professor-led judge design proceeds separately when scheduled.

C01 and C05 are implemented at their deliberately limited scope. C01–C15 identifiers are retained, but C05 and C10–C12 have been revised from the earlier engine-first plan to match the user's decision. No calendar dates are promised.
