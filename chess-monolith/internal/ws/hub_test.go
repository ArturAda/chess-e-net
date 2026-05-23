// Файл: internal/ws/hub_test.go
package ws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := &Client{Send: make(chan []byte, 256)}

	// 1. Тест регистрации
	hub.Register <- client
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.Len(), "Client should be registered")

	// 2. Отключение
	hub.Unregister <- client
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 0, hub.Len(), "Client should be unregistered")
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

	blockedClient := &Client{Send: make(chan []byte, 0)}

	hub.Register <- blockedClient
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 1, hub.Len(), "Client should be added")

	hub.Broadcast <- []byte("test")
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, 0, hub.Len(), "Blocked client should be removed")

	_, ok := <-blockedClient.Send
	assert.False(t, ok, "Client channel should be closed")
}
