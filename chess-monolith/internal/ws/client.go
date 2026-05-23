// Файл: internal/ws/client.go
package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024 // 1 KB для шахматных ходов более чем достаточно
)

// Client - это посредник между websocket-соединением и нашим Hub
type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan []byte
	UserID string
	Rating int
}

// WSMessage - универсальная структура для обмена JSON
type WSMessage struct {
	Type    string          `json:"type"`              // "MOVE", "JOIN_QUEUE", "RESIGN"
	Payload json.RawMessage `json:"payload,omitempty"` // Динамические данные
}

// ReadPump читает сообщения из сокета (от браузера к серверу)
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		err := c.Conn.Close()
		if err != nil {
			return
		}
	}()

	c.Conn.SetReadLimit(maxMessageSize)

	err := c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	if err != nil {
		return
	}

	c.Conn.SetPongHandler(func(string) error {
		err := c.Conn.SetReadDeadline(time.Now().Add(pongWait))

		if err != nil {
			return err
		}

		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Socket Error: %v", err)
			}
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			continue
		}

		log.Printf("Get message from %s: %s", c.UserID, wsMsg.Type)
	}
}

// WritePump пишет сообщения в сокет (от сервера к браузеру)
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		err := c.Conn.Close()
		if err != nil {
			return
		}
	}()

	for {
		select {
		case message, ok := <-c.Send:
			err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err != nil {
				return
			}

			if !ok {
				err := c.Conn.WriteMessage(websocket.CloseMessage, []byte{})

				if err != nil {
					return
				}

				return
			}

			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait))

			if err != nil {
				return
			}

			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
