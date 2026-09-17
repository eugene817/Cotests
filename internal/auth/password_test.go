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

func TestNormalizeEmail(t *testing.T) {
	email, err := NormalizeEmail("  USER@example.com ")
	if err != nil || email != "user@example.com" {
		t.Fatalf("NormalizeEmail() = %q, %v", email, err)
	}
	if _, err := NormalizeEmail("not-an-email"); err == nil {
		t.Fatal("invalid email was accepted")
	}
}

func TestValidatePassword(t *testing.T) {
	for _, password := range []string{"short", string(make([]byte, MaxPasswordBytes+1))} {
		if err := ValidatePassword(password); err == nil {
			t.Fatalf("ValidatePassword(%q) accepted an invalid length", password)
		}
	}
	if err := ValidatePassword("password1"); err != nil {
		t.Fatalf("ValidatePassword(valid): %v", err)
	}
}
