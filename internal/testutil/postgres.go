package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"cotests/internal/db"
	"gorm.io/gorm"
)

// NewPostgresDatabase isolates each integration test in a fresh random schema.
func NewPostgresDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("COTESTS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set COTESTS_TEST_POSTGRES_DSN to exercise PostgreSQL")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Host == "" {
		t.Fatal("PostgreSQL test DSN must be a URL")
	}
	admin, err := db.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		t.Fatal(err)
	}
	schema := "cotests_test_" + hex.EncodeToString(raw)
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	database, err := db.Open(parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	connection, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	return database
}
