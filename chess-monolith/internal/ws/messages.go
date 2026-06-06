package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	MessageTypeGameState    = "GAME_STATE"
	MessageTypeQueueJoined  = "QUEUE_JOINED"
	MessageTypeMatchFound   = "MATCH_FOUND"
	MessageTypeMoveRejected = "MOVE_REJECTED"
	MessageTypeError        = "ERROR"
	MessageTypeDrawOffer    = "DRAW_OFFER"
	MessageTypeDrawAccepted = "DRAW_ACCEPTED"
	MessageTypeDrawDecline  = "DRAW_DECLINE"
	MessageTypeDrawExpired  = "DRAW_EXPIRED"
)

const (
	ErrorCodeInvalidMessage  = "INVALID_MESSAGE"
	ErrorCodeUnknownMessage  = "UNKNOWN_MESSAGE"
	ErrorCodeUnknownMode     = "UNKNOWN_MODE"
	ErrorCodeQueueFailed     = "QUEUE_FAILED"
	ErrorCodeNotInGame       = "NOT_IN_GAME"
	ErrorCodeNotYourTurn     = "NOT_YOUR_TURN"
	ErrorCodeInvalidMove     = "INVALID_MOVE"
	ErrorCodeGameAlreadyOver = "GAME_ALREADY_OVER"
	ErrorCodeDrawOfferActive = "DRAW_OFFER_ACTIVE"
	ErrorCodeDrawOfferState  = "DRAW_OFFER_STATE"
	ErrorCodeInternal        = "INTERNAL_ERROR"
)

type ErrorPayload struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

type QueueJoinedPayload struct {
	Mode             string `json:"mode"`
	BoardSize        int    `json:"board_size"`
	IsRanked         bool   `json:"is_ranked"`
	TimeLimitMinutes int    `json:"time_limit_minutes"`
}

type MatchFoundPayload struct {
	GameID           string          `json:"game_id"`
	Mode             string          `json:"mode"`
	BoardSize        int             `json:"board_size"`
	PlayerColor      string          `json:"player_color"`
	IsRanked         bool            `json:"is_ranked"`
	TimeLimitMinutes int             `json:"time_limit_minutes"`
	Opponent         OpponentPayload `json:"opponent"`
}

type OpponentPayload struct {
	UserID   string `json:"user_id"`
	Username string `json:"username,omitempty"`
	Rating   int    `json:"rating"`
}

type MoveRejectedPayload struct {
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
}

type DrawOfferPayload struct {
	OfferID         string    `json:"offer_id"`
	OfferedBy       string    `json:"offered_by"`
	OfferedByUserID string    `json:"offered_by_user_id"`
	ExpiresAt       time.Time `json:"expires_at"`
	ExpiresInMs     int64     `json:"expires_in_ms"`
	Message         string    `json:"message"`
}

type DrawOfferResultPayload struct {
	OfferID           string `json:"offer_id"`
	OfferedBy         string `json:"offered_by"`
	OfferedByUserID   string `json:"offered_by_user_id"`
	RespondedBy       string `json:"responded_by,omitempty"`
	RespondedByUserID string `json:"responded_by_user_id,omitempty"`
	Message           string `json:"message"`
}

type ProtocolError struct {
	Code        string
	Message     string
	Recoverable bool
}

func NewProtocolError(code, message string, recoverable bool) *ProtocolError {
	return &ProtocolError{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
	}
}

func (e *ProtocolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (c *Client) SendMessage(messageType string, payload any) {
	if c == nil || c.Send == nil {
		return
	}

	msg := Message{Type: messageType}
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			c.SendError(ErrorCodeInternal, "Failed to encode websocket payload", false)
			return
		}
		msg.Payload = payloadBytes
	}

	raw, err := json.Marshal(msg)
	if err != nil {
		c.SendError(ErrorCodeInternal, "Failed to encode websocket message", false)
		return
	}
	c.Send <- raw
}

func (c *Client) SendError(code, message string, recoverable bool) {
	c.SendMessage(MessageTypeError, ErrorPayload{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
	})
}

func (c *Client) SendProtocolError(err error, fallbackCode string) {
	var protocolErr *ProtocolError
	if errors.As(err, &protocolErr) {
		c.SendError(protocolErr.Code, protocolErr.Message, protocolErr.Recoverable)
		return
	}
	c.SendError(fallbackCode, err.Error(), true)
}

func (c *Client) SendQueueJoined(mode string, boardSize int, isRanked bool, timeLimit time.Duration) {
	c.SendMessage(MessageTypeQueueJoined, QueueJoinedPayload{
		Mode:             mode,
		BoardSize:        boardSize,
		IsRanked:         isRanked,
		TimeLimitMinutes: int(timeLimit / time.Minute),
	})
}

func (c *Client) SendMatchFound(opponent *Client, mode string, isRanked bool, timeLimit time.Duration) {
	if c == nil || c.ActiveGame == nil {
		return
	}

	boardSize := 0
	if c.ActiveGame.Board != nil {
		boardSize = c.ActiveGame.Board.Width
	}

	c.SendMessage(MessageTypeMatchFound, MatchFoundPayload{
		GameID:           c.ActiveGame.ID,
		Mode:             mode,
		BoardSize:        boardSize,
		PlayerColor:      string(c.Color),
		IsRanked:         isRanked,
		TimeLimitMinutes: int(timeLimit / time.Minute),
		Opponent:         buildOpponentPayload(opponent),
	})
}

func (c *Client) SendMoveRejected(from, to, code, message string, recoverable bool) {
	c.SendMessage(MessageTypeMoveRejected, MoveRejectedPayload{
		From:        from,
		To:          to,
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
	})
}

func buildOpponentPayload(opponent *Client) OpponentPayload {
	if opponent == nil {
		return OpponentPayload{}
	}

	username := opponent.Username
	if username == "" {
		username = fmt.Sprintf("Player %s", opponent.UserID)
	}

	return OpponentPayload{
		UserID:   opponent.UserID,
		Username: username,
		Rating:   opponent.Rating,
	}
}

func BoardSizeForMode(mode string) int {
	switch mode {
	case "classic":
		return 8
	case "modern10":
		return 10
	case "modern12":
		return 12
	default:
		return 0
	}
}
