package game

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	err := db.AutoMigrate(&Game{})
	if err != nil {
		return nil
	}
	return db
}

func TestRepository_CreateGame(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)

	t.Run("Create Game", func(t *testing.T) {
		testGame := &Game{
			WhiteID:    uuid.New(),
			BlackID:    uuid.New(),
			Status:     "active",
			BoardState: `{"pieces": [{"type": "king", "pos": "e1"}]}`,
			Config:     `{"size": 8}`,
		}

		err := repo.CreateGame(testGame)
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, testGame.ID)
	})
}

func TestRepository_UpdateGame(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)

	t.Run("Update Existing Game", func(t *testing.T) {
		testGame := &Game{
			WhiteID:    uuid.New(),
			BlackID:    uuid.New(),
			Status:     "active",
			BoardState: `{"pieces": [{"type": "king", "pos": "e1"}]}`,
			Config:     `{"size": 8}`,
		}

		err := repo.CreateGame(testGame)
		require.NoError(t, err)

		testGame.Status = "white_won"
		err = repo.UpdateGame(testGame)
		require.NoError(t, err)

		final, _ := repo.GetGame(testGame.ID)
		assert.Equal(t, "white_won", final.Status)
	})

	t.Run("Update Non-existent Game", func(t *testing.T) {
		fakeGame := &Game{
			ID:     uuid.New(),
			Status: "draw",
		}

		err := repo.UpdateGame(fakeGame)
		assert.ErrorIs(t, err, ErrGameNotFound)
	})
}
