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
			Mode:       "classic",
			Turn:       "white",
			Status:     "active",
			BoardState: `{"Grid": {}}`,
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
			Mode:       "classic",
			Turn:       "white",
			Status:     "active",
			BoardState: `{"Grid": {}}`,
		}

		err := repo.CreateGame(testGame)
		require.NoError(t, err)

		newState := `{"Grid": {"0,0": {"Type": "rook"}}}`
		err = repo.UpdateGame(testGame.ID, newState, "checkmate", "black")
		require.NoError(t, err)

		updatedGame, _ := repo.GetGame(testGame.ID)
		assert.Equal(t, newState, updatedGame.BoardState)
		assert.Equal(t, "checkmate", updatedGame.Status)
		assert.Equal(t, "black", updatedGame.Turn)
	})

	t.Run("Update Non-existent Game", func(t *testing.T) {
		err := repo.UpdateGame(uuid.New(), `{"Grid": {}}`, "checkmate", "black")
		assert.ErrorIs(t, err, ErrGameNotFound)
	})
}
