package game

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chess-monolith/internal/users"
	"chess-monolith/pkg/jwtutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testJWTSecret = "history-test-secret"

func setupGameTransportRouter(t *testing.T) (*gin.Engine, users.User, users.User, Game, string) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&users.User{}, &Game{}))

	currentUser := users.User{
		ID:           uuid.New(),
		Username:     "current",
		Email:        "current@test.local",
		PasswordHash: "hash",
		Rating:       1210,
	}
	opponent := users.User{
		ID:           uuid.New(),
		Username:     "opponent",
		Email:        "opponent@test.local",
		PasswordHash: "hash",
		Rating:       1190,
	}
	otherUser := users.User{
		ID:           uuid.New(),
		Username:     "other",
		Email:        "other@test.local",
		PasswordHash: "hash",
		Rating:       1200,
	}

	require.NoError(t, db.Create(&currentUser).Error)
	require.NoError(t, db.Create(&opponent).Error)
	require.NoError(t, db.Create(&otherUser).Error)

	boardState := `{"game_id":"game-1","board_size":8,"moves":[{"from":"e2","to":"e4","piece":{"type":"pawn","color":"white"}}]}`
	whiteVisualState := `{"light_square":{"id":"classic-green"},"pieces":{"white":"pixel"}}`
	blackVisualState := `{"light_square":{"id":"red"},"pieces":{"black":"neo"}}`
	gameForUser := Game{
		ID:               uuid.New(),
		WhiteID:          currentUser.ID,
		BlackID:          opponent.ID,
		Mode:             "classic",
		BoardSize:        8,
		TimeLimitMs:      600000,
		IsRanked:         true,
		Status:           "white_won",
		Turn:             "black",
		BoardState:       boardState,
		WhiteVisualState: whiteVisualState,
		BlackVisualState: blackVisualState,
		CreatedAt:        time.Now().Add(-time.Hour),
	}
	otherGame := Game{
		ID:          uuid.New(),
		WhiteID:     otherUser.ID,
		BlackID:     opponent.ID,
		Mode:        "classic",
		BoardSize:   8,
		TimeLimitMs: 300000,
		IsRanked:    false,
		Status:      "active",
		Turn:        "white",
		BoardState:  `{"moves":[]}`,
		CreatedAt:   time.Now(),
	}

	require.NoError(t, db.Create(&gameForUser).Error)
	require.NoError(t, db.Create(&otherGame).Error)

	token, err := jwtutil.GenerateToken(currentUser.ID.String(), testJWTSecret)
	require.NoError(t, err)

	router := gin.Default()
	NewHandler(NewRepository(db), users.NewRepository(db), testJWTSecret).SetupRoutes(router)

	return router, currentUser, opponent, gameForUser, token
}

func TestHandler_ListGames(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, currentUser, opponent, _, token := setupGameTransportRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/games", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Games []GameSummaryDTO `json:"games"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.Len(t, response.Games, 1)

	game := response.Games[0]
	assert.Equal(t, "classic", game.Mode)
	assert.Equal(t, 8, game.BoardSize)
	assert.Equal(t, int64(600000), game.TimeLimitMs)
	assert.True(t, game.IsRanked)
	assert.Equal(t, "white", game.PlayerColor)
	assert.Equal(t, "win", game.Result)
	assert.Equal(t, currentUser.ID.String(), game.White.ID)
	assert.Equal(t, opponent.ID.String(), game.Black.ID)
	assert.Equal(t, opponent.ID.String(), game.Opponent.ID)
	assert.Equal(t, "opponent", game.Opponent.Username)
	assert.Equal(t, 1190, game.Opponent.Rating)
	assert.True(t, json.Valid(game.BoardState))
	assert.True(t, json.Valid(game.VisualState))

	var boardState map[string]any
	require.NoError(t, json.Unmarshal(game.BoardState, &boardState))
	assert.Equal(t, float64(8), boardState["board_size"])

	var visualState map[string]any
	require.NoError(t, json.Unmarshal(game.VisualState, &visualState))
	assert.Contains(t, visualState, "light_square")
}

func TestHandler_GetGame(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _, _, gameForUser, token := setupGameTransportRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/games/"+gameForUser.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response GameDetailDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, gameForUser.ID.String(), response.ID)
	assert.Equal(t, "white_won", response.Status)
	assert.True(t, json.Valid(response.BoardState))
	assert.True(t, json.Valid(response.VisualState))

	var boardState map[string]any
	require.NoError(t, json.Unmarshal(response.BoardState, &boardState))
	assert.Equal(t, float64(8), boardState["board_size"])
	assert.Len(t, boardState["moves"], 1)

	var visualState map[string]any
	require.NoError(t, json.Unmarshal(response.VisualState, &visualState))
	assert.Contains(t, visualState, "pieces")
}

func TestHandler_GamesRequireAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _, _, _, _ := setupGameTransportRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/games", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandler_GetGame_NotFoundForNonParticipant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, currentUser, opponent, gameForUser, _ := setupGameTransportRouter(t)

	token, err := jwtutil.GenerateToken(opponent.ID.String(), testJWTSecret)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/games/"+gameForUser.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "opponent is a participant and should read the game")

	unknownUser := users.User{
		ID:           uuid.New(),
		Username:     "unknown",
		Email:        "unknown@test.local",
		PasswordHash: "hash",
	}
	// Create a new isolated router where the unknown user exists but is not a game participant.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&users.User{}, &Game{}))
	require.NoError(t, db.Create(&currentUser).Error)
	require.NoError(t, db.Create(&opponent).Error)
	require.NoError(t, db.Create(&unknownUser).Error)
	require.NoError(t, db.Create(&gameForUser).Error)

	unknownToken, err := jwtutil.GenerateToken(unknownUser.ID.String(), testJWTSecret)
	require.NoError(t, err)

	isolatedRouter := gin.Default()
	NewHandler(NewRepository(db), users.NewRepository(db), testJWTSecret).SetupRoutes(isolatedRouter)

	w = httptest.NewRecorder()
	req, _ = http.NewRequest(http.MethodGet, "/api/games/"+gameForUser.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+unknownToken)

	isolatedRouter.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_GetGame_ReturnsVisualStateForCurrentParticipant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _, opponent, gameForUser, _ := setupGameTransportRouter(t)

	opponentToken, err := jwtutil.GenerateToken(opponent.ID.String(), testJWTSecret)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/games/"+gameForUser.ID.String(), nil)
	req.Header.Set("Authorization", "Bearer "+opponentToken)

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response GameDetailDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	var visualState map[string]any
	require.NoError(t, json.Unmarshal(response.VisualState, &visualState))
	lightSquare, ok := visualState["light_square"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "red", lightSquare["id"])
}
