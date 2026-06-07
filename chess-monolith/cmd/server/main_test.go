package main

import (
	"chess-monolith/internal/game/session"
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DummyUserRepository struct{}

func (d *DummyUserRepository) CreateUser(_ *users.User) error               { return nil }
func (d *DummyUserRepository) GetUserByEmail(_ string) (*users.User, error) { return nil, nil }
func (d *DummyUserRepository) GetUserByID(_ uuid.UUID) (*users.User, error) {
	return nil, nil
}
func (d *DummyUserRepository) UpdateRatings(_, _ uuid.UUID, _, _ int) error {
	return nil
}
func (d *DummyUserRepository) GetOrCreateRating(_ uuid.UUID, _ users.RatingScope) (*users.UserRating, error) {
	return &users.UserRating{Rating: users.DefaultRating}, nil
}
func (d *DummyUserRepository) ListRatingsForUser(_ uuid.UUID) ([]users.UserRating, error) {
	return nil, nil
}
func (d *DummyUserRepository) ListLeaderboard(_ users.RatingScope, _ int) ([]users.LeaderboardEntry, error) {
	return nil, nil
}
func (d *DummyUserRepository) ApplyRatingResult(_ uuid.UUID, _ uuid.UUID, _ users.RatingScope, _ float64) (int, int, error) {
	return 1216, 1184, nil
}

type DummyQueueManager struct{}

func (d *DummyQueueManager) AddPlayer(_ *ws.Client, _ string, _ int, _ bool, _ time.Duration) error {
	return nil
}
func (d *DummyQueueManager) RemovePlayer(_ *ws.Client) {}

func TestInitGameRegistryIncludesOnlineBoardSizes(t *testing.T) {
	registry := initGameRegistry()

	tests := []struct {
		modeName string
		size     int
	}{
		{modeName: "classic", size: 8},
		{modeName: "modern10", size: 10},
		{modeName: "modern12", size: 12},
	}

	for _, tt := range tests {
		t.Run(tt.modeName, func(t *testing.T) {
			s, err := session.NewSession(registry, tt.modeName, 10*time.Minute)

			require.NoError(t, err)
			assert.Equal(t, tt.size, s.Board.Width)
			assert.Equal(t, tt.size, s.Board.Height)
		})
	}
}

func TestPingRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var userHandler *users.Handler = nil
	hub := ws.NewHub()
	dummyRepo := &DummyUserRepository{}
	dummyQM := &DummyQueueManager{}

	router := SetupRouter(userHandler, hub, dummyRepo, "test-secret", dummyQM, nil)

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/ping", nil)
	assert.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
	assert.Contains(t, w.Body.String(), "pong")
}

func TestSetupRouter_FullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := ws.NewHub()
	dummyRepo := &DummyUserRepository{}
	dummyQM := &DummyQueueManager{}

	var userHandler *users.Handler = nil

	router := SetupRouter(userHandler, hub, dummyRepo, "test-secret", dummyQM, nil)

	// Проверяем, что роутер поднялся и эндпоинты зарегистрированы
	assert.NotNil(t, router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws", nil)
	router.ServeHTTP(w, req)

	// Ожидаем 401, так как нет токена (значит роут /ws работает и просит авторизацию)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
