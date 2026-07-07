package db

import (
	"cotests/internal/auth"

	"gorm.io/gorm"
)

func CreateUser(database *gorm.DB, email, password, name string) (*User, error) {
	password_hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &User{
		Email:        email,
		PasswordHash: password_hash,
		Name:         name,
	}
	if err := database.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func GetUserByEmail(database *gorm.DB, email string) (*User, error) {
	var user User
	if err := database.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
