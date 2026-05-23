package users

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User - доменная модель пользователя
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"uniqueIndex;not null;size:50"`
	Email        string    `gorm:"uniqueIndex;not null;size:100"`
	PasswordHash string    `gorm:"not null"`
	Rating       int       `gorm:"default:1200"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// BeforeCreate хук для GORM, генерирует UUID перед записью в БД
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}
