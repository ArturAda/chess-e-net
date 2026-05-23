package main

import (
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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

type DummyQueueManager struct{}

func (d *DummyQueueManager) AddPlayer(_ *ws.Client, _ string, _ bool, _ time.Duration) {
}
func (d *DummyQueueManager) RemovePlayer(_ *ws.Client) {}

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
