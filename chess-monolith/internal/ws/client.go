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
	pingPeriod     = networkHeartbeatPing
	maxMessageSize = 16 * 1024 // JOIN_QUEUE can include a compact visual snapshot.
)

// QueueManager абстрагирует логику матчмейкинга от сокетов
type QueueManager interface {
	AddPlayer(c *Client, mode string, boardSize int, isRanked bool, timeLimit time.Duration) error
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
	VisualState  string

	ActiveGame *session.GameSession
	Color      core.Color
	Opponent   *Client

	networkActivity networkActivityState
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

		c.handleDisconnectLoss()

		c.Hub.Unregister <- c
		err := c.Conn.Close()
		if err != nil {
			return
		}
	}()

	networkDone := make(chan struct{})
	defer close(networkDone)

	c.Conn.SetReadLimit(maxMessageSize)
	err := c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	if err != nil {
		return
	}

	c.Conn.SetPongHandler(func(string) error {
		now := time.Now()
		c.markNetworkActivity(now)
		err := c.Conn.SetReadDeadline(now.Add(pongWait))

		if err != nil {
			return err
		}

		return nil
	})

	c.markNetworkActivity(time.Now())
	go c.monitorNetworkActivity(networkDone)

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Socket Error: %v", err)
			}
			break
		}

		c.markNetworkActivity(time.Now())

		var wsMsg Message
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			c.SendError(ErrorCodeInvalidMessage, "Invalid websocket message", true)
			continue
		}

		switch wsMsg.Type {
		case "MOVE":
			var moveReq struct {
				From      string `json:"from"`
				To        string `json:"to"`
				Promotion string `json:"promotion,omitempty"`
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

			err = c.ActiveGame.MakeMoveWithPromotion(from, to, moveReq.Promotion)
			if err != nil {
				c.ActiveGame.Mu.Lock()
				status := c.ActiveGame.Status
				c.ActiveGame.Mu.Unlock()
				if status != "active" {
					c.ActiveGame.EndGame(status)
					continue
				}

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
				Mode        string          `json:"mode"`
				BoardSize   int             `json:"board_size"`
				IsRanked    bool            `json:"is_ranked"`
				TimeLimit   int             `json:"time_limit"` // Время в минутах
				VisualState json.RawMessage `json:"visual_state"`
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

			boardSize := joinReq.BoardSize
			if boardSize == 0 {
				boardSize = BoardSizeForMode(joinReq.Mode)
			}
			expectedBoardSize := BoardSizeForMode(joinReq.Mode)
			if expectedBoardSize != 0 && boardSize != expectedBoardSize {
				c.SendError(ErrorCodeInvalidMessage, "board_size does not match mode", true)
				continue
			}

			timeLimit := time.Duration(joinReq.TimeLimit) * time.Minute
			c.VisualState = NormalizeVisualState(joinReq.VisualState)
			if err := c.QueueManager.AddPlayer(c, joinReq.Mode, boardSize, joinReq.IsRanked, timeLimit); err != nil {
				c.SendProtocolError(err, ErrorCodeQueueFailed)
				continue
			}

			c.SendQueueJoined(joinReq.Mode, boardSize, joinReq.IsRanked, timeLimit)

		case "CANCEL_QUEUE":
			if c.QueueManager == nil {
				c.SendError(ErrorCodeQueueFailed, "Queue manager is not available", true)
				continue
			}

			c.QueueManager.RemovePlayer(c)

		case "RESIGN":
			c.handleResign()

		case MessageTypeLeaveGame:
			c.handleLeaveGame()

		case "DRAW_OFFER":
			c.handleDrawOffer()

		case "DRAW_ACCEPT":
			c.handleDrawAccept()

		case "DRAW_DECLINE":
			c.handleDrawDecline()

		case "CHAT_STICKER":
			var stickerReq ChatStickerRequest
			if err := json.Unmarshal(wsMsg.Payload, &stickerReq); err != nil {
				c.SendError(ErrorCodeInvalidMessage, "Invalid chat sticker payload", true)
				continue
			}
			c.handleChatSticker(stickerReq)

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
