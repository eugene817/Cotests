package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"cotests/internal/db"
	"gorm.io/gorm"
)

const csrfCookieName = "csrf_token"

type contextKey string

const userContextKey contextKey = "user"

type IdentityProvider interface {
	GetUser(*http.Request) *db.User
}

type CookieSessionProvider struct {
	DB *gorm.DB
}

func (p CookieSessionProvider) GetUser(r *http.Request) *db.User {
	cookie, err := r.Cookie("session_token")
	if err != nil || cookie.Value == "" {
		return nil
	}

	session, err := db.GetSessionByToken(p.DB, cookie.Value)
	if err != nil {
		return nil
	}
	if time.Now().After(session.ExpiresAt) {
		if err := db.DeleteSession(p.DB, session.ID); err != nil {
			return nil
		}
		return nil
	}

	var user db.User
	if err := p.DB.First(&user, session.UserID).Error; err != nil {
		return nil
	}
	return &user
}

func AuthMiddleware(provider IdentityProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := provider.GetUser(r)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
		})
	}
}

func UserFromContext(ctx context.Context) *db.User {
	user, _ := ctx.Value(userContextKey).(*db.User)
	return user
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			if isHTMX(r) {
				renderAccessError(w, http.StatusUnauthorized, "Please log in to continue.")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			for _, role := range roles {
				if user != nil && user.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			renderAccessError(w, http.StatusForbidden, "You do not have access to this page.")
		})
	}
}

func renderAccessError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<div class="card surface-container-highest" role="alert">%s</div>`, message)
}

func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
