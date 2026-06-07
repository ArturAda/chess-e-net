package ws

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServeWS - заглушка хендлера для тестов без JWT авторизации,
// чтобы сфокусироваться только на работе клиента (ReadPump / WritePump)
func mockServeWS(hub *Hub) http.HandlerFunc {
	return mockServeWSWithQueueManager(hub, &DummyQueueManager{})
}

func mockServeWSWithQueueManager(hub *Hub, qm QueueManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &Client{
			Hub:          hub,
			Conn:         conn,
			Send:         make(chan []byte, 256),
			UserID:       "test_user_id",
			QueueManager: qm,
		}
		client.Hub.Register <- client

		go client.WritePump()
		client.ReadPump()
	}
}

func readWSMessage(t *testing.T, conn *websocket.Conn) Message {
	t.Helper()

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	var msg Message
	require.NoError(t, conn.ReadJSON(&msg))
	return msg
}

func readClientMessage(t *testing.T, ch <-chan []byte) Message {
	t.Helper()

	select {
	case raw := <-ch:
		var msg Message
		require.NoError(t, json.Unmarshal(raw, &msg))
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for client message")
		return Message{}
	}
}

func newActiveGameTestClients() (*Client, *Client, *session.GameSession) {
	game := &session.GameSession{
		ID:     "game-1",
		Status: "active",
	}
	white := &Client{
		Send:       make(chan []byte, 16),
		UserID:     "white-user",
		Username:   "White",
		ActiveGame: game,
		Color:      core.White,
	}
	black := &Client{
		Send:       make(chan []byte, 16),
		UserID:     "black-user",
		Username:   "Black",
		ActiveGame: game,
		Color:      core.Black,
	}
	white.Opponent = black
	black.Opponent = white
	return white, black, game
}

func TestClient_WritePump(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Запускаем тестовый HTTP сервер с нашей заглушкой
	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	// Превращаем http:// в ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Подключаемся к серверу как клиент
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond) // Ждем окончания регистрации

	// Берем единственного зарегистрированного клиента из хаба

	// Имитируем отправку сообщения сервером
	testMsg := []byte(`{"type":"MOVE", "payload": "e2e4"}`)
	hub.Broadcast <- testMsg

	// На стороне клиента (нашего тестового соединения) читаем сообщение
	err = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		return
	}

	msgType, receivedMsg, err := conn.ReadMessage()

	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, msgType)
	assert.Equal(t, testMsg, receivedMsg)
}

func TestWritePump_ChannelClosed(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Канал для сигнала о том, что WritePump успешно завершился
	done := make(chan bool)

	// Поднимаем тестовый сервер, чтобы получить реальный объект websocket.Conn
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		assert.NoError(t, err)

		// Создаем канал и СРАЗУ его закрываем
		ch := make(chan []byte)
		close(ch)

		client := &Client{
			Hub:          hub,
			Conn:         conn,
			Send:         ch,
			UserID:       "test_user_id",
			QueueManager: &DummyQueueManager{},
		}

		// Запускаем WritePump. Так как канал ch закрыт (!ok),
		// функция должна отправить CloseMessage и завершить работу.
		client.WritePump()

		// Отправляем сигнал об успешном выходе
		done <- true
	}))
	defer server.Close()

	// Имитируем подключение браузера
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	// Ждем, пока WritePump отработает
	select {
	case <-done:
		// Успех, WritePump корректно завершил работу и вызвал c.Conn.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("WritePump did not exit when channel was closed (Timeout)")
	}
}

func TestClient_ReadPump_ValidJSON(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond) // Ждем регистрацию

	// Формируем сообщение, которое соответствует структуре WSMessage
	wsMsg := Message{
		Type:    "JOIN_QUEUE",
		Payload: json.RawMessage(`{"mode": "classic"}`),
	}

	// Клиент пишет сообщение в сокет
	err = conn.WriteJSON(wsMsg)
	assert.NoError(t, err)

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeQueueJoined, resp.Type)

	// Проверяем, что соединение все еще живо и клиент не удален из-за ошибки
	assert.Equal(t, 1, hub.Len())
}

