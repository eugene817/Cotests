package db

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

func CreateSession(database *gorm.DB, userID uint) (*Session, string, error) {
	if err := DeleteExpiredSessions(database); err != nil {
		return nil, "", fmt.Errorf("delete expired sessions: %w", err)
	}
	token, err := GenerateToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	session := &Session{
		UserID:    userID,
		TokenHash: hashToken(token),
		ExpiresAt: time.Now().Add(time.Hour * 24 * 7),
	}
	if err := database.Create(session).Error; err != nil {
		return nil, "", err
	}

	return session, token, nil
}

func GetSessionByToken(database *gorm.DB, token string) (*Session, error) {
	var session Session
	if err := database.Where("token_hash = ?", hashToken(token)).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash[:])
}

func DeleteSession(database *gorm.DB, sessionID uint) error {
	return database.Delete(&Session{}, sessionID).Error
}

func DeleteExpiredSessions(database *gorm.DB) error {
	return database.Where("expires_at < ?", time.Now()).Delete(&Session{}).Error
}
