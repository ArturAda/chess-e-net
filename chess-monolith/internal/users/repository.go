package users

import (
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
	return r.db.Create(user).Error
}

func (r *repository) GetUserByEmail(email string) (*User, error) {
	var user User
	// gorm.DB.First генерирует SELECT * FROM users WHERE users.email = email LIMIT 1
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
