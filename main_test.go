package main

import (
	"bytes"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"cotests/internal/db"
)

func TestEmbeddedTemplatesParse(t *testing.T) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		t.Fatalf("sub filesystem: %v", err)
	}
	if _, err := template.ParseFS(sub, "templates/*.html"); err != nil {
		t.Fatalf("parse templates: %v", err)
	}
}

func TestRunAdminCreate(t *testing.T) {
	database, closeDatabase, err := openDatabase(filepath.Join(t.TempDir(), "cotests.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(closeDatabase)

	passwords := []string{"password1", "password1"}
	var output bytes.Buffer
	err = runAdminCreate([]string{"--email", " ADMIN@example.com ", "--name", "Admin"}, database, func(string) (string, error) {
		password := passwords[0]
		passwords = passwords[1:]
		return password, nil
	}, &output)
	if err != nil {
		t.Fatalf("run admin create: %v", err)
	}

	user, err := db.GetUserByEmail(database, "admin@example.com")
	if err != nil {
		t.Fatalf("load administrator: %v", err)
	}
	if user.Role != db.RoleAdmin || user.Name != "Admin" {
		t.Fatalf("administrator = %#v", user)
	}
	if !strings.Contains(output.String(), "Created administrator admin@example.com") {
		t.Fatalf("command output = %q", output.String())
	}
}

func TestRunAdminCreateRejectsDuplicateAndMismatchedPasswords(t *testing.T) {
	database, closeDatabase, err := openDatabase(filepath.Join(t.TempDir(), "cotests.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(closeDatabase)
	if _, err := db.CreateAdmin(database, "admin@example.com", "password1", "Admin"); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	for _, test := range []struct {
		name      string
		passwords []string
		want      string
	}{
		{"duplicate", []string{"password1", "password1"}, "already exists"},
		{"mismatch", []string{"password1", "password2"}, "do not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			passwords := append([]string(nil), test.passwords...)
			err := runAdminCreate([]string{"--email", "admin@example.com"}, database, func(string) (string, error) {
				password := passwords[0]
				passwords = passwords[1:]
				return password, nil
			}, io.Discard)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run admin create error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRunCommandRejectsUnknownCommand(t *testing.T) {
	err := runCommand([]string{"admin", "delete"}, "unused", nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("run command error = %v", err)
	}
}
