package matchmaking

import (
	"chess-monolith/internal/users"
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

// QueueKey позволяет группировать игроков по режиму, размеру доски и лимиту времени.
type QueueKey struct {
	Mode      string
	BoardSize int
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
func (m *Matchmaker) AddPlayer(client *ws.Client, mode string, boardSize int, isRanked bool, timeLimit time.Duration) error {
	if _, err := m.registry.Get(mode); err != nil {
		log.Printf("User %s requested unknown mode: %s", client.UserID, mode)
		return ws.NewProtocolError(ws.ErrorCodeUnknownMode, fmt.Sprintf("Unknown mode: %s", mode), true)
	}

	boardSize = normalizeBoardSize(mode, boardSize)
	if expectedBoardSize := ws.BoardSizeForMode(mode); expectedBoardSize != 0 && boardSize != expectedBoardSize {
		return ws.NewProtocolError(ws.ErrorCodeInvalidMessage, "board_size does not match mode", true)
	}

	key := QueueKey{Mode: mode, BoardSize: boardSize, TimeLimit: timeLimit}

	if isRanked {
		rating, err := m.loadScopedRating(client, key)
		if err != nil {
			return err
		}
		client.Rating = rating
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.removePlayerUnsafe(client) // Чтобы игрок не стоял в двух очередях сразу

	if isRanked {
		m.rankedQueues[key] = append(m.rankedQueues[key], client)
		log.Printf("User %s in RANKED queue for %s %dx%d (%v), rating %d", client.UserID, mode, boardSize, boardSize, timeLimit, client.Rating)
	} else {
		m.casualQueues[key] = append(m.casualQueues[key], client)
		log.Printf("User %s in CASUAL queue for %s %dx%d (%v)", client.UserID, mode, boardSize, boardSize, timeLimit)
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
				m.startGame(p1, p2, key.Mode, key.BoardSize, false, key.TimeLimit)
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
						m.startGame(p1, p2, key.Mode, key.BoardSize, true, key.TimeLimit)
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
func (m *Matchmaker) startGame(player1, player2 *ws.Client, mode string, boardSize int, isRanked bool, timeLimit time.Duration) {
	// Используем динамический timeLimit
	sess, err := session.NewSession(m.registry, mode, timeLimit)
	if err != nil {
		log.Printf("Session error: %v", err)
		return
	}

	p1UUID, _ := uuid.Parse(player1.UserID)
	p2UUID, _ := uuid.Parse(player2.UserID)

	gameID := uuid.New()
	sess.ID = gameID.String()

	actualBoardSize := boardSize
	if sess.Board != nil && sess.Board.Width > 0 {
		actualBoardSize = sess.Board.Width
	}

	initialBoardState, err := sess.ExportPersistedStateJSON()
	if err != nil {
		log.Printf("Failed to encode initial board state: %v", err)
		initialBoardState = "{}"
	}

	newDBGame := &game.Game{
		ID:               gameID,
		WhiteID:          p1UUID,
		BlackID:          p2UUID,
		Mode:             mode,
		BoardSize:        actualBoardSize,
		TimeLimitMs:      timeLimit.Milliseconds(),
		IsRanked:         isRanked,
		Status:           "active",
		Turn:             "white",
		BoardState:       initialBoardState,
		WhiteVisualState: ws.NormalizeVisualStateString(player1.VisualState),
		BlackVisualState: ws.NormalizeVisualStateString(player2.VisualState),
	}

	if err := m.repo.CreateGame(newDBGame); err != nil {
		log.Printf("Failed to create game in DB: %v", err)
		player1.SendError(ws.ErrorCodeInternal, "Failed to start game due to server error", false)
		player2.SendError(ws.ErrorCodeInternal, "Failed to start game due to server error", false)
		return
	}

	sess.IsRanked = isRanked

	player1.ActiveGame = sess
	player1.Color = core.White
	player1.Opponent = player2

	player2.ActiveGame = sess
	player2.Color = core.Black
	player2.Opponent = player1

	sess.OnGameEnd = func(finalStatus string) {
		parsedGameID, _ := uuid.Parse(sess.ID)
		boardState, err := sess.ExportPersistedStateJSON()
		if err != nil {
			log.Printf("Failed to encode final board state for game %s: %v", sess.ID, err)
			boardState = "{}"
		}

		if err := m.repo.UpdateGame(parsedGameID, boardState, finalStatus, string(sess.Turn)); err != nil {
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

			p1UUID, _ := uuid.Parse(player1.UserID)
			p2UUID, _ := uuid.Parse(player2.UserID)

			scope := users.BoardRatingScope(actualBoardSize, timeLimit.Milliseconds())
			newR1, newR2, err := m.userRepo.ApplyRatingResult(p1UUID, p2UUID, scope, p1Score)
			if err == nil {
				player1.Rating = newR1
				player2.Rating = newR2
				log.Printf("Game %s ELO updated: %s (%d), %s (%d)", sess.ID, player1.UserID, newR1, player2.UserID, newR2)
			} else {
				log.Printf("Failed to update scoped ELO for game %s: %v", sess.ID, err)
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

func normalizeBoardSize(mode string, boardSize int) int {
	if boardSize > 0 {
		return boardSize
	}
	if inferred := ws.BoardSizeForMode(mode); inferred > 0 {
		return inferred
	}
	return 8
}

func (m *Matchmaker) loadScopedRating(client *ws.Client, key QueueKey) (int, error) {
	if m.userRepo == nil {
		return 0, ws.NewProtocolError(ws.ErrorCodeQueueFailed, "Rating repository is not available", false)
	}

	userID, err := uuid.Parse(client.UserID)
	if err != nil {
		return 0, ws.NewProtocolError(ws.ErrorCodeInvalidMessage, "Invalid user id", false)
	}

	rating, err := m.userRepo.GetOrCreateRating(userID, users.BoardRatingScope(key.BoardSize, key.TimeLimit.Milliseconds()))
	if err != nil {
		log.Printf("Failed to load scoped rating for user %s: %v", client.UserID, err)
		return 0, ws.NewProtocolError(ws.ErrorCodeQueueFailed, "Failed to load rating", true)
	}

	return rating.Rating, nil
}
