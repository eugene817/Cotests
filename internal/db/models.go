package db

import (
	"cotests/internal/auth"
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Email        string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	Name         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index;not null"`
	Token     string `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Session{})
}

func (u *User) SetPassword(p string) error {
	password_hash, err := auth.HashPassword(p)
	if err != nil {
		return err
	}
	u.PasswordHash = password_hash
	return nil
}

func (u *User) CheckPassword(p string) bool {
	return auth.CheckPassword(p, u.PasswordHash)
}
