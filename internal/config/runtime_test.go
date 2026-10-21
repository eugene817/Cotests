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
	if config.DataDir != "." || config.ArtifactDir != "artifacts" || config.DatabaseDSN != "cotests.db" || config.ListenAddress != ":3000" || config.MaxArtifactBytes != 10<<20 || config.MaxImportFiles != 100 || config.MaxStorageBytes != 1<<30 {
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
	if config.ArtifactDir != filepath.Join("runtime-data", "artifacts") {
		t.Fatalf("artifact directory = %q", config.ArtifactDir)
	}
}

func TestLoadRejectsInvalidLimits(t *testing.T) {
	for _, key := range []string{"MAX_ARTIFACT_BYTES", "MAX_IMPORT_BYTES", "MAX_IMPORT_FILES", "MAX_STORAGE_BYTES"} {
		t.Run(key, func(t *testing.T) {
			_, err := Load(func(current string) string {
				if current == key {
					return "0"
				}
				return ""
			})
			if err == nil {
				t.Fatal("invalid limit was accepted")
			}
		})
	}
}

func TestLoadRejectsUnboundedUploadSettings(t *testing.T) {
	for key, value := range map[string]string{"MAX_ARTIFACT_BYTES": "9223372036854775807", "MAX_IMPORT_BYTES": "536870913", "MAX_IMPORT_FILES": "10001"} {
		_, err := Load(func(name string) string {
			if name == key {
				return value
			}
			return ""
		})
		if err == nil {
			t.Fatalf("%s accepted an unbounded setting", key)
		}
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
