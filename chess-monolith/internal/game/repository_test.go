package game

import (
	"testing"
	"time"

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

func TestRepository_GetGame(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)
	game := &Game{
		WhiteID:    uuid.New(),
		BlackID:    uuid.New(),
		Mode:       "classic",
		Status:     "active",
		Turn:       "white",
		BoardState: `{"moves":[]}`,
	}
	require.NoError(t, repo.CreateGame(game))

	found, err := repo.GetGame(game.ID)
	require.NoError(t, err)
	assert.Equal(t, game.ID, found.ID)

	_, err = repo.GetGame(uuid.New())
	assert.ErrorIs(t, err, ErrGameNotFound)
}

func TestIsStaleActiveGame(t *testing.T) {
	now := time.Now()

	assert.False(t, isStaleActiveGame(Game{Status: "draw", CreatedAt: now.Add(-time.Hour)}, now))
	assert.False(t, isStaleActiveGame(Game{Status: "active"}, now))
	assert.False(t, isStaleActiveGame(Game{
		Status:      "active",
		TimeLimitMs: int64(time.Minute / time.Millisecond),
		CreatedAt:   now.Add(-time.Minute),
	}, now))
	assert.True(t, isStaleActiveGame(Game{
		Status:    "active",
		CreatedAt: now.Add(-(2*defaultGameTimeLimit + staleActiveGameGraceTime + time.Second)),
	}, now))
}

func TestRepository_ListGamesForUser(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)

	userID := uuid.New()
	opponentID := uuid.New()
	otherUserID := uuid.New()

	oldGame := &Game{
		WhiteID:    userID,
		BlackID:    opponentID,
		Mode:       "classic",
		IsRanked:   true,
		Status:     "white_won",
		Turn:       "black",
		BoardState: `{"moves":[]}`,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
	}
	newGame := &Game{
		WhiteID:    opponentID,
		BlackID:    userID,
		Mode:       "classic",
		Status:     "active",
		Turn:       "white",
		BoardState: `{"moves":[]}`,
		CreatedAt:  time.Now(),
	}
	otherGame := &Game{
		WhiteID:    otherUserID,
		BlackID:    uuid.New(),
		Mode:       "classic",
		Status:     "active",
		Turn:       "white",
		BoardState: `{"moves":[]}`,
		CreatedAt:  time.Now().Add(time.Hour),
	}

	require.NoError(t, repo.CreateGame(oldGame))
	require.NoError(t, repo.CreateGame(newGame))
	require.NoError(t, repo.CreateGame(otherGame))

	games, err := repo.ListGamesForUser(userID)
	require.NoError(t, err)
	require.Len(t, games, 2)
	assert.Equal(t, newGame.ID, games[0].ID)
	assert.Equal(t, oldGame.ID, games[1].ID)
	assert.True(t, games[1].IsRanked)
}

func TestRepository_ListGamesForUserExpiresStaleActiveGames(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)
	userID := uuid.New()
	opponentID := uuid.New()

	staleGame := &Game{
		WhiteID:     userID,
		BlackID:     opponentID,
		Mode:        "classic",
		TimeLimitMs: int64(time.Second / time.Millisecond),
		Status:      "active",
		Turn:        "white",
		BoardState:  `{"moves":[]}`,
		CreatedAt:   time.Now().Add(-10 * time.Minute),
	}
	freshGame := &Game{
		WhiteID:     opponentID,
		BlackID:     userID,
		Mode:        "classic",
		TimeLimitMs: int64(10 * time.Minute / time.Millisecond),
		Status:      "active",
		Turn:        "black",
		BoardState:  `{"moves":[]}`,
		CreatedAt:   time.Now(),
	}

	require.NoError(t, repo.CreateGame(staleGame))
	require.NoError(t, repo.CreateGame(freshGame))

	games, err := repo.ListGamesForUser(userID)
	require.NoError(t, err)
	require.Len(t, games, 2)

	updatedStale, err := repo.GetGame(staleGame.ID)
	require.NoError(t, err)
	assert.Equal(t, StaleActiveGameStatus, updatedStale.Status)

	updatedFresh, err := repo.GetGame(freshGame.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", updatedFresh.Status)
}

func TestRepository_GetGameForUser(t *testing.T) {
	db := setupTestDB()
	require.NotNil(t, db)

	repo := NewRepository(db)

	userID := uuid.New()
	gameForUser := &Game{
		WhiteID:    userID,
		BlackID:    uuid.New(),
		Mode:       "classic",
		Status:     "active",
		Turn:       "white",
		BoardState: `{"moves":[]}`,
	}
	require.NoError(t, repo.CreateGame(gameForUser))

	t.Run("Participant Can Read Game", func(t *testing.T) {
		found, err := repo.GetGameForUser(gameForUser.ID, userID)
		require.NoError(t, err)
		assert.Equal(t, gameForUser.ID, found.ID)
	})

	t.Run("Non Participant Cannot Read Game", func(t *testing.T) {
		_, err := repo.GetGameForUser(gameForUser.ID, uuid.New())
		assert.ErrorIs(t, err, ErrGameNotFound)
	})
}
