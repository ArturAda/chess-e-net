package ws

import (
	"chess-monolith/internal/users"
	"log"
	"net/http"

	"chess-monolith/pkg/jwtutil" // Замените yourname!

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // TODO : В продакшене нужно проверять Origin для безопасности
	},
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
		if err == nil && user != nil {
			rating = user.Rating
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
			Rating:       rating,
			QueueManager: qm,
		}

		client.Hub.Register <- client

		// ReadPump блокирует текущую горутину, пока сокет не закроется
		go client.WritePump()
		client.ReadPump()
	}
}