func TestClient_ReadPump_InvalidJSON(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond) // Ждем регистрацию

	// Отправляем мусор вместо JSON
	err = conn.WriteMessage(websocket.TextMessage, []byte("this is not a valid json!!!"))
	assert.NoError(t, err)

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeError, resp.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(resp.Payload, &payload))
	assert.Equal(t, ErrorCodeInvalidMessage, payload.Code)
	assert.True(t, payload.Recoverable)

	// Проверяем, что соединение не упало, и клиент все еще в хабе
	assert.Equal(t, 1, hub.Len(), "Client should remain connected after sending invalid JSON")
}

func TestClient_ReadPump_SocketError(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.Len())

	conn.UnderlyingConn().Close()

	time.Sleep(100 * time.Millisecond) // Ждем отработки defer

	// Клиент должен быть удален
	assert.Equal(t, 0, hub.Len(), "Client should be removed after hard disconnect")
}

// Тип: Integration Test
// Что проверяет: Автоматическое отключение клиента при закрытии соединения
func TestClient_Disconnect(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.Len(), "Client should be connected")

	// Клиент разрывает соединение (закрыл вкладку)
	conn.Close()

	// Ждем, пока ReadPump поймает ошибку чтения,
	// вызовет defer и отправит сигнал в Unregister
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, hub.Len(), "Client should be removed after disconnect")
}

func TestClient_ReadPump_JoinQueue(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	qm := &DummyQueueManager{Added: make(chan struct{}, 1)}
	server := httptest.NewServer(mockServeWSWithQueueManager(hub, qm))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond)

	wsMsg := Message{
		Type:    "JOIN_QUEUE",
		Payload: json.RawMessage(`{"mode": "classic", "is_ranked": true, "time_limit": 10, "visual_state": {"light_square": {"id": "classic-green"}, "pieces": {"white": "pixel"}}}`),
	}

	err = conn.WriteJSON(wsMsg)
	assert.NoError(t, err)

	select {
	case <-qm.Added:
	case <-time.After(2 * time.Second):
		t.Fatal("JOIN_QUEUE did not call AddPlayer")
	}

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeQueueJoined, resp.Type)

	var payload QueueJoinedPayload
	require.NoError(t, json.Unmarshal(resp.Payload, &payload))
	assert.Equal(t, "classic", payload.Mode)
	assert.Equal(t, 8, payload.BoardSize)
	assert.True(t, payload.IsRanked)
	assert.Equal(t, 10, payload.TimeLimitMinutes)
	require.NotNil(t, qm.LastClient)
	assert.JSONEq(t, `{"light_square":{"id":"classic-green"},"pieces":{"white":"pixel"}}`, qm.LastClient.VisualState)

	assert.Equal(t, 1, hub.Len(), "Client should still be connected after valid JOIN_QUEUE")
}

func TestClient_ReadPump_JoinQueueRejectsBoardSizeMismatch(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	qm := &DummyQueueManager{Added: make(chan struct{}, 1)}
	server := httptest.NewServer(mockServeWSWithQueueManager(hub, qm))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond)

	wsMsg := Message{
		Type:    "JOIN_QUEUE",
		Payload: json.RawMessage(`{"mode": "classic", "board_size": 10, "is_ranked": true, "time_limit": 10}`),
	}

	err = conn.WriteJSON(wsMsg)
	assert.NoError(t, err)

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeError, resp.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(resp.Payload, &payload))
	assert.Equal(t, ErrorCodeInvalidMessage, payload.Code)
	assert.Equal(t, "board_size does not match mode", payload.Message)
	assert.True(t, payload.Recoverable)

	select {
	case <-qm.Added:
		t.Fatal("JOIN_QUEUE with mismatched board_size should not call AddPlayer")
	default:
	}
}

