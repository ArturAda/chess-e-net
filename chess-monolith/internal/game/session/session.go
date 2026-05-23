package session

import (
	"chess-monolith/internal/game/core"
	"errors"
	"fmt"
	"sync"
	"time"
)

type GameSession struct {
	ID       string
	IsRanked bool

	Mode   core.GameMode
	Board  *core.Board
	Turn   core.Color
	Status string
	Mu     sync.Mutex

	WhiteTime time.Duration
	BlackTime time.Duration
	LastMove  time.Time

	StopTimer chan struct{}

	OnGameEnd func(finalStatus string)
	isEnded   bool
}

type GameStateDTO struct {
	Status        string              `json:"status"`             // "active", "white_won", "draw" и т.д.
	Turn          string              `json:"turn"`               // "white" или "black"
	WhiteTimeLeft int64               `json:"white_time_left_ms"` // Оставшееся время в миллисекундах
	BlackTimeLeft int64               `json:"black_time_left_ms"`
	ValidMoves    map[string][]string `json:"valid_moves"` // Пример: {"e2": ["e3", "e4"], "g1": ["f3", "h3"]}}
}

// NewSession создает партию, просто передав строку "classic"
func NewSession(registry *core.Registry, modeName string, timeLimit time.Duration) (*GameSession, error) {
	mode, err := registry.Get(modeName)
	if err != nil {
		return nil, err
	}

	return &GameSession{
		Mode:   mode,
		Board:  mode.Setup(),
		Turn:   core.White,
		Status: "active",

		WhiteTime: timeLimit,
		BlackTime: timeLimit,
		LastMove:  time.Now(),
		StopTimer: make(chan struct{}), // <-- Добавлено
	}, nil
}

func (s *GameSession) MakeMove(from, to core.Pos) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if s.Status != "active" {
		return errors.New("game is over")
	}

	if err := s.Mode.ValidateMove(s.Board, s.Turn, from, to); err != nil {
		return err
	}

	now := time.Now()
	elapsed := now.Sub(s.LastMove)

	if s.Turn == core.White {
		s.WhiteTime -= elapsed
		if s.WhiteTime <= 0 {
			s.WhiteTime = 0
			s.Status = "black_won"
			return errors.New("time is up")
		}
	} else {
		s.BlackTime -= elapsed
		if s.BlackTime <= 0 {
			s.BlackTime = 0
			s.Status = "white_won"
			return errors.New("time is up")
		}
	}

	s.Mode.ApplyMoveSideEffects(s.Board, from, to)

	if s.Turn == core.White {
		s.Turn = core.Black
	} else {
		s.Turn = core.White
	}

	s.LastMove = now
	s.Status = s.Mode.CheckState(s.Board, s.Turn)
	return nil
}

// RunTimer запускает фоновую проверку времени (например, 2 раза в секунду)
func (s *GameSession) RunTimer(onTimeout func(newStatus string)) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.Mu.Lock()
			if s.Status != "active" {
				s.Mu.Unlock()
				return // Партия завершена (мат, пат и т.д.), таймер больше не нужен
			}

			now := time.Now()
			elapsed := now.Sub(s.LastMove)
			timeIsUp := false

			if s.Turn == core.White {
				if s.WhiteTime-elapsed <= 0 {
					s.WhiteTime = 0
					s.Status = "black_won_timeout"
					timeIsUp = true
				}
			} else {
				if s.BlackTime-elapsed <= 0 {
					s.BlackTime = 0
					s.Status = "white_won_timeout"
					timeIsUp = true
				}
			}
			s.Mu.Unlock()

			if timeIsUp {
				onTimeout(s.Status)
				return
			}

		case <-s.StopTimer:
			return // Принудительная остановка сессии
		}
	}
}

func posToString(p core.Pos) string {
	return fmt.Sprintf("%c%d", 'a'+p.X, p.Y+1)
}

// GetAvailableMoves собирает словарь всех возможных ходов для текущего игрока.
func (s *GameSession) GetAvailableMoves() map[string][]string {
	moves := make(map[string][]string)

	for x1 := 0; x1 < s.Board.Width; x1++ {
		for y1 := 0; y1 < s.Board.Height; y1++ {
			from := core.Pos{X: x1, Y: y1}

			piece, ok := s.Board.Grid[from]

			if !ok || piece.Color != s.Turn {
				continue
			}

			var validDestinations []string

			// Проверяем все клетки доски в качестве цели
			for x2 := 0; x2 < s.Board.Width; x2++ {
				for y2 := 0; y2 < s.Board.Height; y2++ {
					to := core.Pos{X: x2, Y: y2}

					// ValidateMove делает всю грязную работу (перекрытия, шах королю)
					if err := s.Mode.ValidateMove(s.Board, s.Turn, from, to); err == nil {
						validDestinations = append(validDestinations, posToString(to))
					}
				}
			}

			if len(validDestinations) > 0 {
				moves[posToString(from)] = validDestinations
			}
		}
	}

	return moves
}

// ExportState создает безопасный снимок текущей игры для отправки клиентам.
func (s *GameSession) ExportState() GameStateDTO {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	now := time.Now()

	wTime := s.WhiteTime
	bTime := s.BlackTime

	// Считаем актуальное время только если партия еще идет
	if s.Status == "active" {
		elapsed := now.Sub(s.LastMove)
		if s.Turn == core.White {
			wTime -= elapsed
		} else {
			bTime -= elapsed
		}
	}

	// Защита от отрицательного времени на клиенте
	if wTime < 0 {
		wTime = 0
	}
	if bTime < 0 {
		bTime = 0
	}

	// Упаковываем всё в DTO. Тип Color - это string под капотом, поэтому делаем приведение типа.
	return GameStateDTO{
		Status:        s.Status,
		Turn:          string(s.Turn),
		WhiteTimeLeft: wTime.Milliseconds(),
		BlackTimeLeft: bTime.Milliseconds(),
		ValidMoves:    s.GetAvailableMoves(),
	}
}

func (s *GameSession) EndGame(finalStatus string) {
	s.Mu.Lock()
	if s.isEnded {
		s.Mu.Unlock()
		return
	}

	s.isEnded = true
	s.Status = finalStatus
	s.Mu.Unlock()

	if s.OnGameEnd != nil {
		s.OnGameEnd(finalStatus)
	}
}
