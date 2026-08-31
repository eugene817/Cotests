# Authentication & Authorization Plan

This document describes the current authentication/authorization architecture and the step-by-step plan to implement role-based access control (RBAC) for Cotests.

For the project goals and constraints, see [ROADMAP.md](./ROADMAP.md).

---

## Current State (Phase 1 complete)

Cotests already has a working local authentication stack:

- `User` and `Session` GORM models.
- Password hashing with `bcrypt`.
- Session tokens stored in HTTP-only cookies.
- Registration, login, and logout handlers.
- `admin` and `user` roles, with the first registered user promoted to `admin`.
- Request-context identity middleware plus login and role guards.
- An admin-only `/admin` dashboard placeholder and role-aware navigation.
- CSRF validation for every POST endpoint.
- Hashed session tokens at rest; HTTP-only, SameSite cookies.
- Graceful shutdown closes the database connection pool.

Phase 2 can now add contest-management routes under `/admin/*` using the existing role middleware.

---

## Design Decisions

### 1. Do NOT integrate Keycloak now

Keycloak (or any OIDC/SAML provider) is intentionally deferred. Reasons:

- Cotests is designed to be a **single binary** with SQLite by default. Adding Keycloak introduces an external runtime dependency.
- The project is still in the foundation phase; OIDC complexity is premature.
- HTMX + OIDC flow requires extra handling (login redirects, token refresh, CSRF) that is not needed yet.
- Custom roles are sufficient for the next several roadmap phases.

Keycloak integration will be revisited later. The architecture described below is designed to make that migration straightforward.

### 2. Role storage

Roles are stored as a `string` column on the `User` table.

```go
type User struct {
    // ... existing fields ...
    Role string `gorm:"default:'user';index"`
}
```

Initial roles:

- `user` — default for all registered accounts.
- `admin` — full access to contest management and platform configuration.

Why `string` and not a boolean or enum:

- Simple to extend later (`moderator`, `organizer`, etc.).
- Easy to sync from an external identity provider (e.g., Keycloak realm roles) into the same column.
- GORM migrations stay trivial.

### 3. First registered user becomes admin

To avoid a manual "create admin" step in the default SQLite deployment, the very first `User` created is automatically promoted to `admin`.

```go
// Inside CreateUser or a dedicated service function:
var count int64
database.Model(&User{}).Count(&count)
if count == 1 {
    user.Role = "admin"
    database.Save(user)
}
```

### 4. Identity provider abstraction

A small interface isolates the HTTP layer from the concrete session mechanism. This makes future OIDC migration easier.

```go
type IdentityProvider interface {
    GetUser(r *http.Request) *db.User
}
```

Current implementation: `CookieSessionProvider` reads the `session_token` cookie, validates it against the `sessions` table, and returns the associated user.

Future implementation: `OIDCProvider` validates a JWT/access token, maps the external `sub` claim to a local `User`, and optionally syncs roles from token claims.

---

## Middleware

All middleware lives in `internal/server/middleware.go`.

### `AuthMiddleware`

Loads the user into `request.Context`. It never redirects. Anonymous requests are allowed through with `User == nil`.

```go
func AuthMiddleware(ip IdentityProvider) func(http.Handler) http.Handler
```

### `RequireAuth`

Requires a logged-in user. If `UserFromContext` returns `nil`, the request is redirected to `/login` (or returns an HTMX-compatible redirect/fragment based on `HX-Request`).

```go
func RequireAuth(next http.Handler) http.Handler
```

### `RequireRole(roles ...string)`

Requires an authenticated user whose role is one of the allowed values. Otherwise returns HTTP 403 or an error fragment.

```go
func RequireRole(roles ...string) func(http.Handler) http.Handler
```

### Usage example

```go
r.Group(func(r chi.Router) {
    r.Use(AuthMiddleware(sessionProvider))
    r.Use(RequireAuth)

    r.Get("/profile", h.Profile)
})

r.Group(func(r chi.Router) {
    r.Use(AuthMiddleware(sessionProvider))
    r.Use(RequireAuth)
    r.Use(RequireRole("admin"))

    r.Get("/admin", h.AdminDashboard)
})
```

---

## UI Behavior

- The shared `layout.html` shows navigation links based on `User` and `User.Role`.
- `admin` users see a link to `/admin`.
- Guests see `Login` / `Register` links.
- Authenticated users see `Logout` and `Profile` links.
- Forbidden/admin-only content is not rendered in the menu at all (defense in depth: middleware still protects the route).

---

## Session Hygiene

- `logout` already deletes the session record from the database.
- Expired sessions are cleaned up opportunistically when sessions are created and when an expired token is presented.
- Session cookies are HTTP-only, `SameSite=Lax`, `Path: "/"`, and have a fixed `MaxAge` matching the DB expiry. Set `SECURE_COOKIES=true` in HTTPS deployments.
- The random cookie token is SHA-256 hashed before persistence. Existing raw-token sessions are revoked by the migration.
- POST endpoints compare a form or `X-CSRF-Token` token against the `csrf_token` cookie using a constant-time comparison.

---

## Future Keycloak Migration Path

When Keycloak is introduced, the migration should be limited to:

1. Adding `ExternalID` and `Provider` columns to `User` (e.g., `sub` and `keycloak`).
2. Implementing `OIDCProvider`.
3. Replacing `CookieSessionProvider` with `OIDCProvider` in `main.go` (or supporting both behind a feature flag).
4. Syncing roles either:
   - from Keycloak claims into `User.Role`, or
   - by reading roles from the token at request time and keeping `User.Role` as a fallback/cache.

No route-level code should change because middleware depends only on `IdentityProvider` and `UserFromContext`.

---

## Implementation Checklist

### Session 5 — Roles and Admin Promotion

- [x] Add `Role string` field to `internal/db/models.go`.
- [x] Update `CreateUser` to promote the first registered user to `admin`.

### Session 6 — Authorization Middleware

- [x] Define `IdentityProvider` interface and `CookieSessionProvider`.
- [x] Implement and wire `AuthMiddleware`, `RequireAuth`, and `RequireRole`.
- [x] Protect `/admin` with `RequireAuth` + `RequireRole("admin")`.
- [x] Add `AdminDashboard` handler placeholder.

### Session 7 — Role-Aware UI

- [x] Update `layout.html` to show the admin link only for admins.
- [x] Add `/admin` page template stub.
- [x] Test first-user promotion, access for `admin`, denial for `user`, and login redirect for guests.

---

## Resolved Decisions

1. HTMX requests without access receive a small error fragment. Normal requests from guests redirect to `/login`; authenticated users without a required role receive 403.
2. Session cleanup is opportunistic. A background cleanup job can be introduced if the deployment requires it.
3. The future Keycloak role-sync policy remains open and should be decided when OIDC integration is scheduled.
