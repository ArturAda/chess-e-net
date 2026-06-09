package ws

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wsTestMode struct{}

func (m *wsTestMode) Setup() *core.Board { return core.NewBoard(8, 8) }
func (m *wsTestMode) ValidateMove(_ *core.Board, _ core.Color, _, _ core.Pos) error {
	return nil
}
func (m *wsTestMode) ApplyMoveSideEffects(_ *core.Board, _, _ core.Pos, _ core.MoveOptions) {}
func (m *wsTestMode) CheckState(_ *core.Board, _ core.Color) string                         { return "active" }

func TestProtocolErrorAndSendProtocolError(t *testing.T) {
	var nilErr *ProtocolError
	assert.Equal(t, "", nilErr.Error())

	protocolErr := NewProtocolError(ErrorCodeUnknownMode, "Unknown mode", true)
	assert.Equal(t, "Unknown mode", protocolErr.Error())

	client := &Client{Send: make(chan []byte, 4)}
	client.SendProtocolError(protocolErr, ErrorCodeInternal)

	msg := readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeError, msg.Type)
	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, ErrorCodeUnknownMode, payload.Code)
	assert.Equal(t, "Unknown mode", payload.Message)
	assert.True(t, payload.Recoverable)

	client.SendProtocolError(errors.New("plain error"), ErrorCodeQueueFailed)
	msg = readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeError, msg.Type)
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, ErrorCodeQueueFailed, payload.Code)
	assert.Equal(t, "plain error", payload.Message)
	assert.True(t, payload.Recoverable)
}

func TestClientSendMessageNoopAndMarshalFailure(t *testing.T) {
	var nilClient *Client
	nilClient.SendMessage("IGNORED", nil)
	(&Client{}).SendMessage("IGNORED", nil)

	client := &Client{Send: make(chan []byte, 1)}
	client.SendMessage("BROKEN", func() {})

	msg := readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeError, msg.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, ErrorCodeInternal, payload.Code)
	assert.False(t, payload.Recoverable)
}

func TestClientSendQueueJoinedAndMoveRejected(t *testing.T) {
	client := &Client{Send: make(chan []byte, 2)}

	client.SendQueueJoined("modern10", 10, true, 5*time.Minute)
	msg := readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeQueueJoined, msg.Type)
	var queuePayload QueueJoinedPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &queuePayload))
	assert.Equal(t, "modern10", queuePayload.Mode)
	assert.Equal(t, 10, queuePayload.BoardSize)
	assert.True(t, queuePayload.IsRanked)
	assert.Equal(t, 5, queuePayload.TimeLimitMinutes)

	client.SendMoveRejected("e2", "e5", ErrorCodeInvalidMove, "bad move", true)
	msg = readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeMoveRejected, msg.Type)
	var rejectedPayload MoveRejectedPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &rejectedPayload))
	assert.Equal(t, "e2", rejectedPayload.From)
	assert.Equal(t, "e5", rejectedPayload.To)
	assert.Equal(t, ErrorCodeInvalidMove, rejectedPayload.Code)
	assert.Equal(t, "bad move", rejectedPayload.Message)
	assert.True(t, rejectedPayload.Recoverable)
}

func TestClientSendMatchFoundBuildsPayload(t *testing.T) {
	client := &Client{
		Send: make(chan []byte, 1),
		ActiveGame: &session.GameSession{
			ID:    "game-42",
			Board: core.NewBoard(12, 12),
		},
		Color: core.Black,
	}
	opponent := &Client{
		UserID: "opponent-id",
		Rating: 1310,
	}

	client.SendMatchFound(opponent, "modern12", true, time.Minute)

	msg := readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeMatchFound, msg.Type)

	var payload MatchFoundPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, "game-42", payload.GameID)
	assert.Equal(t, "modern12", payload.Mode)
	assert.Equal(t, 12, payload.BoardSize)
	assert.Equal(t, "black", payload.PlayerColor)
	assert.True(t, payload.IsRanked)
	assert.Equal(t, 1, payload.TimeLimitMinutes)
	assert.Equal(t, OpponentPayload{
		UserID:   "opponent-id",
		Username: "Player opponent-id",
		Rating:   1310,
	}, payload.Opponent)
}

