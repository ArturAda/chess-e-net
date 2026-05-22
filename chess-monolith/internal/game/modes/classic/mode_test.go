package classic

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupTestSession() *session.GameSession {
	reg := core.NewRegistry()
	Register(reg)
	s, _ := session.NewSession(reg, "classic")
	return s
}

func TestFoolsMate(t *testing.T) {
	s := setupTestSession()

	_ = s.MakeMove(core.Pos{X: 5, Y: 1}, core.Pos{X: 5, Y: 2}) // w: f3
	_ = s.MakeMove(core.Pos{X: 4, Y: 6}, core.Pos{X: 4, Y: 4}) // b: e5
	_ = s.MakeMove(core.Pos{X: 6, Y: 1}, core.Pos{X: 6, Y: 3}) // w: g4
	_ = s.MakeMove(core.Pos{X: 3, Y: 7}, core.Pos{X: 7, Y: 3}) // b: Qh4#

	assert.Equal(t, "black_won", s.Status)
}

func TestEnPassant(t *testing.T) {
	s := setupTestSession()
	_ = s.MakeMove(core.Pos{X: 4, Y: 1}, core.Pos{X: 4, Y: 3}) // w: e4
	_ = s.MakeMove(core.Pos{X: 0, Y: 6}, core.Pos{X: 0, Y: 5}) // b: a6
	_ = s.MakeMove(core.Pos{X: 4, Y: 3}, core.Pos{X: 4, Y: 4}) // w: e5
	_ = s.MakeMove(core.Pos{X: 3, Y: 6}, core.Pos{X: 3, Y: 4}) // b: d5

	// Взятие на проходе e5xd6
	err := s.MakeMove(core.Pos{X: 4, Y: 4}, core.Pos{X: 3, Y: 5})
	assert.NoError(t, err)

	_, targetOccupied := s.Board.Grid[core.Pos{X: 3, Y: 4}]
	assert.False(t, targetOccupied, "Black pawn should be removed")
}
