package ws

// Hub хранит активные подключения и управляет их регистрацией
type Hub struct {
	// Активные клиенты (используем map для быстрого O(1) поиска и удаления)
	Clients map[*Client]bool

	// Входящие сообщения от клиентов (опционально, если хотим глобальный чат)
	Broadcast chan []byte

	// Запросы на регистрацию
	Register chan *Client

	// Запросы на отключение
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

// Run - это select-loop, который постоянно слушает каналы регистрации, отключения и вещания сообщений.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			// Пользователь подключился
			h.Clients[client] = true

		case client := <-h.Unregister:
			// Пользователь отключился (закрыл вкладку браузера)
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}

		case message := <-h.Broadcast:
			// Если нужно отправить сообщение ВСЕМ пользователям онлайн
			for client := range h.Clients {
				select {
				case client.Send <- message:
				default:
					// Если канал клиента переполнен (завис), принудительно отключаем
					close(client.Send)
					delete(h.Clients, client)
				}
			}
		}
	}
}