func TestClient_ReadPump_JoinQueueError(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	qm := &DummyQueueManager{
		AddErr: NewProtocolError(ErrorCodeUnknownMode, "Unknown mode: blitz", true),
	}
	server := httptest.NewServer(mockServeWSWithQueueManager(hub, qm))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond)

	wsMsg := Message{
		Type:    "JOIN_QUEUE",
		Payload: json.RawMessage(`{"mode": "blitz", "is_ranked": true, "time_limit": 10}`),
	}

	err = conn.WriteJSON(wsMsg)
	assert.NoError(t, err)

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeError, resp.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(resp.Payload, &payload))
	assert.Equal(t, ErrorCodeUnknownMode, payload.Code)
	assert.Equal(t, "Unknown mode: blitz", payload.Message)
	assert.True(t, payload.Recoverable)
}

func TestClient_ReadPump_CancelQueue(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	qm := &DummyQueueManager{Removed: make(chan struct{}, 1)}
	server := httptest.NewServer(mockServeWSWithQueueManager(hub, qm))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond)

	err = conn.WriteJSON(Message{Type: "CANCEL_QUEUE"})
	assert.NoError(t, err)

	select {
	case <-qm.Removed:
	case <-time.After(2 * time.Second):
		t.Fatal("CANCEL_QUEUE did not call RemovePlayer")
	}

	assert.Equal(t, 1, hub.Len(), "Client should still be connected after CANCEL_QUEUE")
}

func TestClient_ReadPump_UnknownMessage(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond)

	err = conn.WriteJSON(Message{Type: "SOMETHING_ELSE"})
	assert.NoError(t, err)

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeError, resp.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(resp.Payload, &payload))
	assert.Equal(t, ErrorCodeUnknownMessage, payload.Code)
	assert.True(t, payload.Recoverable)
}

func TestClient_ReadPump_Move(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	server := httptest.NewServer(mockServeWS(hub))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer func(conn *websocket.Conn) {
		_ = conn.Close()
	}(conn)

	time.Sleep(50 * time.Millisecond)

	wsMsg := Message{
		Type:    "MOVE",
		Payload: json.RawMessage(`{"from": "e2", "to": "e4"}`),
	}

	err = conn.WriteJSON(wsMsg)
	assert.NoError(t, err)

	resp := readWSMessage(t, conn)
	assert.Equal(t, MessageTypeMoveRejected, resp.Type)

	var payload MoveRejectedPayload
	require.NoError(t, json.Unmarshal(resp.Payload, &payload))
	assert.Equal(t, ErrorCodeNotInGame, payload.Code)
	assert.Equal(t, "e2", payload.From)
	assert.Equal(t, "e4", payload.To)

	assert.Equal(t, 1, hub.Len(), "Client should survive a MOVE payload parse")
}

func TestClient_LeaveGameEndsActiveGameAsCurrentPlayerLoss(t *testing.T) {
	white, _, game := newActiveGameTestClients()

	white.handleLeaveGame()

	assert.Equal(t, "black_won_resign", game.Status)
}

func TestClient_ResignEndsActiveGameAsCurrentPlayerLoss(t *testing.T) {
	_, black, game := newActiveGameTestClients()

	black.handleResign()

	assert.Equal(t, "white_won_resign", game.Status)
}

func TestClient_DisconnectLossIgnoresFinishedGame(t *testing.T) {
	white, _, game := newActiveGameTestClients()
	game.Status = "draw"

	white.handleDisconnectLoss()

	assert.Equal(t, "draw", game.Status)
}

func TestClient_NetworkActivityStartsWaitingAfterIdle(t *testing.T) {
	white, black, _ := newActiveGameTestClients()
	now := time.Now()
	waitStartedAt := now.Add(NetworkIdleThreshold + time.Millisecond)

	white.markNetworkActivity(now)
	white.checkNetworkActivity(waitStartedAt)

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypePlayerNetworkWaiting, whiteMsg.Type)
	assert.Equal(t, MessageTypePlayerNetworkWaiting, blackMsg.Type)

	var payload PlayerNetworkWaitingPayload
	require.NoError(t, json.Unmarshal(blackMsg.Payload, &payload))
	assert.Equal(t, "white-user", payload.UserID)
	assert.Equal(t, "White", payload.Username)
	assert.Equal(t, "white", payload.Color)
	assert.Equal(t, NetworkLossGrace.Milliseconds(), payload.RemainingMs)
	assert.True(t, payload.ExpiresAt.Equal(waitStartedAt.Add(NetworkLossGrace)))
	assert.Equal(t, "Waiting for White network.", payload.Message)
}

