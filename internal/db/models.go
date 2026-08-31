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
	Role         string `gorm:"not null;default:user;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Session struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"index;not null"`
	TokenHash string `gorm:"uniqueIndex"`
	ExpiresAt time.Time
	CreatedAt time.Time
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(&User{}, &Session{}); err != nil {
		return err
	}
	if db.Migrator().HasColumn(&Session{}, "token") {
		// Existing tokens must not remain in the database after the hash-only migration.
		if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&Session{}).Error; err != nil {
			return err
		}
		return db.Migrator().DropColumn(&Session{}, "token")
	}
	return nil
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
