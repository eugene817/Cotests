package db

import (
	"cotests/internal/auth"
	"time"
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
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
	User      User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Contest struct {
	ID          uint   `gorm:"primaryKey"`
	Title       string `gorm:"not null;check:title <> ''"`
	Description string
	Visibility  string     `gorm:"not null;default:draft;check:visibility IN ('draft','published');index"`
	StartAt     *time.Time `gorm:"check:contest_dates,end_at IS NULL OR start_at IS NULL OR end_at > start_at"`
	EndAt       *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Series      []Series `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type Series struct {
	ID        uint   `gorm:"primaryKey"`
	ContestID uint   `gorm:"not null;index;uniqueIndex:idx_series_contest_position,priority:1"`
	Title     string `gorm:"not null;check:title <> ''"`
	Position  int    `gorm:"not null;check:position >= 0;uniqueIndex:idx_series_contest_position,priority:2"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Contest   Contest
}

const (
	RoleAdmin        = "admin"
	RoleUser         = "user"
	ContestDraft     = "draft"
	ContestPublished = "published"
)

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
