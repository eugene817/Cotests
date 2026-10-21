# Frontend TODO

## Purpose

Track frontend work remaining after backend MR 1 (B0–B4).

This is a working backlog, not a final visual specification. A full application
walkthrough and UX review are still pending. Refine this list after reviewing
the application in a browser.

Backend contracts: [HTTP_API.md](HTTP_API.md).
Backend scope: [BACKEND_MR1.md](BACKEND_MR1.md).

## Current State

- Existing pages cover authentication, contest/series administration,
  public contest browsing, and basic administrator-only problem authoring.
- Backend MR 1 adds scoped memberships, schedules, revisioned content,
  assignments, protected files, and ZIP preview/commit.
- Frontend coverage of these operations is incomplete.
- Submission collection and history require backend MR 2 (B5–B8).
- Judging is unavailable; language/checker identifiers are metadata only.

## Implementation Guidelines

- Keep Go templates, HTMX, and small JavaScript modules.
- Follow the existing Digital Atelier design tokens.
- Use shared backend authorization and validation rules.
- Treat statements as plain text until a safe Markdown rendering policy exists.
- Do not display fabricated scores, verdicts, rankings, or queue states.
- Complete useful end-to-end workflows before visual polish.

## P1 — Participant Content Browsing

- [ ] Complete navigation: contest → series → problem.
- [ ] Show series schedules and release states.
- [ ] List tasks available to the current participant.
- [ ] Display statement, language metadata, and optional limits.
- [ ] Display sample input/output with preserved whitespace and copy actions.
- [ ] Provide authorized PDF and sample downloads.
- [ ] Handle upcoming, ended, inaccessible, and missing content clearly.
- [ ] Explain membership requirements when access is denied.
- [ ] Keep released content readable after the contest ends.
- [ ] Add breadcrumbs and clear return navigation.

## P1 — Series Composition

- [ ] Add a task-management section to the series page.
- [ ] Select a task from the authorized problem library.
- [ ] Select a specific published revision.
- [ ] Set the assignment code and position.
- [ ] Reorder and remove task assignments.
- [ ] Display the assigned revision number.
- [ ] Make it clear that publishing a new revision does not automatically
      replace an existing assignment.
- [ ] Link to a participant-style statement preview.

## P1 — Problem Library and Authoring

- [ ] Make authoring navigation available to scoped organizers.
- [ ] List authorized problems with pagination.
- [ ] Create and edit problem titles and codes.
- [ ] Display revision history and distinguish drafts from publications.
- [ ] Edit draft statements and optional limit/language/checker metadata.
- [ ] Upload and replace draft PDFs.
- [ ] Preview statements and samples.
- [ ] Publish a revision with a clear explanation of immutability.
- [ ] Make published revisions read-only.
- [ ] Create a new draft from the latest revision.
- [ ] Preserve entered content when validation fails.

## P1 — Tests and ZIP Import

- [ ] List sample and hidden tests in an author-only section.
- [ ] Create, edit, and delete draft tests.
- [ ] Edit position, sample flag, points metadata, and time overrides.
- [ ] Provide authorized input/output downloads.
- [ ] Keep hidden tests out of participant views and browser payloads.
- [ ] Upload a ZIP and display preview results before confirmation.
- [ ] Show file names, sizes, validation errors, and preview expiry.
- [ ] Confirm import and refresh the test list.
- [ ] Handle expired previews and retryable failures.
- [ ] Explain the supported flat `.in`/`.out` format and upload limits.

## P2 — Contest Access and Scheduling

- [ ] Organize contest management into overview, series, and membership sections.
- [ ] Show publication status separately from opening/closing dates.
- [ ] Edit series start, end, and statement-release timestamps.
- [ ] Show the timezone explicitly.
- [ ] Reorder series without manually inventing intermediate positions.
- [ ] List and remove participants.
- [ ] Add existing users as participants.
- [ ] Allow platform administrators to manage organizer membership.
- [ ] Distinguish platform administration from scoped organizer permissions.

### Backend Dependency

Membership operations currently use user IDs. A friendly user picker requires
an authorized user lookup/search contract. Confirm this before implementing
membership forms; do not expose the full account directory to participants.

## P2 — Shared Request and UI States

- [ ] Integrate CSRF handling for JSON and multipart requests.
- [ ] Refresh CSRF state after authentication changes or server restart.
- [ ] Handle expired sessions without silently losing editor content.
- [ ] Display field validation, access denial, conflicts, and upload limits.
- [ ] Show upload/import progress or an appropriate busy state.
- [ ] Prevent accidental repeated actions while requests are running.
- [ ] Provide clear empty, success, and failure states.
- [ ] Keep normal form and HTMX behavior consistent where both are supported.
- [ ] Add pagination controls to backed lists.
- [ ] Ensure authorized downloads handle lost access gracefully.

## Later — Requires Backend MR 2

- [ ] Source paste/upload and language selection.
- [ ] Submission confirmation and own submission history.
- [ ] Source viewing/downloading and authorized organizer inspection.
- [ ] Explicit “Saved — not judged” status.
- [ ] Unavailable ranking and rejudge states.
- [ ] Profile, password, session, recovery, and account-disable flows.
- [ ] Private/public questions and organizer replies.
- [ ] Relevant quota, audit, and archive administration views.

## Application Walkthrough and UX Review

- [ ] Review the whole application as a guest, participant, organizer, and admin.
- [ ] Review navigation, terminology, and permission-related dead ends.
- [ ] Verify mobile layouts, long statements, and wide test data.
- [ ] Verify keyboard navigation, focus, labels, and error announcements.
- [ ] Replace fragile inline layout styles with reusable classes where needed.
- [ ] Check visual consistency against the existing design system.
- [ ] Record findings here and reprioritize after the walkthrough.

## First Frontend Completion Gate

An organizer can prepare and publish a contest entirely through the UI,
including tasks, tests, PDFs, assignments, schedules, and membership.

A participant can sign in, navigate to released tasks, read statements and
samples, and download permitted files. Hidden tests and unreleased content
remain inaccessible.

Both workflows handle validation failures and expired sessions clearly.
