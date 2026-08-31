package main

import (
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cotests/internal/db"
	"cotests/internal/server"
)

//go:embed static
var staticFS embed.FS

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "cotests.db"
	}
	database, err := db.Open(dsn)
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

	tpl, err := template.ParseFS(sub, "templates/*.html")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	r := server.NewRouter(database, http.FileServer(http.FS(sub)), tpl, server.Config{
		SecureCookies: os.Getenv("SECURE_COOKIES") == "true",
	})
	httpServer := &http.Server{Addr: ":3000", Handler: r, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	log.Println("listening on :3000")
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
