package ws

import (
	"chess-monolith/internal/users"
	"chess-monolith/pkg/jwtutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DummyUserRepository - простая заглушка репозитория для тестов транспортного уровня
type DummyUserRepository struct{}

func (d *DummyUserRepository) CreateUser(_ *users.User) error               { return nil }
func (d *DummyUserRepository) GetUserByEmail(_ string) (*users.User, error) { return nil, nil }
func (d *DummyUserRepository) GetUserByID(_ uuid.UUID) (*users.User, error) {
	return nil, nil
}

func TestServeWS_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	hub := NewHub()
	dummyRepo := &DummyUserRepository{}

	router.GET("/ws", ServeWS(hub, dummyRepo, "test_secret"))

	w := httptest.NewRecorder()
	// Делаем запрос без query параметра ?token=
	req, _ := http.NewRequest("GET", "/ws", nil)

	router.ServeHTTP(w, req)

	// Ожидаем 401 Unauthorized согласно логике хендлера
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Token is not provided")
}

func TestServeWS_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	hub := NewHub()
	dummyRepo := &DummyUserRepository{}

	router.GET("/ws", ServeWS(hub, dummyRepo, "test_secret"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token=fake_invalid_token", nil)

	router.ServeHTTP(w, req)

	// Ожидаем 401 Unauthorized, так как jwtutil.ParseToken вернет ошибку
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid token")
}

func TestServeWS_UpgradeError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	hub := NewHub()
	dummyRepo := &DummyUserRepository{}

	router.GET("/ws", ServeWS(hub, dummyRepo, "secret"))

	validToken, err := jwtutil.GenerateToken("test_user_id", "secret")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token="+validToken, nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
