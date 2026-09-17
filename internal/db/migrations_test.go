package db_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cotests/internal/db"
)

type migrationRecord struct {
	Version uint
}

func (migrationRecord) TableName() string { return "schema_migrations" }

func TestMigrateRecordsEachVersionAndIsIdempotent(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "contests.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	pool, err := database.DB()
	if err != nil {
		t.Fatalf("database pool: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	if err := db.Migrate(database); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("second migration: %v", err)
	}

	var records []migrationRecord
	if err := database.Order("version ASC").Find(&records).Error; err != nil {
		t.Fatalf("read migration records: %v", err)
	}
	if got, want := len(records), 4; got != want {
		t.Fatalf("migration records = %d, want %d", got, want)
	}
	for index, record := range records {
		if want := uint(index + 1); record.Version != want {
			t.Fatalf("record %d version = %d, want %d", index, record.Version, want)
		}
	}
	if !database.Migrator().HasConstraint(&db.Session{}, "User") {
		t.Fatal("sessions user foreign key is missing")
	}
	if !database.Migrator().HasIndex(&db.Session{}, "ExpiresAt") {
		t.Fatal("sessions expiry index is missing")
	}
	for _, column := range []string{"token_hash", "expires_at"} {
		var details struct {
			NotNull int
		}
		if err := database.Raw("SELECT \"notnull\" AS not_null FROM pragma_table_info('sessions') WHERE name = ?", column).Scan(&details).Error; err != nil {
			t.Fatalf("read %s nullability: %v", column, err)
		}
		if details.NotNull != 1 {
			t.Fatalf("%s is nullable", column)
		}
	}
}

func TestMigrateStopsWhenExistingSessionsAreOrphaned(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&db.User{}, &legacyHashedSession{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := database.Create(&legacyHashedSession{UserID: 999, TokenHash: "hash", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatalf("create orphaned session: %v", err)
	}

	err = db.Migrate(database)
	if err == nil || !strings.Contains(err.Error(), "orphaned sessions") {
		t.Fatalf("migration error = %v, want orphaned-session error", err)
	}

	var count int64
	if err := database.Table("schema_migrations").Count(&count).Error; err != nil {
		t.Fatalf("count migration records: %v", err)
	}
	if count != 0 {
		t.Fatalf("migration records = %d, want 0 after rollback", count)
	}
}

func TestMigrateStopsWhenExistingSessionsAreIncomplete(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&db.User{}, &legacyHashedSession{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	user := db.User{Email: "legacy@example.com", PasswordHash: "hash", Role: db.RoleUser}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create legacy user: %v", err)
	}
	if err := database.Create(&legacyHashedSession{UserID: user.ID, TokenHash: "", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatalf("create incomplete session: %v", err)
	}

	err = db.Migrate(database)
	if err == nil || !strings.Contains(err.Error(), "incomplete sessions") {
		t.Fatalf("migration error = %v, want incomplete-session error", err)
	}
}

type legacyHashedSession struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index;not null"`
	TokenHash string `gorm:"uniqueIndex"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (legacyHashedSession) TableName() string { return "sessions" }
