package session

import (
	"chess-monolith/internal/game/core"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockMode реализует упрощенные правила для теста
type MockMode struct{}

func (m *MockMode) Setup() *core.Board                                             { return core.NewBoard(8, 8) }
func (m *MockMode) ValidateMove(b *core.Board, t core.Color, f, to core.Pos) error { return nil }
func (m *MockMode) ApplyMoveSideEffects(b *core.Board, f, to core.Pos)             {}
func (m *MockMode) CheckState(b *core.Board, t core.Color) string                  { return "active" }

func TestGameSession_TurnRotation(t *testing.T) {
	reg := core.NewRegistry()
	reg.Register("mock", &MockMode{})

	s, _ := NewSession(reg, "mock")

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

	s, _ := NewSession(reg, "mock")
	s.Status = "white_won" // Принудительно завершаем игру

	err := s.MakeMove(core.Pos{0, 0}, core.Pos{0, 1})
	assert.Error(t, err, "Should not be able to move when game is over")
	assert.Equal(t, "game is over", err.Error())
}
