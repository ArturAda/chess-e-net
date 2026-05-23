package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run() // Запускаем хаб

	client := &Client{Send: make(chan []byte, 256)}

	// 1. Тест регистрации
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	assert.True(t, hub.Clients[client], "Client should be registered")
	assert.Equal(t, 1, len(hub.Clients))

	// 2. Отключение
	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	assert.False(t, hub.Clients[client], "Client should be unregistered")
	assert.Equal(t, 0, len(hub.Clients))
}

func TestHub_Broadcast_Success(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := &Client{Send: make(chan []byte, 10)}
	client2 := &Client{Send: make(chan []byte, 10)}

	hub.Register <- client1
	hub.Register <- client2
	time.Sleep(10 * time.Millisecond)

	testMessage := []byte(`{"type":"CHAT","payload":"hello"}`)
	hub.Broadcast <- testMessage

	// Проверяем, что оба клиента получили сообщение
	for _, client := range []*Client{client1, client2} {
		select {
		case msg := <-client.Send:
			assert.Equal(t, testMessage, msg)
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Timeout waiting for broadcast message")
		}
	}
}

func TestHub_Broadcast_Backpressure(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	// Создаем клиента с каналом нулевой емкости (зависший клиент)
	blockedClient := &Client{Send: make(chan []byte, 0)}

	hub.Register <- blockedClient
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, len(hub.Clients), "Client should be added")

	// Отправляем сообщение. Так как канал blockedClient заблокирован,
	// сработает секция default в hub.Run(), которая закроет канал и удалит клиента.
	hub.Broadcast <- []byte("test")
	time.Sleep(10 * time.Millisecond)

	// Проверяем, что зависший клиент был принудительно кикнут
	assert.Equal(t, 0, len(hub.Clients), "Blocked client should be removed")

	// Проверяем, что канал закрыт (чтение из закрытого канала возвращает false)
	_, ok := <-blockedClient.Send
	assert.False(t, ok, "Client channel should be closed")
}
