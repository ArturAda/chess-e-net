package ws

import (
	"chess-monolith/internal/users"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"chess-monolith/pkg/jwtutil" // Замените yourname!

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     websocketOriginAllowed,
}

func websocketOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	originURL, err := url.Parse(origin)
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return false
	}

	if strings.EqualFold(originURL.Host, r.Host) {
		return true
	}

	for _, allowed := range websocketAllowedOrigins() {
		if canonicalOrigin(allowed) == canonicalOrigin(origin) {
			return true
		}
	}

	return false
}

func websocketAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if raw == "" {
		return []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"http://localhost:8080",
			"http://127.0.0.1:8080",
		}
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	return result
}

func canonicalOrigin(origin string) string {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host)
}

// ServeWS обрабатывает GET /ws запросы от браузера
func ServeWS(hub *Hub, userRepo users.Repository, jwtSecret string, qm QueueManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.Query("token")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token is not provided"})
			return
		}

		userID, err := jwtutil.ParseToken(tokenString, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		parsedUUID, _ := uuid.Parse(userID)
		user, err := userRepo.GetUserByID(parsedUUID) // Предполагаем, что такой метод есть в репозитории
		rating := 1200                                // Дефолтное значение
		username := ""
		if err == nil && user != nil {
			rating = user.Rating
			username = user.Username
		}

		// Апгрейдим HTTP в WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}

		client := &Client{
			Hub:          hub,
			Conn:         conn,
			Send:         make(chan []byte, 256),
			UserID:       userID,
			Username:     username,
			Rating:       rating,
			QueueManager: qm,
		}

		client.Hub.Register <- client

		// ReadPump блокирует текущую горутину, пока сокет не закроется
		go client.WritePump()
		client.ReadPump()
	}
}
