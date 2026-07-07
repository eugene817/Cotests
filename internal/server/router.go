package server

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB, sqlDB *sql.DB, staticHandler http.Handler, tpl Template) chi.Router {
	h := &Handler{
		DB:    db,
		SQLDB: sqlDB,
		Tpl:   tpl,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler))

	r.Get("/health", h.Health)
	r.Get("/", h.Home)
	r.Get("/login", h.LoginPage)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)

	return r
}
