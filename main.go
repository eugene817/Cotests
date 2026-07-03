package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cotests/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed static
var staticFS embed.FS

func main() {
	database, err := db.Open("cotests.db")
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("db underlying: %v", err)
	}
	defer sqlDB.Close()

	if err := db.AutoMigrate(database); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("migrations applied")

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("embed: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := sqlDB.Ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "db: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(indexHTML))
	})

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down...")
		sqlDB.Close()
		os.Exit(0)
	}()

	log.Println("listening on :3000")
	if err := http.ListenAndServe(":3000", r); err != nil {
		log.Fatalf("server: %v", err)
	}
}

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Cotests — The Digital Atelier</title>
  <link rel="preconnect" href="https://fonts.googleapis.com">
  <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
  <link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&family=Noto+Serif:ital,wght@0,400;0,500;0,600;1,400&display=swap" rel="stylesheet">
  <link rel="stylesheet" href="/static/css/style.css">
  <script src="/static/js/htmx.min.js"></script>
  <script src="/static/js/app.js"></script>
</head>
<body>
  <nav class="glass glass-nav">
    <span class="label" style="font-family:var(--font-display); font-size:1.25rem; color:var(--primary);">Cotests</span>
  </nav>

  <main style="max-width: 720px; margin: 0 auto; padding: 0 var(--spacing-8) var(--spacing-16);">
    <h1>The Digital&nbsp;Atelier</h1>
    <p class="label" style="margin-bottom:var(--spacing-8)">A premium editorial experience for modern testing</p>

    <div class="card" style="margin-bottom:var(--spacing-8)">
      <div class="card-header">
        <h3>Session 2: GORM</h3>
      </div>
      <div class="card-body">
        <p>SQLite via <code>github.com/glebarez/sqlite</code> — pure Go GORM driver. PostgreSQL support via <code>gorm.io/driver/postgres</code>. AutoMigrate from Go structs.</p>
      </div>
      <button class="btn btn-primary" hx-get="/health" hx-trigger="click" hx-target="#health" hx-swap="innerText">
        Health Check
      </button>
    </div>

    <div id="health" class="label-sm surface-container-low" style="padding:var(--spacing-4); border-radius:var(--radius-base); min-height:2rem; margin-bottom:var(--spacing-8);">
      Click the button to check DB and server health.
    </div>

    <div style="display:flex; gap:var(--spacing-4); flex-wrap:wrap;">
      <button class="btn btn-primary">Primary CTA</button>
      <button class="btn btn-secondary">Secondary</button>
      <button class="btn btn-tertiary">Tertiary</button>
    </div>
  </main>
</body>
</html>`
