# Cotests web MVP

Updated: 2026-09-08.

**Goal: organizers author contests/problems/tests, participants save submissions and review their own history, and the application explicitly shows that judging is unavailable.**

The judge is an interface plus a side-effect-free Noop. Its design belongs to the user and professor; real grading is outside this MVP. See [JUDGE.md](JUDGE.md).

## Current state

Accounts and contest/series CRUD exist. Minor SQLite, HTMX empty-state and password form fixes are implemented on refine-plan-fixes. The judge placeholder package exists, but there are no problem/test/submission models or routes yet.

We are in M0 of [ROADMAP.md](ROADMAP.md), corresponding to partial Phase 2 in the original roadmap. M2 will be a submission-collection alpha; M3 will be the web MVP. Full replacement also needs M4 and the deferred judge work.

## Included scope

| Area | Web MVP behavior |
| --- | --- |
| Account | Register/login/logout, display-name editing, password change, session revocation, operator recovery and disabling. |
| Permissions | Platform admin, scoped organizer, participant; enforce ownership on source and private artifacts. |
| Contest | Contest/series authoring, publication/archive, rules, membership, schedules and readable release status. |
| Problems | Title/code, statement/PDF, samples, limits, language/checker metadata, points metadata and revisioned publication. |
| Tests | Manual input/output and bounded test ZIP import, order/weights/time overrides; inputs/answers stay private. |
| Submission | Bounded source paste/upload, language metadata selection, transactional save and retry idempotency. |
| Judge | Noop returns ErrUnavailable. No compilation, evaluation, queue, retries, metrics, scores or verdicts. |
| Participant history | Saved time, problem, language, source and “Saved — not judged”. Own-code download; no terminal-result polling. |
| Rankings/rejudge | Clear unavailable states. No fake scores, artificial ranks, fake queue or successful rejudge message. |
| Administration | Inspect authorized submissions, manage content/membership and quotas, answer private/public clarifications. |
| Operations | Foundation protections, schema upgrades, backup/restore and mobile/keyboard-usable normal/HTMX flows. |

Language and checker metadata do not promise executable support. Unset verdict/score/time/memory must remain absent, not zero-valued “results”. Do not add manual grading or a mock grader to bypass the deferred judge decision.

Quiz collection, full legacy imports/cloning/aliases and other remaining web features are M4. Actual quiz/code evaluation, scoring, checker execution, ranked results, rejudging and judge operations belong to J after design with the professor.

## Submission contract for later implementation

1. Authenticate and authorize against the contest/series schedule and problem release policy.
2. Validate request size and selected metadata. Record source, owner, problem revision, language and acceptance timestamp in one transaction.
3. Persist an idempotency key so network retries return the same saved submission.
4. The service may request evaluation through the injected judge interface. Noop returns ErrUnavailable immediately and does nothing.
5. Interpret that outcome as **saved, unjudged**. It does not undo a successful source save and does not create a queue job.
6. Show the saved source/history to its owner; keep result fields absent and result actions unavailable.
7. Restarting the application does not evaluate saved submissions. A future decision about backfilling them belongs to the professor-led integration.

The source-save acknowledgement is not a judging verdict. There are no automatic retries or background polling while Noop is installed.

## Concrete backlog

IDs C01–C15 remain stable references; engine-related scopes have been explicitly revised. “Done” covers only the described implementation.

| ID | Status | Milestone / work | Depends on | Acceptance evidence |
| --- | --- | --- | --- | --- |
| C01 | Done | M0: SQLite integrity | Existing DB | Replacement connections retain FK enforcement and caller DSN settings; cascades and invalid-parent checks pass. |
| C02 | Done | M0: operator bootstrap | Existing auth | Public registration cannot create an admin; the local password-confirmed command creates an administrator; existing roles survive upgrade. |
| C03 | In review | M0: migrations/configuration | C01 | Versioned upgrades and explicit data directory/public URL are implemented; run the supported PostgreSQL check against the intended deployment. |
| C04 | Pending | M0: request/access policies | C02, C03 | CSRF/origin/throttle checks and publication/schedule boundaries pass. |
| C05 | Done | M1: interface plus Noop only | None | Interface compiles; Noop only returns ErrUnavailable. It has no executor, dependencies or side effects. Not integrated into a submission flow yet. |
| C06 | Pending | M1: problem/revision/test model | C03, C04 | Editing a draft cannot mutate a published revision; grading configuration is stored as metadata. |
| C07 | Pending | M1: private assets and authoring UI | C06 | Publish statement/PDF/sample/test content; unauthorized access fails; bounded imports validate before commit. |
| C08 | Pending | M1: original-format inventory | Source review | Capture representative XML packages and required checker/language IDs without running them. |
| C09 | Pending | M2: save source | C06, C07 | Source persists once; retry, ownership and opening/closing checks pass. |
| C10 | Pending | M2: disabled-judge service integration | C05, C09 | ErrUnavailable keeps a saved submission unjudged; no queue or repeated evaluation requests are created. |
| C11 | Pending | M2: history/source/status UI | C09, C10 | Owner sees saved source and unavailable grading; no verdict/metrics/score appears. |
| C12 | Pending | M2: unavailable ranking/rejudge views | C11 | Result-dependent actions explain their unavailable state and cannot mutate results. |
| C13 | Pending | M3: account/contest operations | C04, C11 | Profile, password, recovery, disable, membership, schedule and clarification privacy work. |
| C14 | Pending | M3: submission administration | C09, C13 | Authorized inspection and quotas work; hidden tests and other participants' source remain protected. |
| C15 | Pending | M3: release rehearsal | C01–C14 | Complete the checklist below on the intended deployment and record evidence. |

## MVP release checklist

- [ ] Fresh installation and existing-data upgrade work; operator bootstrap is controlled.
- [ ] Organizer creates a contest, two series and at least three problems, using manual test entry and a bounded ZIP.
- [ ] Statement/PDF/samples render; hidden input/answer URLs reject participant access.
- [ ] Participants paste/upload source and select language metadata. Files are saved, never executed.
- [ ] Retrying a request creates one submission; restarting preserves source/history and does not trigger evaluation.
- [ ] Saved submissions explicitly show “Saved — not judged”; verdict, score, runtime and memory are absent.
- [ ] There is no worker/queue activity and no polling for results the placeholder cannot produce.
- [ ] Rankings and rejudge actions show unavailable states; none can manufacture a result.
- [ ] Ownership, membership, start/end boundaries and private/public clarifications pass.
- [ ] Account/password/recovery/disable/session flows work.
- [ ] Restore the database and all referenced artifacts onto a clean instance; inspect statements, source and metadata.
- [ ] Verify mobile/keyboard access, normal/HTMX validation, expired sessions and retained source after validation failure.
- [ ] Exercise a bounded upload/storage workload; record web latency and storage use. Judge throughput is not a metric for this release.
- [ ] Supported database checks and dependency review pass; operator instructions describe the empty judge.

## Professor-led follow-up

No sandbox prototype or worker infrastructure is required to finish this MVP. After joint design, schedule the implementation and integration tests for the actual judge and revisit result/scoring schemas accordingly.

A web-MVP release cannot be described as a completed online judge or full ZawodyWeb replacement. [FEATURE_PARITY.md](FEATURE_PARITY.md) tracks the remaining baseline, including every deferred evaluation capability.
