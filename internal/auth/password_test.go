package auth

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password was stored without hashing")
	}
	if !CheckPassword("correct horse battery staple", hash) {
		t.Fatal("correct password was rejected")
	}
	if CheckPassword("incorrect password", hash) {
		t.Fatal("incorrect password was accepted")
	}
}

func TestCheckPasswordRejectsInvalidHash(t *testing.T) {
	if CheckPassword("password", "not-a-bcrypt-hash") {
		t.Fatal("invalid hash was accepted")
	}
}
