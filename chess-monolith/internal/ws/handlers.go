// Файл: internal/ws/handlers.go
package ws

import (
	"log"
	"net/http"

	"chess-monolith/pkg/jwtutil" // Замените yourname!
	"github.com/gin-gonic/gin"
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
func ServeWS(hub *Hub, jwtSecret string) gin.HandlerFunc {
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

		// Апгрейдим HTTP в WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("Upgrade error:", err)
			return
		}

		client := &Client{
			Hub:    hub,
			Conn:   conn,
			Send:   make(chan []byte, 256),
			UserID: userID,
		}

		client.Hub.Register <- client

		// ReadPump блокирует текущую горутину, пока сокет не закроется
		go client.WritePump()
		client.ReadPump()
	}
}
