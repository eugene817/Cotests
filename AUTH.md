# Authentication and authorization

Reviewed 2026-09-08. This document distinguishes implemented behavior from the proposed release requirements. See [ROADMAP.md](ROADMAP.md) and [CODE_REVIEW.md](CODE_REVIEW.md). The current judge is only the empty interface implementation described in [JUDGE.md](JUDGE.md); result/worker permissions below are future scope.

## Implemented today

- User and Session models in internal/db; bcrypt password hashing in internal/auth.
- Registration, login and logout handlers in internal/server/handlers.go.
- CookieSessionProvider, identity context and auth/role middleware in internal/server/auth.go.
- Random 256-bit session identifiers; SHA-256 token hashes stored in the database.
- Seven-day sessions with HttpOnly, SameSite=Lax cookies and optional Secure via SECURE_COOKIES=true.
- Expired-session cleanup during session creation and when an expired session is presented.
- Public registration always creates a `user` account.
- A local `cotests admin create --email … [--name …]` command creates administrators after hidden password confirmation.
- Admin-only contest/series management, role-aware navigation, and series-parent checks.
- Current POST handlers validate the csrf_token cookie against a submitted form/header token.
- Ordinary unauthenticated protected-page requests redirect to login; HTMX requests receive a 401 fragment. Role denial returns 403.

MR 1 additionally implements scoped contest organizer/participant membership,
series release/submission-window policies and protected content/artifact access.
Browser mutations use centralized origin checks and HMAC-signed, session-bound
CSRF tokens. Authentication attempts are bounded per IP/account with a global
concurrency budget. See [HTTP_API.md](HTTP_API.md) and [BACKEND_MR1.md](BACKEND_MR1.md).

Profile editing, password change/recovery and account disabling are MR 2 work.

## M0 — Changes required before public deployment

**Bootstrap.** Implemented: `cotests admin create --email … [--name …]` opens and migrates the configured database, then requests and confirms a hidden terminal password before creating an administrator. Public registration always creates an ordinary account; existing administrators are preserved. The command rejects duplicate emails and non-interactive password input.

**Abuse protection.** Implemented: bounded per-account/per-IP attempt budgets,
expiring bounded buckets and a global cap on concurrent authentication work.
Forwarded addresses are not trusted. Account lifecycle and further abuse checks
remain MR 2 work.

**CSRF.** Implemented: centralized protection, bounded parsing and origin checks;
HMAC-signed tokens bind to the presented session cookie. Forms retain their token
field; JSON/multipart use `X-CSRF-Token`. Fetch a fresh token after authentication
changes or process restart. Anonymous login/registration are also protected.
Authentication for any future judge API remains part of the deferred design.

**Cookies and deployment.** Validate the configured public URL and Secure cookies in HTTPS deployments. Consider host-prefixed cookies for production. Keep loopback development explicit. Cookies, source code, passwords and CSRF tokens must not appear in logs.

**Database integrity.** Session foreign keys, expiry indexing and null constraints
are versioned migrations. HTTP lookups propagate request contexts. Content API
errors distinguish validation/access/conflict from operational failures.

These changes follow the relevant [OWASP authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html) and [CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html) guidance.

## Contest authorization

Keep a simple platform role and add contest membership rather than accumulating unrelated strings in User.Role.

| Actor | Scope |
| --- | --- |
| Platform administrator | Bootstrap/disable users, configure trusted toolchains and workers, administer all contests. |
| Contest organizer | Manage assigned contests and authorized problems, inspect submissions, publish results and perform audited rejudges. |
| Participant | Read permitted content, submit while eligible, view own source/results, and access published rankings. |
| Guest | Read public schedules, rules and released content according to contest policy. |

Implement policy functions for managing contests, reading problems, submitting, reading source, and viewing results. Resource ownership and membership checks belong in every relevant service, not only navigation or route groups. A PDF or HTMX fragment has the same visibility rules as its corresponding page.

Registration policy can later be open or invitation-only per installation. Membership controls contest participation; registration alone need not grant entry to every contest. Per-contest aliases affect display, not account identity or authorization.

## Account lifecycle for the web MVP

- Password change requires the current password; revoke other sessions and rotate the current session.
- Recovery uses short-lived, hashed, one-use tokens and generic responses. SMTP is optional infrastructure; a controlled operator-issued recovery path can support a small installation.
- Account disabling revokes sessions and denies further authentication.
- Profile fields have explicit length limits; display aliases are distinct from login email.
- Email changes require ownership verification if email is used for recovery.
- Keep bcrypt compatibility. The form/error text now documents the existing 8–72 UTF-8 byte constraint; the conflicting browser character minimum is removed and multibyte boundaries are covered. A future hash/policy upgrade should be independently justified and preserve existing logins.

## Optional institutional login

Do not require Keycloak for the first release. If institutional SSO is needed, use OIDC authorization-code flow with PKCE, state and nonce validation, and map the verified issuer plus subject to a local user.

The web browser should continue using the application's opaque session after the callback. This is more than swapping CookieSessionProvider for a JWT parser: callback validation, linking, logout, recovery and role mapping need explicit design. IdentityProvider can remain a request identity boundary.

Institutional authentication does not automatically grant platform administrator privileges. Keep contest authorization local unless a narrowly defined, audited mapping is required. Do not silently link accounts by unverified email.

## Verification gates

- Public registration never bootstraps an administrator; the local command creates one after password confirmation.
- Guests, participants, organizers of another contest and disabled accounts are denied forbidden pages, fragments, source and artifacts.
- Missing/invalid CSRF and cross-origin browser mutations fail; valid normal/HTMX forms work.
- Login/registration throttles bound work without permanently locking out a victim.
- Logout, password changes, recovery and disabling revoke the intended sessions.
- Database upgrades preserve valid users and remove legacy raw session tokens.
- SQLite and PostgreSQL integration tests cover the supported lifecycle.
