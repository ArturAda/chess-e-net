package main

import (
	"chess-monolith/internal/users"
	"chess-monolith/internal/ws"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestPingRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hub := ws.NewHub()
	dummyRepo := &DummyUserRepository{}
	var userHandler *users.Handler = nil
	router := SetupRouter(userHandler, hub, dummyRepo, "test-secret")

	w := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/api/ping", nil)
	assert.NoError(t, err)

	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), "ok")
	assert.Contains(t, w.Body.String(), "pong")
}
