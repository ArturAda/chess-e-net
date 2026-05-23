package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

// mockServeWS - заглушка хендлера для тестов без JWT авторизации,
// чтобы сфокусироваться только на работе клиента (ReadPump / WritePump)
func mockServeWS(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &Client{
			Hub:    hub,
			Conn:   conn,
			Send:   make(chan []byte, 256),
			UserID: "test_user_id",
		}
		client.Hub.Register <- client

		go client.WritePump()
		client.ReadPump()
	}
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
		err := conn.Close()
		if err != nil {
			return
		}
	}(conn)

	time.Sleep(50 * time.Millisecond) // Ждем окончания регистрации

	// Берем единственного зарегистрированного клиента из хаба
	var serverSideClient *Client
	for client := range hub.Clients {
		serverSideClient = client
		break
	}
	assert.NotNil(t, serverSideClient)

	// Имитируем отправку сообщения сервером
	testMsg := []byte(`{"type":"MOVE", "payload": "e2e4"}`)
	serverSideClient.Send <- testMsg

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
			Hub:  hub,
			Conn: conn,
			Send: ch,
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
		err := conn.Close()
		if err != nil {
			return
		}
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
		err := conn.Close()
		if err != nil {
			return
		}
	}(conn)

	time.Sleep(50 * time.Millisecond) // Ждем регистрацию

	// Формируем сообщение, которое соответствует структуре WSMessage
	wsMsg := WSMessage{
		Type:    "JOIN_QUEUE",
		Payload: json.RawMessage(`{"mode": "blitz"}`),
	}

	// Клиент пишет сообщение в сокет
	err = conn.WriteJSON(wsMsg)
	assert.NoError(t, err)

	// Даем ReadPump время прочитать и обработать (выведет лог)
	time.Sleep(50 * time.Millisecond)

	// Проверяем, что соединение все еще живо и клиент не удален из-за ошибки
	assert.Equal(t, 1, len(hub.Clients))
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
		err := conn.Close()
		if err != nil {

		}
	}(conn)

	time.Sleep(50 * time.Millisecond) // Ждем регистрацию

	// Отправляем мусор вместо JSON
	err = conn.WriteMessage(websocket.TextMessage, []byte("this is not a valid json!!!"))
	assert.NoError(t, err)

	// Даем ReadPump время попытаться распарсить сообщение
	time.Sleep(50 * time.Millisecond)

	// Проверяем, что соединение не упало, и клиент все еще в хабе
	assert.Equal(t, 1, len(hub.Clients), "Client should remain connected after sending invalid JSON")
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
	assert.Equal(t, 1, len(hub.Clients))

	err = conn.UnderlyingConn().Close()
	if err != nil {
		return
	}

	time.Sleep(100 * time.Millisecond) // Ждем отработки defer

	// Клиент должен быть удален
	assert.Equal(t, 0, len(hub.Clients), "Client should be removed after hard disconnect")
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
	assert.Equal(t, 1, len(hub.Clients), "Client should be connected")

	// Клиент разрывает соединение (закрыл вкладку)
	conn.Close()

	// Ждем, пока ReadPump поймает ошибку чтения,
	// вызовет defer и отправит сигнал в Unregister
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 0, len(hub.Clients), "Client should be removed after disconnect")
}
