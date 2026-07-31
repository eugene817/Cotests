package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cotests/internal/db"
	"cotests/internal/server"
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

	tpl, err := template.ParseFS(sub, "templates/*.html")
	if err != nil {
		log.Fatalf("templates: %v", err)
	}

	r := server.NewRouter(database, http.FileServer(http.FS(sub)), tpl)

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
