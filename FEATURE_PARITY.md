# ZawodyWeb functionality and Cotests parity

Updated: 2026-09-08. Cotests is a ZawodyWeb remake with additional functionality.

This matrix defines the baseline from the user's feature inventory and the source review in [CODE_REVIEW.md](CODE_REVIEW.md). It is a release checklist, not a claim that every legacy branch or institution-specific plugin has already been inventoried. Newly discovered baseline behavior must be added and scheduled before declaring full replacement.

Status meanings: **Implemented** exists in current code; **Partial** only a subset exists; **Missing** has no functional implementation; **Inventory** requires inspection of actual deployment/configuration. A planned design is not implementation.

M0–M3 deliver the [web MVP](MVP.md) with the judge disabled. M4 delivers remaining web/content work. **Full parity requires M4 plus J**, the professor-led design and later implementation of real judging. M5 adds extensions. The current [judge interface and Noop](JUDGE.md) do nothing and do not complete any grading capability.

## Participant workflows

| ID | Baseline capability | Current status | Target | Parity evidence |
| --- | --- | --- | --- | --- |
| U01 | Registration, login, logout | Partial: works; bootstrap/throttling fixes pending | M0 | Register/login/logout and attack-limit cases pass on supported DBs. |
| U02 | Profile, password change/recovery, sessions | Missing | M3 | Edit profile, change/recover password, revoke sessions without losing submission ownership. |
| U03 | Profile statistics and progress | Missing | M4 collection statistics; J judged progress | Totals agree with effective submissions; historical/imported results retain provenance. |
| U04 | Contest discovery, schedules, rules | Partial: active published contests only | M0/M3 | Upcoming/live/archive pages and release/schedule boundary fixtures agree with policy. |
| U05 | Problem statements, PDF, limits, languages, samples | Missing | M1 | Statement/attachment content and limits survive import and obey access rules. |
| U06 | Submit source, history, own source, verdict/time/memory | Missing | M2 saved source/history; J results | Known solution fixtures and ownership checks pass; jobs survive restarts. |
| U07 | Contest, series, global and filtered rankings | Missing | M2 unavailable views; M4 + J rankings | Imported fixture rankings match documented legacy scoring/filter rules. |
| U08 | Quiz answers and quiz results | Missing | M4 answer collection; J grading | Reference answer sets, rounding, normalization and partial points match required behavior. |
| U09 | Private/public clarification questions and replies | Missing | M3 | Public replies are visible to the correct audience; private replies remain author/organizer-only. |

A global ranking is baseline; a newly invented competitive rating formula is an extension. Inventory legacy aggregation/tie rules before claiming ranking parity.

## Organizer and administrator workflows

| ID | Baseline capability | Current status | Target | Parity evidence |
| --- | --- | --- | --- | --- |
| A01 | Contest/series create, edit, order, delete | Partial: basic CRUD exists | M0/M3 | Reorder without collisions; delete drafts safely; archive records with submissions. |
| A02 | User create, inspect, edit, disable | Missing: self-registration exists | M3 disable/recovery; M4 full administration | Admin lifecycle works; changes preserve result ownership and produce an audit record. |
| A03 | Roles, permissions, aliases | Partial: global admin/user guards | M3 scoped membership; M4 aliases/full baseline | Cross-contest access denied; alias changes affect display without merging identities. |
| A04 | Series timing and IP restrictions | Missing | M3 timing; M4 IP rules | Start/end and CIDR/trusted-proxy fixtures reproduce the configured restrictions. |
| A05 | Ranking inclusion, freeze and unfreeze | Missing | M1 metadata; M4 + J scored visibility | Rankings/details/exports follow one visibility policy and match expected frozen snapshots. |
| A06 | Problem metadata, code, limits, language selection | Missing | M1 metadata; J runtime enforcement | Validate metadata/limits; retain legacy display codes and explicit language mappings. |
| A07 | Tests: manual entry, ZIP, points, order, time override/config | Missing | M1 basic storage; M4 config; J evaluation | Compare imported test manifests, effective limits and reference scores. |
| A08 | Full problem clone including statement/PDF/tests/settings | Missing | M4 | Editing a clone cannot change its source; all references/settings are accounted for. |
| A09 | Problem/series/contest ZIP import and corresponding export | Missing | M1 fixture inventory; M4 import/export | Representative XML packages import; native round trips preserve supported content; invalid import is atomic. |
| A10 | Rejudge selected submissions and bulk scopes | Missing | M2 unavailable UI; J actual rejudge | New generations are auditable and publish safely; failures preserve prior official results. |
| A11 | Judge queue/worker monitoring and recovery | Missing | J | Queued/running/failed/retry states are explainable; stuck jobs recover or surface an actionable error. |
| A12 | Language/compiler administration | Missing | M1 metadata; J executable profiles | Administrators select/manage trusted versioned profiles; required legacy language fixtures execute safely. |
| A13 | Executable checker/plugin registry (“Classes”) | Missing | M4 metadata; J plugin behavior | Install/select a versioned trusted checker with equivalent functionality; no execution in the web process. |
| A14 | Manual grading | Missing | J | Reviewer, rubric/points, result publication and later corrections are recorded. |
| A15 | External judging integration | Missing; protocol inventory required | J | At least the required external integration completes authenticated, idempotent result delivery and failure recovery. |

