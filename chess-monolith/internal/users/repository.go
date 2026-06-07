package users

import (
	"chess-monolith/pkg/elo"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository описывает контракт работы с БД
type Repository interface {
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id uuid.UUID) (*User, error)
	UpdateRatings(user1ID, user2ID uuid.UUID, newRating1, newRating2 int) error
	GetOrCreateRating(userID uuid.UUID, scope RatingScope) (*UserRating, error)
	ListRatingsForUser(userID uuid.UUID) ([]UserRating, error)
	ListLeaderboard(scope RatingScope, limit int) ([]LeaderboardEntry, error)
	ApplyRatingResult(user1ID, user2ID uuid.UUID, scope RatingScope, user1Score float64) (int, int, error)
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

func (r *repository) GetOrCreateRating(userID uuid.UUID, scope RatingScope) (*UserRating, error) {
	return r.getOrCreateRating(r.db, userID, normalizeRatingScope(scope))
}

func (r *repository) ListRatingsForUser(userID uuid.UUID) ([]UserRating, error) {
	var ratings []UserRating
	err := r.db.
		Where("user_id = ?", userID).
		Order("mode ASC, board_size ASC, time_limit_ms ASC").
		Find(&ratings).Error
	if err != nil {
		return nil, ErrDatabase
	}

	return ratings, nil
}

func (r *repository) ListLeaderboard(scope RatingScope, limit int) ([]LeaderboardEntry, error) {
	scope = normalizeRatingScope(scope)
	limit = normalizeLeaderboardLimit(limit)

	var rows []LeaderboardEntry
	err := r.db.Table("user_ratings").
		Select("user_ratings.user_id, users.username, user_ratings.rating, user_ratings.games_played").
		Joins("JOIN users ON users.id = user_ratings.user_id").
		Where("user_ratings.mode = ? AND user_ratings.board_size = ? AND user_ratings.time_limit_ms = ?",
			scope.Mode, scope.BoardSize, scope.TimeLimitMs).
		Order("user_ratings.rating DESC, user_ratings.games_played DESC, users.username ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, ErrDatabase
	}

	return rows, nil
}

func (r *repository) ApplyRatingResult(user1ID, user2ID uuid.UUID, scope RatingScope, user1Score float64) (int, int, error) {
	scope = normalizeRatingScope(scope)

	var newRating1 int
	var newRating2 int

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if _, err := r.getOrCreateRating(tx, user1ID, scope); err != nil {
			return err
		}
		if _, err := r.getOrCreateRating(tx, user2ID, scope); err != nil {
			return err
		}

		ratings, err := r.getRatingsForUpdate(tx, []uuid.UUID{user1ID, user2ID}, scope)
		if err != nil {
			return err
		}

		rating1, ok := ratings[user1ID]
		if !ok {
			return ErrDatabase
		}
		rating2, ok := ratings[user2ID]
		if !ok {
			return ErrDatabase
		}

		newRating1, newRating2 = elo.Calculate(rating1.Rating, rating2.Rating, user1Score)

		if err := tx.Model(&UserRating{}).
			Where("id = ?", rating1.ID).
			Updates(map[string]any{
				"rating":       newRating1,
				"games_played": gorm.Expr("games_played + ?", 1),
			}).Error; err != nil {
			return ErrDatabase
		}

		if err := tx.Model(&UserRating{}).
			Where("id = ?", rating2.ID).
			Updates(map[string]any{
				"rating":       newRating2,
				"games_played": gorm.Expr("games_played + ?", 1),
			}).Error; err != nil {
			return ErrDatabase
		}

		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	return newRating1, newRating2, nil
}

func (r *repository) getOrCreateRating(db *gorm.DB, userID uuid.UUID, scope RatingScope) (*UserRating, error) {
	rating := UserRating{
		UserID:      userID,
		Mode:        scope.Mode,
		BoardSize:   scope.BoardSize,
		TimeLimitMs: scope.TimeLimitMs,
		Rating:      DefaultRating,
	}

	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "mode"},
			{Name: "board_size"},
			{Name: "time_limit_ms"},
		},
		DoNothing: true,
	}).Create(&rating).Error
	if err != nil {
		return nil, ErrDatabase
	}

	var existing UserRating
	err = db.
		Where("user_id = ? AND mode = ? AND board_size = ? AND time_limit_ms = ?",
			userID, scope.Mode, scope.BoardSize, scope.TimeLimitMs).
		First(&existing).Error
	if err != nil {
		return nil, ErrDatabase
	}

	return &existing, nil
}

func (r *repository) getRatingsForUpdate(db *gorm.DB, userIDs []uuid.UUID, scope RatingScope) (map[uuid.UUID]UserRating, error) {
	query := db.
		Where("user_id IN ? AND mode = ? AND board_size = ? AND time_limit_ms = ?",
			userIDs, scope.Mode, scope.BoardSize, scope.TimeLimitMs).
		Order("user_id ASC")

	if db.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var rows []UserRating
	if err := query.Find(&rows).Error; err != nil {
		return nil, ErrDatabase
	}
	if len(rows) != len(userIDs) {
		return nil, ErrDatabase
	}

	result := make(map[uuid.UUID]UserRating, len(rows))
	for _, row := range rows {
		result[row.UserID] = row
	}

	return result, nil
}

func normalizeRatingScope(scope RatingScope) RatingScope {
	if scope.Mode == "" {
		scope.Mode = "classic"
	}
	if scope.BoardSize <= 0 {
		scope.BoardSize = 8
	}
	if scope.TimeLimitMs <= 0 {
		scope.TimeLimitMs = int64((10 * 60) * 1000)
	}
	return scope
}

func normalizeLeaderboardLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 100 {
		return 100
	}
	return limit
}
