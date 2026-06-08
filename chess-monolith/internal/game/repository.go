package game

import (
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StaleActiveGameStatus    = "abandoned"
	staleActiveGameGraceTime = 5 * time.Minute
	defaultGameTimeLimit     = 10 * time.Minute
)

type Repository interface {
	CreateGame(game *Game) error
	GetGame(id uuid.UUID) (*Game, error)
	GetGameForUser(id, userID uuid.UUID) (*Game, error)
	ListGamesForUser(userID uuid.UUID) ([]Game, error)
	UpdateGame(id uuid.UUID, boardStateJSON, status, turn string) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) CreateGame(game *Game) error {
	if err := r.db.Create(game).Error; err != nil {
		log.Printf("failed to create game: %v", err)
		return ErrDatabase
	}
	return nil
}

func (r *repository) GetGame(id uuid.UUID) (*Game, error) {
	if err := r.expireStaleActiveGames(time.Now()); err != nil {
		return nil, ErrDatabase
	}

	var game Game
	err := r.db.First(&game, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGameNotFound
		}
		return nil, ErrDatabase
	}
	return &game, nil
}

func (r *repository) GetGameForUser(id, userID uuid.UUID) (*Game, error) {
	if err := r.expireStaleActiveGames(time.Now()); err != nil {
		return nil, ErrDatabase
	}

	var game Game
	err := r.db.
		Where("id = ? AND (white_id = ? OR black_id = ?)", id, userID, userID).
		First(&game).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGameNotFound
		}
		return nil, ErrDatabase
	}
	return &game, nil
}

func (r *repository) ListGamesForUser(userID uuid.UUID) ([]Game, error) {
	if err := r.expireStaleActiveGames(time.Now()); err != nil {
		return nil, ErrDatabase
	}

	var games []Game
	if err := r.db.
		Where("white_id = ? OR black_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&games).Error; err != nil {
		return nil, ErrDatabase
	}
	return games, nil
}

func (r *repository) UpdateGame(id uuid.UUID, boardStateJSON, status, turn string) error {
	result := r.db.Model(&Game{}).Where("id = ?", id).Updates(map[string]interface{}{
		"board_state": boardStateJSON,
		"status":      status,
		"turn":        turn,
	})

	if result.Error != nil {
		return ErrDatabase
	}

	if result.RowsAffected == 0 {
		return ErrGameNotFound
	}

	return nil
}

func (r *repository) expireStaleActiveGames(now time.Time) error {
	var activeGames []Game
	if err := r.db.
		Where("status = ?", "active").
		Find(&activeGames).Error; err != nil {
		return err
	}

	for _, activeGame := range activeGames {
		if !isStaleActiveGame(activeGame, now) {
			continue
		}

		if err := r.db.Model(&Game{}).
			Where("id = ? AND status = ?", activeGame.ID, "active").
			Update("status", StaleActiveGameStatus).Error; err != nil {
			return err
		}
	}

	return nil
}

func isStaleActiveGame(item Game, now time.Time) bool {
	if item.Status != "active" || item.CreatedAt.IsZero() {
		return false
	}

	timeLimit := time.Duration(item.TimeLimitMs) * time.Millisecond
	if timeLimit <= 0 {
		timeLimit = defaultGameTimeLimit
	}

	return now.After(item.CreatedAt.Add(2*timeLimit + staleActiveGameGraceTime))
}