func TestOpponentPayloadAndBoardSizeHelpers(t *testing.T) {
	assert.Equal(t, OpponentPayload{}, buildOpponentPayload(nil))
	assert.Equal(t, OpponentPayload{UserID: "user-1", Username: "Alice", Rating: 1205}, buildOpponentPayload(&Client{
		UserID:   "user-1",
		Username: "Alice",
		Rating:   1205,
	}))

	assert.Equal(t, 8, BoardSizeForMode("classic"))
	assert.Equal(t, 10, BoardSizeForMode("modern10"))
	assert.Equal(t, 12, BoardSizeForMode("modern12"))
	assert.Equal(t, 0, BoardSizeForMode("custom"))
}

func TestClientSendGameState(t *testing.T) {
	game := &session.GameSession{
		ID:        "game-state-1",
		Status:    "active",
		Turn:      core.White,
		Board:     core.NewBoard(8, 8),
		Mode:      &wsTestMode{},
		WhiteTime: time.Minute,
		BlackTime: time.Minute,
		LastMove:  time.Now(),
	}
	client := &Client{
		Send:       make(chan []byte, 1),
		ActiveGame: game,
		Color:      core.White,
	}

	client.SendGameState()

	msg := readClientMessage(t, client.Send)
	assert.Equal(t, MessageTypeGameState, msg.Type)
	var payload session.GameStateDTO
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, "game-state-1", payload.GameID)
	assert.Equal(t, "white", payload.PlayerColor)
	assert.Equal(t, 8, payload.BoardSize)
}

func TestNetworkAndVisualStateHelpers(t *testing.T) {
	var nilClient *Client
	nilClient.clearNetworkWaiting()
	assert.Equal(t, "player", nilClient.UsernameOrFallback())

	client := &Client{UserID: "user-1"}
	assert.Equal(t, "Player user-1", client.UsernameOrFallback())
	client.Username = "Alice"
	assert.Equal(t, "Alice", client.UsernameOrFallback())

	client.networkActivity.waiting = true
	client.networkActivity.expiresAt = time.Now().Add(time.Minute)
	client.clearNetworkWaiting()
	assert.False(t, client.networkActivity.waiting)
	assert.True(t, client.networkActivity.expiresAt.IsZero())

	assert.JSONEq(t, `{"a":1}`, NormalizeVisualStateString(`{"a":1}`))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualStateString(""))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualStateString("{bad"))
}

func TestDrawResponseErrorPayloads(t *testing.T) {
	white, black, _ := newActiveGameTestClients()
	offer := session.DrawOfferState{
		ID:              "offer-1",
		OfferedBy:       core.White,
		OfferedByUserID: "white-user",
	}

	white.handleDrawResponseError(offer, session.ErrDrawOfferExpired)

	expiredMsg := readClientMessage(t, white.Send)
	assert.Equal(t, MessageTypeDrawExpired, expiredMsg.Type)
	assert.Equal(t, MessageTypeDrawExpired, readClientMessage(t, black.Send).Type)
	errorMsg := readClientMessage(t, white.Send)
	assert.Equal(t, MessageTypeError, errorMsg.Type)
	var errorPayload ErrorPayload
	require.NoError(t, json.Unmarshal(errorMsg.Payload, &errorPayload))
	assert.Equal(t, ErrorCodeDrawOfferState, errorPayload.Code)
	assert.Equal(t, "Draw offer expired", errorPayload.Message)

	white.handleDrawResponseError(session.DrawOfferState{}, session.ErrDrawOfferOwnResponse)
	errorMsg = readClientMessage(t, white.Send)
	require.NoError(t, json.Unmarshal(errorMsg.Payload, &errorPayload))
	assert.Equal(t, "You cannot respond to your own draw offer", errorPayload.Message)

	white.handleDrawResponseError(session.DrawOfferState{}, session.ErrDrawOfferNotFound)
	errorMsg = readClientMessage(t, white.Send)
	require.NoError(t, json.Unmarshal(errorMsg.Payload, &errorPayload))
	assert.Equal(t, "No active draw offer", errorPayload.Message)

	white.handleDrawResponseError(session.DrawOfferState{}, errors.New("bad state"))
	errorMsg = readClientMessage(t, white.Send)
	require.NoError(t, json.Unmarshal(errorMsg.Payload, &errorPayload))
	assert.Equal(t, "bad state", errorPayload.Message)
}