func TestClient_NetworkActivityRestoredNotifiesBothPlayers(t *testing.T) {
	white, black, _ := newActiveGameTestClients()
	now := time.Now()

	white.markNetworkActivity(now)
	white.checkNetworkActivity(now.Add(NetworkIdleThreshold + time.Millisecond))
	readClientMessage(t, white.Send)
	readClientMessage(t, black.Send)

	white.markNetworkActivity(now.Add(NetworkIdleThreshold + time.Second))

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypePlayerNetworkRestored, whiteMsg.Type)
	assert.Equal(t, MessageTypePlayerNetworkRestored, blackMsg.Type)

	var payload PlayerNetworkRestoredPayload
	require.NoError(t, json.Unmarshal(blackMsg.Payload, &payload))
	assert.Equal(t, "white-user", payload.UserID)
	assert.Equal(t, "White", payload.Username)
	assert.Equal(t, "white", payload.Color)
	assert.Equal(t, "White network restored.", payload.Message)
}

func TestClient_NetworkActivityTimeoutEndsGameAsCurrentPlayerLoss(t *testing.T) {
	white, black, game := newActiveGameTestClients()
	now := time.Now()
	waitStartedAt := now.Add(NetworkIdleThreshold + time.Millisecond)

	white.markNetworkActivity(now)
	white.checkNetworkActivity(waitStartedAt)
	readClientMessage(t, white.Send)
	readClientMessage(t, black.Send)

	white.checkNetworkActivity(waitStartedAt.Add(NetworkLossGrace + time.Millisecond))

	assert.Equal(t, "black_won_resign", game.Status)
}

func TestClient_DrawOfferSendsPayloadToBothPlayers(t *testing.T) {
	white, black, game := newActiveGameTestClients()

	white.handleDrawOffer()

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypeDrawOffer, whiteMsg.Type)
	assert.Equal(t, MessageTypeDrawOffer, blackMsg.Type)
	require.NotNil(t, game.DrawOffer)

	var payload DrawOfferPayload
	require.NoError(t, json.Unmarshal(whiteMsg.Payload, &payload))
	assert.Equal(t, game.DrawOffer.ID, payload.OfferID)
	assert.Equal(t, "white", payload.OfferedBy)
	assert.Equal(t, "white-user", payload.OfferedByUserID)
	assert.Greater(t, payload.ExpiresInMs, int64(0))
	assert.LessOrEqual(t, payload.ExpiresInMs, session.DrawOfferTTL.Milliseconds())
	assert.Equal(t, "white offered a draw.", payload.Message)
}

func TestClient_DrawAcceptEndsGameAndNotifiesBothPlayers(t *testing.T) {
	white, black, game := newActiveGameTestClients()
	white.handleDrawOffer()
	readClientMessage(t, white.Send)
	readClientMessage(t, black.Send)

	black.handleDrawAccept()

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypeDrawAccepted, whiteMsg.Type)
	assert.Equal(t, MessageTypeDrawAccepted, blackMsg.Type)
	assert.Equal(t, "draw", game.Status)

	var payload DrawOfferResultPayload
	require.NoError(t, json.Unmarshal(blackMsg.Payload, &payload))
	assert.Equal(t, "white", payload.OfferedBy)
	assert.Equal(t, "black", payload.RespondedBy)
	assert.Equal(t, "black-user", payload.RespondedByUserID)
	assert.Equal(t, "Draw offer accepted.", payload.Message)
}

