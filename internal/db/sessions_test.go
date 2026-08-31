package db

import (
	"testing"
	"time"

	"gorm.io/gorm"
)

func testDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := AutoMigrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func TestCreateSessionStoresOnlyTokenHash(t *testing.T) {
	database := testDatabase(t)
	user, err := CreateUser(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, token, err := CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.TokenHash == token || session.TokenHash == "" {
		t.Fatal("session token was not stored as a hash")
	}

	loaded, err := GetSessionByToken(database, token)
	if err != nil || loaded.ID != session.ID {
		t.Fatalf("get session by token: %v", err)
	}
}

func TestCreateUserAssignsFirstAdmin(t *testing.T) {
	database := testDatabase(t)
	first, err := CreateUser(database, "first@example.com", "password1", "First")
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}
	second, err := CreateUser(database, "second@example.com", "password1", "Second")
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	if first.Role != RoleAdmin {
		t.Fatalf("first role = %q, want %q", first.Role, RoleAdmin)
	}
	if second.Role != RoleUser {
		t.Fatalf("second role = %q, want %q", second.Role, RoleUser)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	database := testDatabase(t)
	user, err := CreateUser(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	session, _, err := CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := database.Model(&Session{}).Where("id = ?", session.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire session: %v", err)
	}
	if err := DeleteExpiredSessions(database); err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}
	var count int64
	database.Model(&Session{}).Count(&count)
	if count != 0 {
		t.Fatalf("session count = %d, want 0", count)
	}
}

func TestAutoMigrateRemovesLegacyRawTokens(t *testing.T) {
	database, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&User{}, &legacySession{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := AutoMigrate(database); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}
	if database.Migrator().HasColumn(&Session{}, "token") {
		t.Fatal("legacy raw token column still exists")
	}
}

type legacySession struct {
	ID        uint `gorm:"primaryKey"`
	UserID    uint
	Token     string `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (legacySession) TableName() string { return "sessions" }
