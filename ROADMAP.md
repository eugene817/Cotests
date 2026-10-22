# Cotests roadmap

## Delivery status

**MR 1 / B0–B4 is implemented:** request protection, scoped memberships, series
schedules, immutable problem revisions/tests, assignments, protected PDF/sample
delivery and bounded ZIP preview/commit. SQLite and PostgreSQL 17 race tests pass,
including local PostgreSQL checks through OrbStack. Browser workflows remain
partial; see [FRONTEND_TODO.md](FRONTEND_TODO.md).

**MR 2 / B5–B8 is next.** Its goal is a web MVP for authoring and collecting
submissions with an explicitly unavailable judge. The implemented API is in
[HTTP_API.md](HTTP_API.md); setup and architecture are in [README.md](README.md).

## MR 2 — Remaining backend

### B5 — Saved submissions (C09–C12)

- [ ] Persist immutable source, author, assignment, revision, language and timestamp.
- [ ] Accept bounded source paste/upload after membership and schedule checks.
- [ ] Enforce quotas and transactional file/metadata persistence.
- [ ] Persist idempotency keys: identical retries return the same submission;
      conflicting reuse fails, including parallel requests.
- [ ] Inject Noop after saving; preserve source on `ErrUnavailable`.
- [ ] Provide paginated own history/source/download and authorized organizer inspection.
- [ ] Keep verdict, score, runtime and memory absent; expose unavailable ranking/rejudge states.

### B6 — Account lifecycle (C13)

- [ ] Profile editing with explicit field limits and authorized user lookup for membership UI.
- [ ] Password change requires current password, rotates the current session and revokes others.
- [ ] Own session listing/revocation and account disabling with immediate session revocation.
- [ ] Operator recovery using short-lived, hashed, single-use tokens and generic responses.
- [ ] Verify authentication abuse limits and disabled-account behavior.
- [ ] Require ownership verification before email changes if email recovery is introduced.

### B7 — Administration and questions (C13–C14)

- [ ] Private/public contest questions, optional problem reference and organizer replies.
- [ ] Explicit audience checks and deliberate publication of private questions.
- [ ] Administrative submission/content/membership operations and quotas.
- [ ] Audit actor/action/target for role changes, publication, imports and account operations.
- [ ] Archive referenced contests/content; prohibit deletion that destroys submission history.

### B8 — Release rehearsal (C15)

- [ ] Rehearse fresh installation and existing-data upgrades on supported databases.
- [ ] Restore a consistent DB snapshot and every referenced artifact onto a clean instance.
- [ ] Verify retained statements, tests, metadata and saved sources after restart/restore.
- [ ] Exercise parallel submissions, retries, bounded uploads, quota failures and interrupted saves.
- [ ] Verify readiness/configuration, deployment instructions and dependency review.
- [ ] Record bounded storage-workload latency and storage use; no judge throughput claims.

**Web MVP gate:** organizers prepare a contest with two series and at least three
tasks, manual and imported tests. Participants read released content, save source
once and retrieve only their own history. Cross-contest permissions, schedule
boundaries, question privacy and session lifecycle pass. Restart/restore preserves
content and source without triggering evaluation. Complete normal/HTMX, mobile,
keyboard, validation and expired-session browser checks before release.

## Later — Full ZawodyWeb replacement

These remain required baseline work, not completed by the web MVP:

- [ ] Full user administration, aliases, account/history inventory and collection statistics.
- [ ] Independent problem cloning with statement/PDF/tests/settings.
- [ ] Native and legacy problem/series/contest/test imports and exports with preview;
      native round trips preserve supported fields and invalid imports are atomic.
- [ ] Legacy configuration plus language/compiler/checker catalogues; “Classes”
      means executable plugin metadata, not educational cohorts.
- [ ] Quiz answer collection; quiz evaluation waits for real judging.
- [ ] Contest/series/global rankings, filters, inclusion flags, freeze/unfreeze,
      progress statistics and rejudge administration; actual results require the judge.
- [ ] Optional series IP restrictions with an explicit trusted-proxy policy.
- [ ] Historical user/content/submission migration with provenance, count/ownership
      checks and an explicit password conversion/reset policy; never reuse old sessions.
- [ ] Rehearse cutover, source-system freeze/export and rollback. Do not run two
      writable applications against a shared database.

## Legacy compatibility notes (C08)

Current ZIP support is only bounded flat `.in`/`.out` pairs. Legacy packages use
`contest.xml`, `serie.xml`, `problem.xml` and `tests.xml`, referencing statement
and test files. Capture ordering, codes, limits/units, points, timing, language and
checker identifiers. Obtain representative institutional exports before claiming
compatibility; unsupported required fields/plugins remain release blockers.

Preserve unknown identifiers as metadata without executing plugins. The original
checker contract can run the program itself, so execution/comparison boundaries
need deliberate mapping. ExactDiff compares Java characters; NormalDiff
normalizes internal whitespace; TrailingDiff trims both line ends and allows extra
trailing blank lines. QuizDiff uses numbered, case-normalized answers with rounded
partial points and ACC for any positive score. A generic byte/regex comparison is
not automatically equivalent. Inventory C/C++/Java/Python/Pascal and installed
custom profiles, scoring/ties, manual grading and external integrations.

Source review reference: [ZawodyWeb at 0d1e68e](https://github.com/faramir/ZawodyWeb/tree/0d1e68e4cda77b539774f95ce87808bf0966cb17).

## Judge — Deferred professor-led design

`internal/judge` contains only `Judge.RequestEvaluation(ctx, submissionID)` and a
side-effect-free Noop returning `ErrUnavailable`. Save source first and display
**Saved — not judged**. No queues, workers, polling, retries, callbacks, fake
results or automatic evaluation of old submissions are part of the placeholder.

Agree execution isolation, languages/toolchains, transport, result schema/units,
checker/plugin compatibility, scoring, quiz/manual/external workflows, ranking
visibility, cancellation, retries, rejudging and backfill policy with the professor.
No sandbox, worker technology or protocol has been selected. Implement and verify
the agreed engine separately against representative fixtures. Full replacement
requires both the remaining web/content baseline and real judging.
