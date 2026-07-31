# Authentication & Authorization Plan

This document describes the current authentication/authorization architecture and the step-by-step plan to implement role-based access control (RBAC) for Cotests.

For the project goals and constraints, see [ROADMAP.md](./ROADMAP.md).

---

## Current State (as of Session 4)

Cotests already has a working local authentication stack:

- `User` and `Session` GORM models.
- Password hashing with `bcrypt`.
- Session tokens stored in HTTP-only cookies.
- Registration, login, and logout handlers.
- Graceful shutdown closes the database connection pool.

The next milestone is to add **roles** and **authorization middleware** so that admin-only areas (e.g., `/admin/*`) can be protected.

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
- Expired sessions are cleaned up opportunistically on login/logout and can later be moved to a background goroutine if needed.
- Session cookies are HTTP-only, `Path: "/"`, and have a fixed `MaxAge` matching the DB expiry.

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

- [ ] Add `Role string` field to `internal/db/models.go`.
- [ ] Run `go build` to verify GORM `AutoMigrate` picks up the change.
- [ ] Update `CreateUser` to promote the first registered user to `admin`.
- [ ] Add `internal/db/users.go` helper: `IsFirstUser(database) bool` or inline the count check.

### Session 6 — Authorization Middleware

- [ ] Define `IdentityProvider` interface.
- [ ] Move session-to-user resolution into `CookieSessionProvider`.
- [ ] Implement `AuthMiddleware`, `RequireAuth`, and `RequireRole`.
- [ ] Wire middleware into `NewRouter`.
- [ ] Protect `/admin` with `RequireAuth` + `RequireRole("admin")`.
- [ ] Add `AdminDashboard` handler placeholder.

### Session 7 — Role-Aware UI

- [ ] Update `PageData` if needed (no new fields required if `User` is already present).
- [ ] Update `layout.html` to show admin link only for `admin` users.
- [ ] Add `/admin` page template stub.
- [ ] Test flows:
  - [ ] First registration → user becomes admin → `/admin` accessible.
  - [ ] Second registration → user role → `/admin` returns 403.
  - [ ] Logout → `/admin` redirects to `/login`.

---

## Open Decisions

These should be resolved before coding starts:

1. **403 behavior for HTMX:** return a small error fragment, redirect to `/login`, or render a full "Access Denied" page?
2. **Role sync with Keycloak:** when Keycloak arrives, will `User.Role` remain the source of truth, or will roles be read from the OIDC token on every request?
3. **Session cleanup frequency:** opportunistic cleanup only, or a background goroutine every N minutes?
