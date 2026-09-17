package db_test

import (
	"errors"
	"testing"

	"cotests/internal/db"
	"cotests/internal/testutil"

	"gorm.io/gorm"
)

func TestGetUserByEmail(t *testing.T) {
	database := testutil.NewDatabase(t)
	created, err := db.CreateUser(database, "user@example.com", "password1", "User")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	loaded, err := db.GetUserByEmail(database, created.Email)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if loaded.ID != created.ID || loaded.Name != created.Name {
		t.Fatalf("loaded user = %#v, want %#v", loaded, created)
	}
}

func TestCreateUserRejectsDuplicateEmail(t *testing.T) {
	database := testutil.NewDatabase(t)
	if _, err := db.CreateUser(database, "user@example.com", "password1", "User"); err != nil {
		t.Fatalf("create first user: %v", err)
	}
	_, err := db.CreateUser(database, "user@example.com", "password2", "Other")
	if !db.IsDuplicateError(err) {
		t.Fatalf("duplicate error = %v, want duplicated-key error", err)
	}
}

func TestCreateUserAlwaysAssignsUserRole(t *testing.T) {
	database := testutil.NewDatabase(t)
	first, err := db.CreateUser(database, "first@example.com", "password1", "First")
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}
	if first.Role != db.RoleUser {
		t.Fatalf("first role = %q, want %q", first.Role, db.RoleUser)
	}
}

func TestCreateAdminAssignsAdminRole(t *testing.T) {
	database := testutil.NewDatabase(t)
	admin, err := db.CreateAdmin(database, "admin@example.com", "password1", "Admin")
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if admin.Role != db.RoleAdmin {
		t.Fatalf("admin role = %q, want %q", admin.Role, db.RoleAdmin)
	}
}

func TestGetUserByEmailReturnsNotFound(t *testing.T) {
	_, err := db.GetUserByEmail(testutil.NewDatabase(t), "missing@example.com")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("get missing user: %v, want record not found", err)
	}
}

func TestUserPasswordMethods(t *testing.T) {
	var user db.User
	if err := user.SetPassword("password1"); err != nil {
		t.Fatalf("set password: %v", err)
	}
	if !user.CheckPassword("password1") {
		t.Fatal("correct password was rejected")
	}
	if user.CheckPassword("wrongpass") {
		t.Fatal("incorrect password was accepted")
	}
}
