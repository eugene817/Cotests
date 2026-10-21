package db_test

import (
	"testing"
	"time"

	"cotests/internal/db"
	"cotests/internal/testutil"
)

func TestPostgresMigrations(t *testing.T) {
	database := testutil.NewPostgresDatabase(t)
	user := testutil.CreateUser(t, database, "postgres@example.com")
	if err := database.Create(&db.Session{UserID: user.ID, TokenHash: "postgres-token", ExpiresAt: time.Now().Add(time.Hour)}).Error; err != nil {
		t.Fatalf("create PostgreSQL session: %v", err)
	}
	if err := database.Create(&db.Session{UserID: user.ID + 1, TokenHash: "orphan", ExpiresAt: time.Now().Add(time.Hour)}).Error; err == nil {
		t.Fatal("PostgreSQL accepted a session with an unknown user")
	}
}
