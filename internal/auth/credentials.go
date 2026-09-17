package auth

import (
	"fmt"
	"net/mail"
	"strings"
)

const (
	MinPasswordBytes = 8
	MaxPasswordBytes = 72
)

func NormalizeEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", fmt.Errorf("enter a valid email address")
	}
	return email, nil
}

func ValidatePassword(password string) error {
	if len(password) < MinPasswordBytes || len(password) > MaxPasswordBytes {
		return fmt.Errorf("password must be between %d and %d UTF-8 bytes", MinPasswordBytes, MaxPasswordBytes)
	}
	return nil
}
