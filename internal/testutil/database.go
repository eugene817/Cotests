package testutil

import (
	"testing"

	"cotests/internal/db"

	"gorm.io/gorm"
)

func NewDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
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

	return database
}

func CreateUser(t *testing.T, database *gorm.DB, email string) *db.User {
	t.Helper()

	user, err := db.CreateUser(database, email, "password1", "Test User")
	if err != nil {
		t.Fatalf("create user: %v", err)
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
