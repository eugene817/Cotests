package server

import (
	"crypto/rand"
	"net/http"
	"net/url"
	"strings"

	"cotests/internal/artifact"
	"cotests/internal/content"
	"cotests/internal/db"
	"cotests/internal/importer"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gorm.io/gorm"
)

type Config struct {
	SecureCookies    bool
	PublicURL        *url.URL
	ArtifactDir      string
	MaxArtifactBytes int64
	MaxStorageBytes  int64
	MaxImportBytes   int64
	MaxImportFiles   int
}

func NewRouter(database *gorm.DB, staticHandler http.Handler, tpl Template, config Config) chi.Router {
	if config.MaxArtifactBytes <= 0 {
		config.MaxArtifactBytes = 10 << 20
	}
	if config.MaxImportBytes <= 0 {
		config.MaxImportBytes = 50 << 20
	}
	if config.MaxImportFiles <= 0 {
		config.MaxImportFiles = 100
	}
	if config.MaxStorageBytes <= 0 {
		config.MaxStorageBytes = 1 << 30
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	service := &content.Service{DB: database, Store: artifact.Store{Root: config.ArtifactDir, MaxBytes: config.MaxArtifactBytes, MaxTotalBytes: config.MaxStorageBytes}, ImportLimits: importer.Limits{MaxBytes: config.MaxImportBytes, MaxFiles: config.MaxImportFiles}}
	h := &Handler{
		DB:               database,
		Tpl:              tpl,
		SecureCookies:    config.SecureCookies,
		ArtifactDir:      config.ArtifactDir,
		MaxArtifactBytes: config.MaxArtifactBytes,
		Content:          service,
		CSRFKey:          key,
		authWork:         make(chan struct{}, 4),
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(AuthMiddleware(CookieSessionProvider{DB: database}))
	r.Use(RequestProtection(config.PublicURL, h, config))

	r.Handle("/static/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !(strings.HasPrefix(r.URL.Path, "/static/css/") || strings.HasPrefix(r.URL.Path, "/static/js/")) || strings.Contains(r.URL.Path, "..") || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		http.StripPrefix("/static/", staticHandler).ServeHTTP(w, r)
	}))
	r.Get("/live", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	r.Route("/api/v1", (&apiHandler{service: service, pages: h}).routes)

	r.Get("/health", h.Health)
	r.Get("/", h.Home)
	r.Get("/contests", h.PublicContests)
	r.Get("/contests/{contestID}", h.PublicContest)
	r.Get("/login", h.LoginPage)
	r.Get("/register", h.RegisterPage)
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)
	r.Post("/logout", h.Logout)
	r.Group(func(r chi.Router) {
		r.Use(RequireAuth)
		r.Use(RequireRole(db.RoleAdmin))
		r.Get("/admin", h.AdminRedirect)
		r.Get("/admin/contests", h.AdminDashboard)
		r.Post("/admin/contests", h.CreateContest)
		r.Get("/admin/contests/{contestID}", h.AdminContest)
		r.Post("/admin/contests/{contestID}", h.UpdateContest)
		r.Post("/admin/contests/{contestID}/delete", h.DeleteContest)
		r.Post("/admin/contests/{contestID}/series", h.CreateSeries)
		r.Post("/admin/contests/{contestID}/series/{seriesID}", h.UpdateSeries)
		r.Post("/admin/contests/{contestID}/series/{seriesID}/delete", h.DeleteSeries)
		r.Get("/admin/problems", h.AdminProblems)
		r.Post("/admin/problems", h.CreateProblem)
		r.Get("/admin/problems/{problemID}", h.AdminProblem)
		r.Post("/admin/problems/{problemID}/revisions", h.CreateProblemRevision)
		r.Post("/admin/problems/{problemID}/revisions/{revisionID}", h.UpdateProblemRevision)
		r.Post("/admin/problems/{problemID}/revisions/{revisionID}/publish", h.PublishProblemRevision)
		r.Post("/admin/problems/{problemID}/revisions/{revisionID}/tests", h.CreateProblemTest)
	})

	return r
}
