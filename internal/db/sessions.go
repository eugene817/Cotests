package db

import (
	"crypto/rand"
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

func CreateSession(database *gorm.DB, userID uint) (*Session, error) {
	token, err := GenerateToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	session := &Session{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(time.Hour * 24 * 7),
	}
	if err := database.Create(session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

func GetSessionByToken(database *gorm.DB, token string) (*Session, error) {
	var session Session
	if err := database.Where("token = ?", token).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func DeleteSession(database *gorm.DB, sessionID uint) error {
	return database.Delete(&Session{}, sessionID).Error
}

func DeleteExpiredSessions(database *gorm.DB) error {
	return database.Where("expires_at < ?", time.Now()).Delete(&Session{}).Error
}
