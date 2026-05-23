package users

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository описывает контракт работы с БД
type Repository interface {
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id uuid.UUID) (*User, error)
	UpdateRatings(user1ID, user2ID uuid.UUID, newRating1, newRating2 int) error
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

func (r *repository) GetUserByID(id uuid.UUID) (*User, error) {
	var user User
	err := r.db.First(&user, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, ErrDatabase
	}

	return &user, nil
}

func (r *repository) UpdateRatings(user1ID, user2ID uuid.UUID, newRating1, newRating2 int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).Where("id = ?", user1ID).Update("rating", newRating1).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", user2ID).Update("rating", newRating2).Error; err != nil {
			return err
		}
		return nil
	})
}
