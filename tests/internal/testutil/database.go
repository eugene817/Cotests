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

func NewDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	return openDatabase(t, ":memory:")
}

func openDatabase(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("get database connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return database
}

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
	return openDatabase(t, parsed.String())
}

func CreateUser(t *testing.T, database *gorm.DB, email string) *db.User {
	t.Helper()

	user, err := db.CreateUser(database, email, "password1", "Test User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func CreateAdmin(t *testing.T, database *gorm.DB, email string) *db.User {
	t.Helper()

	user, err := db.CreateAdmin(database, email, "password1", "Test Administrator")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	return user
}

func CreateSession(t *testing.T, database *gorm.DB, userID uint) string {
	t.Helper()

	_, token, err := db.CreateSession(database, userID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}