func TestClient_DrawDeclineClearsOfferAndNotifiesBothPlayers(t *testing.T) {
	white, black, game := newActiveGameTestClients()
	white.handleDrawOffer()
	readClientMessage(t, white.Send)
	readClientMessage(t, black.Send)

	black.handleDrawDecline()

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypeDrawDecline, whiteMsg.Type)
	assert.Equal(t, MessageTypeDrawDecline, blackMsg.Type)
	assert.Nil(t, game.DrawOffer)
	assert.Equal(t, "active", game.Status)
}

func TestClient_DrawOffererCannotAcceptOwnOffer(t *testing.T) {
	white, black, game := newActiveGameTestClients()
	white.handleDrawOffer()
	readClientMessage(t, white.Send)
	readClientMessage(t, black.Send)

	white.handleDrawAccept()

	msg := readClientMessage(t, white.Send)
	assert.Equal(t, MessageTypeError, msg.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, ErrorCodeDrawOfferState, payload.Code)
	assert.Equal(t, "You cannot respond to your own draw offer", payload.Message)
	require.NotNil(t, game.DrawOffer)
}

func TestClient_DrawOfferExpirationNotifiesBothPlayers(t *testing.T) {
	white, black, game := newActiveGameTestClients()
	offer, err := game.CreateDrawOffer(core.White, white.UserID, time.Now().Add(-session.DrawOfferTTL-time.Millisecond))
	require.NoError(t, err)

	white.scheduleDrawOfferExpiration(offer)

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypeDrawExpired, whiteMsg.Type)
	assert.Equal(t, MessageTypeDrawExpired, blackMsg.Type)
	assert.Nil(t, game.DrawOffer)
}

func TestClient_ChatStickerRelaysWhitelistedStickerToBothPlayers(t *testing.T) {
	white, black, _ := newActiveGameTestClients()

	white.handleChatSticker(ChatStickerRequest{StickerID: "clown"})

	whiteMsg := readClientMessage(t, white.Send)
	blackMsg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypeChatSticker, whiteMsg.Type)
	assert.Equal(t, MessageTypeChatSticker, blackMsg.Type)

	var payload ChatStickerPayload
	require.NoError(t, json.Unmarshal(blackMsg.Payload, &payload))
	assert.NotEmpty(t, payload.MessageID)
	assert.Equal(t, "game-1", payload.GameID)
	assert.Equal(t, "white-user", payload.SenderUserID)
	assert.Equal(t, "White", payload.SenderUsername)
	assert.Equal(t, "white", payload.SenderColor)
	assert.Equal(t, "clown", payload.StickerID)
	assert.Equal(t, "Clown", payload.Label)
	assert.Equal(t, "images/smiles/clown.png", payload.Src)
	assert.False(t, payload.SentAt.IsZero())
}

func TestClient_ChatStickerNormalizesStickerID(t *testing.T) {
	white, black, _ := newActiveGameTestClients()

	white.handleChatSticker(ChatStickerRequest{StickerID: "  CLOWN  "})

	msg := readClientMessage(t, black.Send)
	assert.Equal(t, MessageTypeChatSticker, msg.Type)

	var payload ChatStickerPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, "clown", payload.StickerID)
}

func TestClient_ChatStickerRejectsUnknownSticker(t *testing.T) {
	white, black, _ := newActiveGameTestClients()

	white.handleChatSticker(ChatStickerRequest{StickerID: "external-url"})

	msg := readClientMessage(t, white.Send)
	assert.Equal(t, MessageTypeError, msg.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, ErrorCodeInvalidSticker, payload.Code)
	assert.Equal(t, "Unknown sticker", payload.Message)
	assert.Empty(t, black.Send)
}

func TestClient_ChatStickerRequiresActiveGame(t *testing.T) {
	white, _, game := newActiveGameTestClients()
	game.Status = "draw"

	white.handleChatSticker(ChatStickerRequest{StickerID: "clown"})

	msg := readClientMessage(t, white.Send)
	assert.Equal(t, MessageTypeError, msg.Type)

	var payload ErrorPayload
	require.NoError(t, json.Unmarshal(msg.Payload, &payload))
	assert.Equal(t, ErrorCodeNotInGame, payload.Code)
}
