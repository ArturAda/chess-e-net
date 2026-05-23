package matchmaking

import (
	"encoding/json"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"chess-monolith/internal/game"
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"chess-monolith/internal/ws"
)

// Matchmaker управляет пулами игроков
type Matchmaker struct {
	rankedQueues map[string][]*ws.Client
	casualQueues map[string][]*ws.Client
	mu           sync.Mutex
	registry     *core.Registry
	repo         game.Repository
}

func NewMatchmaker(registry *core.Registry, repo game.Repository) *Matchmaker {
	return &Matchmaker{
		rankedQueues: make(map[string][]*ws.Client),
		casualQueues: make(map[string][]*ws.Client),
		registry:     registry,
		repo:         repo,
	}
}

// AddPlayer принимает параметр isRanked (true - на рейтинг, false - обычная)
func (m *Matchmaker) AddPlayer(client *ws.Client, mode string, isRanked bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.registry.Get(mode); err != nil {
		log.Printf("User %s requested unknown mode: %s", client.UserID, mode)
		return
	}

	m.removePlayerUnsafe(client)

	if isRanked {
		m.rankedQueues[mode] = append(m.rankedQueues[mode], client)
		log.Printf("User %s in RANKED queue for %s", client.UserID, mode)
	} else {
		m.casualQueues[mode] = append(m.casualQueues[mode], client)
		log.Printf("User %s in CASUAL queue for %s", client.UserID, mode)
	}
}

func (m *Matchmaker) removePlayerUnsafe(client *ws.Client) {
	for mode, queue := range m.rankedQueues {
		for i, c := range queue {
			if c.UserID == client.UserID {
				m.rankedQueues[mode] = append(queue[:i], queue[i+1:]...)
				return
			}
		}
	}
	for mode, queue := range m.casualQueues {
		for i, c := range queue {
			if c.UserID == client.UserID {
				m.casualQueues[mode] = append(queue[:i], queue[i+1:]...)
				return
			}
		}
	}
}

// RemovePlayer чистит обе очереди при отключении игрока
func (m *Matchmaker) RemovePlayer(client *ws.Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removePlayerUnsafe(client)
}

func (m *Matchmaker) Run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop() // Предотвращаем утечку памяти!

	for range ticker.C {
		m.mu.Lock()

		// Обработка обычных игр по каждому режиму
		for mode, queue := range m.casualQueues {
			for len(queue) >= 2 {
				p1, p2 := queue[0], queue[1]
				m.casualQueues[mode] = queue[2:] // Срезаем первых двух
				queue = m.casualQueues[mode]     // Обновляем локальную ссылку для цикла
				m.startGame(p1, p2, mode, false)
			}
		}

		// Обработка рейтинговых игр по каждому режиму
		for mode, queue := range m.rankedQueues {
			if len(queue) >= 2 {
				sort.Slice(queue, func(i, j int) bool {
					return queue[i].Rating < queue[j].Rating
				})

				var remaining []*ws.Client
				for i := 0; i < len(queue); {
					if i+1 < len(queue) && abs(queue[i+1].Rating-queue[i].Rating) <= 100 {
						p1, p2 := queue[i], queue[i+1]
						m.startGame(p1, p2, mode, true)
						i += 2
					} else {
						remaining = append(remaining, queue[i])
						i++
					}
				}
				m.rankedQueues[mode] = remaining
			}
		}

		m.mu.Unlock()
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (m *Matchmaker) startGame(p1, p2 *ws.Client, mode string, isRanked bool) {
	// Теперь берем правила именно для того режима, который просили игроки
	sess, err := session.NewSession(m.registry, mode)
	if err != nil {
		log.Printf("Session error: %v", err)
		return
	}

	gameID := uuid.New()
	wID, _ := uuid.Parse(p1.UserID)
	bID, _ := uuid.Parse(p2.UserID)

	boardState, _ := json.Marshal(sess.Board)

	newGame := &game.Game{
		ID:         gameID,
		WhiteID:    wID,
		BlackID:    bID,
		Mode:       mode,     // Сохраняем режим в БД
		IsRanked:   isRanked, // Сохраняем флаг типа матча
		BoardState: string(boardState),
		Status:     "active",
	}

	if err := m.repo.CreateGame(newGame); err != nil {
		log.Printf("Ошибка БД: %v", err)
		return
	}

	log.Printf("Матч запущен! ID: %s, Mode: %s, Ranked: %v", gameID, mode, isRanked)

	notifyPlayer(p1, gameID.String(), "w", isRanked)
	notifyPlayer(p2, gameID.String(), "b", isRanked)
}

func notifyPlayer(client *ws.Client, gameID string, color string, isRanked bool) {
	msg := map[string]interface{}{
		"gameId":   gameID,
		"color":    color,
		"isRanked": isRanked,
	}
	payload, _ := json.Marshal(msg)

	wsMsg := ws.WSMessage{
		Type:    "MATCH_FOUND",
		Payload: payload,
	}
	bytes, _ := json.Marshal(wsMsg)
	client.Send <- bytes
}
