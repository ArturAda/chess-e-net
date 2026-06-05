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
	AddPlayer(c *Client, mode string, isRanked bool, timeLimit time.Duration) error
	RemovePlayer(c *Client)
}

// Client - это посредник между websocket-соединением и нашим Hub
type Client struct {
	Hub          *Hub
	Conn         *websocket.Conn
	Send         chan []byte
	UserID       string
	Username     string
	Rating       int
	QueueManager QueueManager

	ActiveGame *session.GameSession
	Color      core.Color
	Opponent   *Client
}

// Message - универсальная структура для обмена JSON
type Message struct {
	Type    string          `json:"type"`              // "MOVE", "JOIN_QUEUE", "RESIGN"
	Payload json.RawMessage `json:"payload,omitempty"` // Динамические данные
}

func (c *Client) SendGameState() {
	if c == nil || c.ActiveGame == nil {
		return
	}

	stateDTO := c.ActiveGame.ExportStateForPlayer(c.Color)
	c.SendMessage(MessageTypeGameState, stateDTO)
}

// ReadPump читает сообщения из сокета (от браузера к серверу)
func (c *Client) ReadPump() {
	defer func() {
		if c.QueueManager != nil {
			c.QueueManager.RemovePlayer(c)
		}

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
			c.SendError(ErrorCodeInvalidMessage, "Invalid websocket message", true)
			continue
		}

		switch wsMsg.Type {
		case "MOVE":
			var moveReq struct {
				From string `json:"from"`
				To   string `json:"to"`
			}

			if err := json.Unmarshal(wsMsg.Payload, &moveReq); err != nil {
				c.SendError(ErrorCodeInvalidMessage, "Invalid move payload", true)
				continue
			}

			if c.ActiveGame == nil {
				c.SendMoveRejected(moveReq.From, moveReq.To, ErrorCodeNotInGame, "You are not in a game", true)
				continue
			}

			c.ActiveGame.Mu.Lock()
			currentTurn := c.ActiveGame.Turn
			c.ActiveGame.Mu.Unlock()

			if c.Color != currentTurn {
				c.SendMoveRejected(moveReq.From, moveReq.To, ErrorCodeNotYourTurn, "Not your turn", true)
				continue
			}

			from, err := core.ParseSquare(moveReq.From)
			if err != nil {
				c.SendMoveRejected(moveReq.From, moveReq.To, ErrorCodeInvalidMove, "Invalid from square", true)
				continue
			}

			to, err := core.ParseSquare(moveReq.To)
			if err != nil {
				c.SendMoveRejected(moveReq.From, moveReq.To, ErrorCodeInvalidMove, "Invalid to square", true)
				continue
			}

			err = c.ActiveGame.MakeMove(from, to)
			if err != nil {
				code := ErrorCodeInvalidMove
				recoverable := true
				if err.Error() == "game is over" {
					code = ErrorCodeGameAlreadyOver
					recoverable = false
				}
				c.SendMoveRejected(moveReq.From, moveReq.To, code, err.Error(), recoverable)
				continue
			}

			c.ActiveGame.Mu.Lock()
			status := c.ActiveGame.Status
			c.ActiveGame.Mu.Unlock()

			if status != "active" {
				c.ActiveGame.EndGame(status)
			} else {
				c.SendGameState()
				if c.Opponent != nil {
					c.Opponent.SendGameState()
				}
			}

		case "JOIN_QUEUE":
			// Ожидаем JSON: {"mode": "classic", "is_ranked": true, "time_limit": 10}
			var joinReq struct {
				Mode      string `json:"mode"`
				BoardSize int    `json:"board_size"`
				IsRanked  bool   `json:"is_ranked"`
				TimeLimit int    `json:"time_limit"` // Время в минутах
			}

			if err := json.Unmarshal(wsMsg.Payload, &joinReq); err != nil {
				c.SendError(ErrorCodeInvalidMessage, "Invalid join payload", true)
				continue
			}

			if joinReq.TimeLimit <= 0 {
				joinReq.TimeLimit = 10
			}
			if joinReq.Mode == "" {
				joinReq.Mode = "classic"
			}

			if c.QueueManager == nil {
				c.SendError(ErrorCodeQueueFailed, "Queue manager is not available", true)
				continue
			}

			timeLimit := time.Duration(joinReq.TimeLimit) * time.Minute
			if err := c.QueueManager.AddPlayer(c, joinReq.Mode, joinReq.IsRanked, timeLimit); err != nil {
				c.SendProtocolError(err, ErrorCodeQueueFailed)
				continue
			}

			boardSize := joinReq.BoardSize
			if boardSize == 0 {
				boardSize = BoardSizeForMode(joinReq.Mode)
			}
			c.SendQueueJoined(joinReq.Mode, boardSize, joinReq.IsRanked, timeLimit)

		case "CANCEL_QUEUE":
			if c.QueueManager == nil {
				c.SendError(ErrorCodeQueueFailed, "Queue manager is not available", true)
				continue
			}

			c.QueueManager.RemovePlayer(c)

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
					c.Opponent.SendMessage(MessageTypeDrawOffer, nil)
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
				c.Opponent.SendMessage(MessageTypeDrawDecline, nil)
				log.Printf("Player %s declined the draw.", c.UserID)
			}

		default:
			log.Printf("Unknown message type: %s", wsMsg.Type)
			c.SendError(ErrorCodeUnknownMessage, "Unknown message type: "+wsMsg.Type, true)
		}
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