“Classes” in the inspected original means executable class/plugin storage. Educational cohorts/tags are separate features; inventory any institution-specific categorization before assigning parity requirements.

## Checker and import compatibility

| ID | Compatibility contract | Current status | Target | Acceptance evidence |
| --- | --- | --- | --- | --- |
| K01 | ExactDiff behavior | Missing | J | Encoding/newline/empty-output fixtures expose and resolve differences between Java string and byte comparison. |
| K02 | NormalDiff behavior | Missing | J | Internal/leading/trailing/Unicode whitespace fixtures receive expected verdicts. |
| K03 | TrailingDiff behavior | Missing | J | Line trimming and trailing blank-line fixtures match required legacy behavior. |
| K04 | QuizDiff behavior | Missing | J | Numbered answers, case handling, missing answers, proportional points and rounding are verified. |
| K05 | Custom checker configuration, test context and partial points | Inventory | M1 inventory; J support | Every checker required by the baseline has a reviewed port or isolated adapter and representative fixtures. |
| K06 | ManualCheck and ExternalChecker workflows | Inventory | J | Workflow and failure cases match A14/A15, including scoring history. |
| K07 | Legacy XML ZIP formats and field mapping | Missing | M1 fixtures; M4 implementation | Test/problem/series/contest packages preserve statements, ordering, limits, points and configuration; unknown fields have an explicit resolution. |
| K08 | Existing users, aliases, content and submission history | Inventory | M4 | Migration rehearsal compares counts, ownership and historical scores; authentication uses a verified conversion/reset policy; sessions are not reused. |
| K09 | Legacy language inventory | Inventory | M1 inventory; J implementation | C, C++, Java, Python, Pascal and any other required installed profiles are inventoried; every in-scope profile passes compile/run/resource fixtures. |
| K10 | Baseline ranking algorithms and filters | Inventory | M1 capture; M4 + J implementation | Tie handling, score aggregation, exclusions and global/series filters are documented and reproduced on reference data. |

Parity means equivalent supported user workflows and defined scoring behavior, not identical Java internals or identical wall-clock measurements across different hardware. The handling of legacy Java plugins belongs to the professor-led design; required functionality must not be silently omitted.

Not every unknown third-party plugin can be promised compatible before inspection. **An unresolved required plugin blocks the full-parity release**; a warning that it is unsupported is not proof of completion. Optional/out-of-scope integrations require an explicit recorded scope decision.

## Continuous compatibility work

- M0/M1: inventory features and formats, obtain representative exports when available, record expected checker/verdict/score fixtures. Source-derived fixtures can begin immediately; institutional exports remain needed to validate that deployment.
- M2: verify saved source, ownership and unjudged status through Noop. Do not execute solutions or compare actual judge outcomes.
- M3: rehearse the web MVP's authoring, submission collection and unavailable-result states.
- M4: migrate representative contest content and user/history records with provenance; exercise remaining web administration. Keep grading-dependent rows incomplete.
- J: after design with the professor, verify real evaluation, scoring, ranking, rejudging and required checker/language/integration behavior. Only then run the full replacement rehearsal.
- Before cutover: rehearse backup, restore, rollback to the original installation and a submission freeze/final export procedure. Do not run two writable systems against a shared database.

## Full-functionality release gate

- [ ] Every baseline row has implementation and acceptance evidence; Inventory rows are resolved.
- [ ] Every required checker/language/integration has a verified mapping and test fixtures.
- [ ] Problem, series and contest imports reproduce the expected structure and scores.
- [ ] User/alias/history migration preserves ownership and provenance, or the chosen fresh-install scope is explicitly documented rather than described as migrated.
- [ ] Participant and administrator acceptance walkthroughs pass.
- [ ] Queue failure/retry, rejudge publication, ranking freeze, IP rules and recovery checks pass.
- [ ] Supported database upgrades and a full restore are demonstrated.
- [ ] No required legacy workflow remains on an “unsupported” list.
- [ ] Deployment/cutover instructions and compatibility differences are documented.

## Additional functionality after parity

M5 can add richer reusable-library browsing, separate practice mode, progress dashboards, cohort tools, new rating/scoring algorithms, new language integrations, institutional SSO and larger deployments. Content revisions and auditing can be introduced in the web application; job/result architecture waits for J.

New feature ideas must not remove or indefinitely defer baseline rows. If priorities change, update the matrix and release definition together.
