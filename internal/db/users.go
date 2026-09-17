package db

import (
	"cotests/internal/auth"
	"errors"

	"gorm.io/gorm"
)

func CreateUser(database *gorm.DB, email, password, name string) (*User, error) {
	return createUser(database, email, password, name, RoleUser)
}

// CreateAdmin creates an administrator through a local operator command.
// Public registration must use CreateUser.
func CreateAdmin(database *gorm.DB, email, password, name string) (*User, error) {
	return createUser(database, email, password, name, RoleAdmin)
}

func createUser(database *gorm.DB, email, password, name, role string) (*User, error) {
	password_hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &User{Email: email, PasswordHash: password_hash, Name: name, Role: role}
	if err := database.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

func IsDuplicateError(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

func GetUserByEmail(database *gorm.DB, email string) (*User, error) {
	var user User
	if err := database.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
