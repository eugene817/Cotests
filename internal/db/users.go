package db

import (
	"cotests/internal/auth"
	"database/sql"
	"errors"

	"gorm.io/gorm"
)

func CreateUser(database *gorm.DB, email, password, name string) (*User, error) {
	password_hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &User{Email: email, PasswordHash: password_hash, Name: name, Role: RoleUser}
	if err := database.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&User{}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			user.Role = RoleAdmin
		}
		return tx.Create(user).Error
	}, &sql.TxOptions{Isolation: sql.LevelSerializable}); err != nil {
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
