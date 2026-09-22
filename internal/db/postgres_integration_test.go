package db_test

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"cotests/internal/db"
)

// This test is opt-in because it needs a PostgreSQL server and CREATE SCHEMA
// permission. It uses a generated schema and removes only that schema.
func TestPostgresMigrations(t *testing.T) {
	baseDSN := os.Getenv("COTESTS_TEST_POSTGRES_DSN")
	if baseDSN == "" {
		t.Skip("set COTESTS_TEST_POSTGRES_DSN to run PostgreSQL integration checks")
	}

	parsed, err := url.Parse(baseDSN)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		t.Fatal("COTESTS_TEST_POSTGRES_DSN must be a PostgreSQL URL")
	}
	schema := "cotests_test_" + randomHex(t, 8)

	admin, err := db.Open(baseDSN)
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	adminPool, err := admin.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL test pool: %v", err)
	}
	t.Cleanup(func() { _ = adminPool.Close() })
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated PostgreSQL schema: %v", err)
		}
	})

	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	database, err := db.Open(parsed.String())
	if err != nil {
		t.Fatalf("open isolated PostgreSQL schema: %v", err)
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatalf("get isolated PostgreSQL pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate PostgreSQL schema: %v", err)
	}
	user, err := db.CreateUser(database, "postgres@example.com", "password1", "PostgreSQL")
	if err != nil {
		t.Fatalf("create PostgreSQL user: %v", err)
	}
	if err := database.Create(&db.Session{UserID: user.ID, TokenHash: "postgres-token", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatalf("create PostgreSQL session: %v", err)
	}
	if err := database.Create(&db.Session{UserID: user.ID + 1, TokenHash: "orphan", ExpiresAt: time.Now().Add(time.Hour)}).Error; err == nil {
		t.Fatal("PostgreSQL accepted a session with an unknown user")
	}
}

func randomHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	return hex.EncodeToString(value)
}
