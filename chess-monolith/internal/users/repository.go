package users

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// Контракт работы с БД
type Repository interface {
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateUser(user *User) error {
	// gorm.DB.Create генерирует INSERT INTO users ...
	if err := r.db.Create(user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) ||
			strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			return ErrUserExists
		}
		return ErrDatabase
	}

	return nil
}

func (r *repository) GetUserByEmail(email string) (*User, error) {
	var user User
	// gorm.DB.First генерирует SELECT * FROM users WHERE users.email = email LIMIT 1
	err := r.db.Where("email = ?", email).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, ErrDatabase
	}
	return &user, nil
}
