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

const DefaultRating = 1200

type RatingScope struct {
	Mode        string
	BoardSize   int
	TimeLimitMs int64
}

type RatingScopeDTO struct {
	Mode             string `json:"mode"`
	BoardSize        int    `json:"board_size"`
	TimeLimitMs      int64  `json:"time_limit_ms"`
	TimeLimitMinutes int    `json:"time_limit_minutes"`
}

type UserRatingDTO struct {
	RatingScopeDTO
	Rating      int `json:"rating"`
	GamesPlayed int `json:"games_played"`
}

type LeaderboardEntry struct {
	UserID      uuid.UUID
	Username    string
	Rating      int
	GamesPlayed int
}

type LeaderboardEntryDTO struct {
	Rank        int    `json:"rank"`
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Rating      int    `json:"rating"`
	GamesPlayed int    `json:"games_played"`
}

type LeaderboardDTO struct {
	Scope   RatingScopeDTO        `json:"scope"`
	Players []LeaderboardEntryDTO `json:"players"`
}

type UserRating struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_user_ratings_scope"`
	Mode        string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_user_ratings_scope"`
	BoardSize   int       `gorm:"not null;uniqueIndex:idx_user_ratings_scope"`
	TimeLimitMs int64     `gorm:"not null;uniqueIndex:idx_user_ratings_scope"`
	Rating      int       `gorm:"not null;default:1200"`
	GamesPlayed int       `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (r *UserRating) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return
}
