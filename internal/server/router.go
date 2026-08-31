package server

import (
	"net/http"

	"cotests/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"
)

type Config struct {
	SecureCookies bool
}

func NewRouter(database *gorm.DB, staticHandler http.Handler, tpl Template, config Config) chi.Router {
	h := &Handler{
		DB:            database,
		Tpl:           tpl,
		SecureCookies: config.SecureCookies,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(AuthMiddleware(CookieSessionProvider{DB: database}))

	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler))

	r.Get("/health", h.Health)
	r.Get("/", h.Home)
	r.Get("/login", h.LoginPage)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth)
		r.Use(RequireRole(db.RoleAdmin))
		r.Get("/admin", h.AdminDashboard)
	})

	return r
}
