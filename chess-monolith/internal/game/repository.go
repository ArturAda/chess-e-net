package game

import (
	"log"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	CreateGame(game *Game) error
	GetGame(id uuid.UUID) (*Game, error)
	UpdateGame(game *Game) error
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
	var game Game
	err := r.db.First(&game, "id = ?", id).Error
	return &game, err
}

func (r *repository) UpdateGame(game *Game) error {
	result := r.db.Model(&Game{}).Where("id = ?", game.ID).Updates(game)

	if result.Error != nil {
		return ErrDatabase
	}

	if result.RowsAffected == 0 {
		return ErrGameNotFound
	}

	return nil
}
