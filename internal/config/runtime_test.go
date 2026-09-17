package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsAndDataDirectoryDatabase(t *testing.T) {
	config, err := Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	if config.DataDir != "." || config.DatabaseDSN != "cotests.db" || config.ListenAddress != ":3000" {
		t.Fatalf("default config = %#v", config)
	}

	config, err = Load(func(key string) string {
		if key == "DATA_DIR" {
			return "runtime-data"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.DatabaseDSN != filepath.Join("runtime-data", "cotests.db") {
		t.Fatalf("database DSN = %q", config.DatabaseDSN)
	}
}

func TestLoadKeepsExplicitDatabaseAndDerivesSecureCookies(t *testing.T) {
	config, err := Load(func(key string) string {
		switch key {
		case "DATABASE_URL":
			return "postgres://cotests:secret@db.example/cotests"
		case "PUBLIC_URL":
			return "https://cotests.example/"
		case "LISTEN_ADDR":
			return "127.0.0.1:8080"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.DatabaseDSN != "postgres://cotests:secret@db.example/cotests" || config.ListenAddress != "127.0.0.1:8080" {
		t.Fatalf("config = %#v", config)
	}
	if config.PublicURL == nil || config.PublicURL.String() != "https://cotests.example" || !config.SecureCookies {
		t.Fatalf("public configuration = %#v", config)
	}
}

func TestLoadRejectsInvalidPublicURL(t *testing.T) {
	for _, value := range []string{"/cotests", "ftp://cotests.example", "https://cotests.example/app", "https://user@cotests.example", "https://cotests.example/?x=1"} {
		t.Run(value, func(t *testing.T) {
			_, err := Load(func(key string) string {
				if key == "PUBLIC_URL" {
					return value
				}
				return ""
			})
			if err == nil || !strings.Contains(err.Error(), "PUBLIC_URL") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestEnsureDataDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "data")
	if err := EnsureDataDir(path); err != nil {
		t.Fatal(err)
	}
}
