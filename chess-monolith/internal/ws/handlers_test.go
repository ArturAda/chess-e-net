package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestServeWS_NoToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.Default()

	hub := NewHub()
	router.GET("/ws", ServeWS(hub, "test_secret"))

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
	router.GET("/ws", ServeWS(hub, "test_secret"))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ws?token=fake_invalid_token", nil)

	router.ServeHTTP(w, req)

	// Ожидаем 401 Unauthorized, так как jwtutil.ParseToken вернет ошибку
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid token")
}
