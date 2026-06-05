package session

import (
	"chess-monolith/internal/game/core"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockMode реализует упрощенные правила для теста
type MockMode struct{}

func (m *MockMode) Setup() *core.Board                                            { return core.NewBoard(8, 8) }
func (m *MockMode) ValidateMove(_ *core.Board, _ core.Color, _, _ core.Pos) error { return nil }
func (m *MockMode) ApplyMoveSideEffects(_ *core.Board, _, _ core.Pos)             {}
func (m *MockMode) CheckState(_ *core.Board, _ core.Color) string                 { return "active" }

func TestGameSession_TurnRotation(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("mock", &MockMode{})

	s, _ := NewSession(reg, "mock", 10*time.Minute)

	assert.Equal(t, core.White, s.Turn, "Game should start with White")

	// Первый ход
	_ = s.MakeMove(core.Pos{0, 0}, core.Pos{0, 1})
	assert.Equal(t, core.Black, s.Turn, "Turn should rotate to Black")

	// Второй ход
	_ = s.MakeMove(core.Pos{0, 1}, core.Pos{0, 2})
	assert.Equal(t, core.White, s.Turn, "Turn should rotate to White")
}

func TestGameSession_GameOver(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("mock", &MockMode{})

	s, _ := NewSession(reg, "mock", 10*time.Minute)
	s.Status = "white_won" // Принудительно завершаем игру

	err := s.MakeMove(core.Pos{0, 0}, core.Pos{0, 1})
	assert.Error(t, err, "Should not be able to move when game is over")
	assert.Equal(t, "game is over", err.Error())
}

func TestGameSession_ExportState(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("mock", &MockMode{})
	s, _ := NewSession(reg, "mock", 10*time.Minute)
	s.ID = "game-123"
	s.Board.Grid[core.Pos{X: 4, Y: 3}] = core.Piece{Type: "pawn", Color: core.White}
	captured := core.Piece{Type: "pawn", Color: core.Black}
	s.Board.History = append(s.Board.History, core.MoveRecord{
		From:     core.Pos{X: 4, Y: 1},
		To:       core.Pos{X: 4, Y: 3},
		Piece:    core.Piece{Type: "pawn", Color: core.White},
		Captured: &captured,
	})

	time.Sleep(100 * time.Millisecond)

	dto := s.ExportStateForPlayer(core.White)

	assert.Equal(t, "game-123", dto.GameID)
	assert.Equal(t, "white", dto.PlayerColor)
	assert.Equal(t, 8, dto.BoardSize)
	assert.Equal(t, 8, dto.Board.Width)
	assert.Equal(t, 8, dto.Board.Height)
	assert.Equal(t, "active", dto.Status)
	assert.Equal(t, "white", dto.Turn)
	assert.Len(t, dto.Board.Pieces, 1)
	assert.Equal(t, PieceDTO{Square: "e4", Type: "pawn", Color: "white"}, dto.Board.Pieces[0])
	assert.NotNil(t, dto.LastMove)
	assert.Equal(t, "e2", dto.LastMove.From)
	assert.Equal(t, "e4", dto.LastMove.To)
	assert.Equal(t, PieceDTO{Square: "e4", Type: "pawn", Color: "white"}, dto.LastMove.Piece)
	assert.NotNil(t, dto.LastMove.Captured)
	assert.Equal(t, PieceDTO{Type: "pawn", Color: "black"}, *dto.LastMove.Captured)
	assert.Empty(t, dto.CapturedWhite)
	assert.Equal(t, []PieceDTO{{Type: "pawn", Color: "black"}}, dto.CapturedBlack)

	assert.Less(t, dto.WhiteTimeLeft, int64(600000))  // Время белых должно уменьшиться
	assert.Equal(t, int64(600000), dto.BlackTimeLeft) // Время черных не изменилось
}

func TestGameSession_RunTimer_Timeout(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("mock", &MockMode{})

	// Создаем игру с очень коротким лимитом (200 миллисекунд)
	s, _ := NewSession(reg, "mock", 200*time.Millisecond)

	timeoutStatus := ""
	var wg sync.WaitGroup
	wg.Add(1)

	// Запускаем таймер
	go s.RunTimer(func(newStatus string) {
		timeoutStatus = newStatus
		wg.Done()
	})

	// Ждем, пока таймер сработает
	wg.Wait()

	assert.Equal(t, "black_won_timeout", timeoutStatus)
	assert.Equal(t, "black_won_timeout", s.Status)
	assert.Equal(t, time.Duration(0), s.WhiteTime)
}
