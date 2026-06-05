package matchmaking

import (
	"chess-monolith/internal/users"
	"chess-monolith/pkg/elo"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"chess-monolith/internal/game"
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"chess-monolith/internal/ws"

	"github.com/google/uuid"
)

// QueueKey позволяет группировать игроков по режиму И лимиту времени
type QueueKey struct {
	Mode      string
	TimeLimit time.Duration
}

// Matchmaker управляет пулами игроков
type Matchmaker struct {
	rankedQueues map[QueueKey][]*ws.Client
	casualQueues map[QueueKey][]*ws.Client
	mu           sync.Mutex
	registry     *core.Registry
	repo         game.Repository
	userRepo     users.Repository
}

func NewMatchmaker(registry *core.Registry, repo game.Repository, userRepo users.Repository) *Matchmaker {
	return &Matchmaker{
		rankedQueues: make(map[QueueKey][]*ws.Client),
		casualQueues: make(map[QueueKey][]*ws.Client),
		registry:     registry,
		repo:         repo,
		userRepo:     userRepo,
	}
}

// AddPlayer принимает параметр isRanked (true - на рейтинг, false - обычная)
func (m *Matchmaker) AddPlayer(client *ws.Client, mode string, isRanked bool, timeLimit time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, err := m.registry.Get(mode); err != nil {
		log.Printf("User %s requested unknown mode: %s", client.UserID, mode)
		return ws.NewProtocolError(ws.ErrorCodeUnknownMode, fmt.Sprintf("Unknown mode: %s", mode), true)
	}

	m.removePlayerUnsafe(client) // Чтобы игрок не стоял в двух очередях сразу

	key := QueueKey{Mode: mode, TimeLimit: timeLimit}

	if isRanked {
		m.rankedQueues[key] = append(m.rankedQueues[key], client)
		log.Printf("User %s in RANKED queue for %s (%v)", client.UserID, mode, timeLimit)
	} else {
		m.casualQueues[key] = append(m.casualQueues[key], client)
		log.Printf("User %s in CASUAL queue for %s (%v)", client.UserID, mode, timeLimit)
	}

	return nil
}

func (m *Matchmaker) removePlayerUnsafe(client *ws.Client) {
	for key, queue := range m.rankedQueues {
		for i, c := range queue {
			if c.UserID == client.UserID {
				m.rankedQueues[key] = append(queue[:i], queue[i+1:]...)
				return
			}
		}
	}
	for key, queue := range m.casualQueues {
		for i, c := range queue {
			if c.UserID == client.UserID {
				m.casualQueues[key] = append(queue[:i], queue[i+1:]...)
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
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()

		// Обработка обычных игр
		for key, queue := range m.casualQueues {
			for len(queue) >= 2 {
				p1, p2 := queue[0], queue[1]
				m.casualQueues[key] = queue[2:]
				queue = m.casualQueues[key]
				m.startGame(p1, p2, key.Mode, false, key.TimeLimit)
			}
		}

		// Обработка рейтинговых игр
		for key, queue := range m.rankedQueues {
			if len(queue) >= 2 {
				sort.Slice(queue, func(i, j int) bool {
					return queue[i].Rating < queue[j].Rating
				})

				var remaining []*ws.Client
				for i := 0; i < len(queue); {
					if i+1 < len(queue) && abs(queue[i+1].Rating-queue[i].Rating) <= 100 {
						p1, p2 := queue[i], queue[i+1]
						m.startGame(p1, p2, key.Mode, true, key.TimeLimit)
						i += 2
					} else {
						remaining = append(remaining, queue[i])
						i++
					}
				}
				m.rankedQueues[key] = remaining
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

// startGame связывает двух клиентов и запускает таймеры
func (m *Matchmaker) startGame(player1, player2 *ws.Client, mode string, isRanked bool, timeLimit time.Duration) {
	// Используем динамический timeLimit
	sess, err := session.NewSession(m.registry, mode, timeLimit)
	if err != nil {
		log.Printf("Session error: %v", err)
		return
	}

	p1UUID, _ := uuid.Parse(player1.UserID)
	p2UUID, _ := uuid.Parse(player2.UserID)

	newDBGame := &game.Game{
		WhiteID:  p1UUID,
		BlackID:  p2UUID,
		Mode:     mode,
		IsRanked: isRanked,
		Status:   "active",
		Turn:     "white",
	}

	if err := m.repo.CreateGame(newDBGame); err != nil {
		log.Printf("Failed to create game in DB: %v", err)
		player1.SendError(ws.ErrorCodeInternal, "Failed to start game due to server error", false)
		player2.SendError(ws.ErrorCodeInternal, "Failed to start game due to server error", false)
		return
	}

	sess.ID = newDBGame.ID.String()
	sess.IsRanked = isRanked

	player1.ActiveGame = sess
	player1.Color = core.White
	player1.Opponent = player2

	player2.ActiveGame = sess
	player2.Color = core.Black
	player2.Opponent = player1

	sess.OnGameEnd = func(finalStatus string) {
		parsedGameID, _ := uuid.Parse(sess.ID)
		if err := m.repo.UpdateGame(parsedGameID, "{}", finalStatus, string(sess.Turn)); err != nil {
			log.Printf("Failed to update game %s: %v", sess.ID, err)
		} else {
			log.Printf("Game %s saved with status: %s", sess.ID, finalStatus)
		}

		if sess.IsRanked {
			var p1Score float64
			if finalStatus == "draw" {
				p1Score = 0.5
			} else if finalStatus == "white_won" || finalStatus == "white_won_resign" || finalStatus == "white_won_timeout" {
				p1Score = 1.0 // Белые победили
			} else {
				p1Score = 0.0 // Черные победили
			}

			newR1, newR2 := elo.Calculate(player1.Rating, player2.Rating, p1Score)
			p1UUID, _ := uuid.Parse(player1.UserID)
			p2UUID, _ := uuid.Parse(player2.UserID)

			if err := m.userRepo.UpdateRatings(p1UUID, p2UUID, newR1, newR2); err == nil {
				player1.Rating = newR1
				player2.Rating = newR2
				log.Printf("Game %s ELO updated: %s (%d), %s (%d)", sess.ID, player1.UserID, newR1, player2.UserID, newR2)
			}
		}

		player1.SendGameState()
		player2.SendGameState()
	}

	onTimeout := func(status string) {
		sess.EndGame(status)
	}

	go sess.RunTimer(onTimeout)

	player1.SendMatchFound(player2, mode, isRanked, timeLimit)
	player2.SendMatchFound(player1, mode, isRanked, timeLimit)
	player1.SendGameState()
	player2.SendGameState()

	log.Printf("Game started: %s (White) vs %s (Black) | %s | Ranked: %v | Time: %v",
		player1.UserID, player2.UserID, mode, isRanked, timeLimit)
}
