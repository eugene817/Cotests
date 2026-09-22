package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cotests/internal/auth"
	"cotests/internal/config"
	"cotests/internal/db"
	"cotests/internal/server"

	"golang.org/x/term"
	"gorm.io/gorm"
)

//go:embed static
var staticFS embed.FS

func main() {
	runtime, err := config.Load(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	if err := config.EnsureDataDir(runtime.DataDir); err != nil {
		log.Fatal(err)
	}
	dsn := runtime.DatabaseDSN
	if len(os.Args) > 1 {
		if err := runCommand(os.Args[1:], dsn, terminalPasswordPrompt(os.Stdin, os.Stdout), os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}

	database, closeDatabase, err := openDatabase(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer closeDatabase()
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
		SecureCookies: runtime.SecureCookies,
	})
	httpServer := &http.Server{Addr: runtime.ListenAddress, Handler: r, ReadHeaderTimeout: 5 * time.Second}

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

	if runtime.PublicURL != nil {
		log.Printf("listening on %s (public URL %s)", runtime.ListenAddress, runtime.PublicURL)
	} else {
		log.Printf("listening on %s", runtime.ListenAddress)
	}
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}

type passwordPrompt func(label string) (string, error)

func openDatabase(dsn string) (*gorm.DB, func(), error) {
	database, err := db.Open(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("db: %w", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		return nil, nil, fmt.Errorf("db underlying: %w", err)
	}
	closeDatabase := func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}
	if err := db.Migrate(database); err != nil {
		closeDatabase()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	return database, closeDatabase, nil
}

func runCommand(args []string, dsn string, prompt passwordPrompt, output io.Writer) error {
	if len(args) < 2 || args[0] != "admin" || args[1] != "create" {
		return errors.New("usage: cotests admin create --email admin@example.com [--name \"Administrator\"]")
	}
	database, closeDatabase, err := openDatabase(dsn)
	if err != nil {
		return err
	}
	defer closeDatabase()

	return runAdminCreate(args[2:], database, prompt, output)
}

func runAdminCreate(args []string, database *gorm.DB, prompt passwordPrompt, output io.Writer) error {
	flags := flag.NewFlagSet("admin create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	emailFlag := flags.String("email", "", "administrator email")
	nameFlag := flags.String("name", "", "administrator display name")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *emailFlag == "" {
		return errors.New("usage: cotests admin create --email admin@example.com [--name \"Administrator\"]")
	}

	email, err := auth.NormalizeEmail(*emailFlag)
	if err != nil {
		return err
	}
	password, err := prompt("Administrator password: ")
	if err != nil {
		return fmt.Errorf("read administrator password: %w", err)
	}
	confirmation, err := prompt("Confirm password: ")
	if err != nil {
		return fmt.Errorf("read administrator password confirmation: %w", err)
	}
	if password != confirmation {
		return errors.New("administrator passwords do not match")
	}
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}

	user, err := db.CreateAdmin(database, email, password, *nameFlag)
	if err != nil {
		if db.IsDuplicateError(err) {
			return fmt.Errorf("an account already exists for %s", email)
		}
		return fmt.Errorf("create administrator: %w", err)
	}
	fmt.Fprintf(output, "Created administrator %s (user ID %d).\n", user.Email, user.ID)
	return nil
}

func terminalPasswordPrompt(input *os.File, output io.Writer) passwordPrompt {
	return func(label string) (string, error) {
		if !term.IsTerminal(int(input.Fd())) {
			return "", errors.New("administrator password prompt requires an interactive terminal")
		}
		if _, err := fmt.Fprint(output, label); err != nil {
			return "", err
		}
		password, err := term.ReadPassword(int(input.Fd()))
		fmt.Fprintln(output)
		if err != nil {
			return "", err
		}
		return string(password), nil
	}
}
