package ws

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
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

// QueueManager абстрагирует логику матчмейкинга от сокетов
type QueueManager interface {
	AddPlayer(c *Client, mode string, isRanked bool, timeLimit time.Duration)
}

// Client - это посредник между websocket-соединением и нашим Hub
type Client struct {
	Hub          *Hub
	Conn         *websocket.Conn
	Send         chan []byte
	UserID       string
	Rating       int
	QueueManager QueueManager

	ActiveGame *session.GameSession
	Color      core.Color
	Opponent   *Client
}

// parsePos превращает шахматную координату "e2" в core.Pos (X: 4, Y: 1)
func parsePos(s string) core.Pos {
	if len(s) != 2 {
		return core.Pos{X: -1, Y: -1}
	}
	return core.Pos{
		X: int(s[0] - 'a'),
		Y: int(s[1] - '1'),
	}
}

// Message - универсальная структура для обмена JSON
type Message struct {
	Type    string          `json:"type"`              // "MOVE", "JOIN_QUEUE", "RESIGN"
	Payload json.RawMessage `json:"payload,omitempty"` // Динамические данные
}

// ReadPump читает сообщения из сокета (от браузера к серверу)
func (c *Client) ReadPump() {
	defer func() {
		if c.ActiveGame != nil {
			c.ActiveGame.Mu.Lock()
			status := c.ActiveGame.Status
			c.ActiveGame.Mu.Unlock()

			if status == "active" {
				resignStatus := "white_won_resign"
				if c.Color == core.White {
					resignStatus = "black_won_resign"
				}
				c.ActiveGame.EndGame(resignStatus)
				log.Printf("Player %s disconnected. Technical defeat applied.", c.UserID)
			}
		}

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

		var wsMsg Message
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case "MOVE":
			if c.ActiveGame == nil {
				c.sendError("You are not in a game")
				continue
			}

			c.ActiveGame.Mu.Lock()
			currentTurn := c.ActiveGame.Turn
			c.ActiveGame.Mu.Unlock()

			if c.Color != currentTurn {
				c.sendError("Not your turn")
				continue
			}

			var moveReq struct {
				From string `json:"from"`
				To   string `json:"to"`
			}

			if err := json.Unmarshal(wsMsg.Payload, &moveReq); err != nil {
				c.sendError("Invalid move payload")
				continue
			}

			err := c.ActiveGame.MakeMove(parsePos(moveReq.From), parsePos(moveReq.To))
			if err != nil {
				c.sendError(err.Error())
				continue
			}

			c.ActiveGame.Mu.Lock()
			status := c.ActiveGame.Status
			c.ActiveGame.Mu.Unlock()

			if status != "active" {
				c.ActiveGame.EndGame(status)
			} else {
				stateDTO := c.ActiveGame.ExportState()
				stateBytes, _ := json.Marshal(stateDTO)
				broadcastMsg, _ := json.Marshal(Message{
					Type:    "GAME_STATE",
					Payload: stateBytes,
				})

				c.Send <- broadcastMsg
				if c.Opponent != nil {
					c.Opponent.Send <- broadcastMsg
				}
			}

		case "JOIN_QUEUE":
			// Ожидаем JSON: {"mode": "classic", "is_ranked": true, "time_limit": 10}
			var joinReq struct {
				Mode      string `json:"mode"`
				IsRanked  bool   `json:"is_ranked"`
				TimeLimit int    `json:"time_limit"` // Время в минутах
			}

			if err := json.Unmarshal(wsMsg.Payload, &joinReq); err != nil {
				c.sendError("Invalid join payload")
				continue
			}

			if joinReq.TimeLimit <= 0 {
				joinReq.TimeLimit = 10
			}
			if joinReq.Mode == "" {
				joinReq.Mode = "classic"
			}

			c.QueueManager.AddPlayer(c, joinReq.Mode, joinReq.IsRanked, time.Duration(joinReq.TimeLimit)*time.Minute)

		case "RESIGN":
			if c.ActiveGame != nil {
				c.ActiveGame.Mu.Lock()
				status := c.ActiveGame.Status
				c.ActiveGame.Mu.Unlock()

				if status == "active" {
					resignStatus := "white_won_resign"
					if c.Color == core.White {
						resignStatus = "black_won_resign"
					}
					log.Printf("Player %s resigned.", c.UserID)
					c.ActiveGame.EndGame(resignStatus)
				}
			}

		case "DRAW_OFFER":
			if c.ActiveGame != nil && c.Opponent != nil {
				c.ActiveGame.Mu.Lock()
				status := c.ActiveGame.Status
				c.ActiveGame.Mu.Unlock()

				if status == "active" {
					// Пересылаем предложение оппоненту
					offerMsg, _ := json.Marshal(Message{
						Type: "DRAW_OFFER",
					})
					c.Opponent.Send <- offerMsg
					log.Printf("Player %s offered a draw.", c.UserID)
				}
			}

		case "DRAW_ACCEPT":
			if c.ActiveGame != nil {
				c.ActiveGame.Mu.Lock()
				status := c.ActiveGame.Status
				c.ActiveGame.Mu.Unlock()

				if status == "active" {
					log.Printf("Player %s accepted the draw.", c.UserID)
					// Метод processGameEnd сам всё сохранит, пересчитает Эло
					// и разошлет финальный GAME_STATE обоим игрокам
					c.ActiveGame.EndGame("draw")
				}
			}

		case "DRAW_DECLINE":
			if c.ActiveGame != nil && c.Opponent != nil {
				// Уведомляем первого игрока, что ничья отклонена
				declineMsg, _ := json.Marshal(Message{
					Type: "DRAW_DECLINE",
				})
				c.Opponent.Send <- declineMsg
				log.Printf("Player %s declined the draw.", c.UserID)
			}

		default:
			log.Printf("Unknown message type: %s", wsMsg.Type)
		}
	}
}

func (c *Client) sendError(errMsg string) {
	errPayload, _ := json.Marshal(map[string]string{"message": errMsg})
	msg, _ := json.Marshal(Message{
		Type:    "ERROR",
		Payload: errPayload,
	})
	c.Send <- msg
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
