package db_test

import (
	"errors"
	"testing"
	"time"

	"cotests/internal/db"
	"cotests/internal/testutil"

	"gorm.io/gorm"
)

func TestCreateSessionStoresOnlyTokenHash(t *testing.T) {
	database := testutil.NewDatabase(t)
	user, err := db.CreateUser(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	session, token, err := db.CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.TokenHash == token || session.TokenHash == "" {
		t.Fatal("session token was not stored as a hash")
	}

	loaded, err := db.GetSessionByToken(database, token)
	if err != nil || loaded.ID != session.ID {
		t.Fatalf("get session by token: %v", err)
	}
}

func TestCreateUserAssignsFirstAdmin(t *testing.T) {
	database := testutil.NewDatabase(t)
	first, err := db.CreateUser(database, "first@example.com", "password1", "First")
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}
	second, err := db.CreateUser(database, "second@example.com", "password1", "Second")
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	if first.Role != db.RoleAdmin {
		t.Fatalf("first role = %q, want %q", first.Role, db.RoleAdmin)
	}
	if second.Role != db.RoleUser {
		t.Fatalf("second role = %q, want %q", second.Role, db.RoleUser)
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	database := testutil.NewDatabase(t)
	user, err := db.CreateUser(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	session, _, err := db.CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := database.Model(&db.Session{}).Where("id = ?", session.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire session: %v", err)
	}
	if err := db.DeleteExpiredSessions(database); err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}
	var count int64
	database.Model(&db.Session{}).Count(&count)
	if count != 0 {
		t.Fatalf("session count = %d, want 0", count)
	}
}

func TestGenerateToken(t *testing.T) {
	first, err := db.GenerateToken()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, err := db.GenerateToken()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if len(first) != 64 || first == second {
		t.Fatalf("tokens = %q, %q; want distinct 64-character tokens", first, second)
	}
}

func TestGetSessionByTokenReturnsNotFound(t *testing.T) {
	_, err := db.GetSessionByToken(testutil.NewDatabase(t), "missing-token")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("get missing session: %v, want record not found", err)
	}
}

func TestDeleteSession(t *testing.T) {
	database := testutil.NewDatabase(t)
	user := testutil.CreateUser(t, database, "user@example.com")
	session, token, err := db.CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := db.DeleteSession(database, session.ID); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	if _, err := db.GetSessionByToken(database, token); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("deleted session lookup: %v, want record not found", err)
	}
}

func TestCreateSessionRemovesExpiredSessions(t *testing.T) {
	database := testutil.NewDatabase(t)
	user := testutil.CreateUser(t, database, "user@example.com")
	expired, _, err := db.CreateSession(database, user.ID)
	if err != nil {
		t.Fatalf("create expired session: %v", err)
	}
	if err := database.Model(&db.Session{}).Where("id = ?", expired.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire session: %v", err)
	}
	if _, _, err := db.CreateSession(database, user.ID); err != nil {
		t.Fatalf("create replacement session: %v", err)
	}
	var count int64
	if err := database.Model(&db.Session{}).Count(&count).Error; err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("session count = %d, want 1", count)
	}
}

func TestAutoMigrateRemovesLegacyRawTokens(t *testing.T) {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.AutoMigrate(&db.User{}, &legacySession{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := db.AutoMigrate(database); err != nil {
		t.Fatalf("migrate legacy schema: %v", err)
	}
	if database.Migrator().HasColumn(&db.Session{}, "token") {
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
