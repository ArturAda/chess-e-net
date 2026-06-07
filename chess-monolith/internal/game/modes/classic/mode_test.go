package classic

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setupTestSession() *session.GameSession {
	reg := core.NewRegistry()
	Register(reg)
	s, _ := session.NewSession(reg, "classic", 10*time.Minute)
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
	lastRecord := s.Board.History[len(s.Board.History)-1]
	assert.NotNil(t, lastRecord.Captured, "En passant should store captured pawn in move history")
	assert.Equal(t, "pawn", lastRecord.Captured.Type)
	assert.Equal(t, core.Black, lastRecord.Captured.Color)
}

func TestPawnPromotionUsesRequestedPiece(t *testing.T) {
	s := setupTestSession()
	s.Board.Grid = map[core.Pos]core.Piece{
		{X: 0, Y: 6}: {Type: "pawn", Color: core.White, Meta: map[string]any{}},
		{X: 4, Y: 0}: {Type: "king", Color: core.White, Meta: map[string]any{}},
		{X: 4, Y: 7}: {Type: "king", Color: core.Black, Meta: map[string]any{}},
	}
	s.Turn = core.White

	err := s.MakeMoveWithPromotion(core.Pos{X: 0, Y: 6}, core.Pos{X: 0, Y: 7}, "rook")

	assert.NoError(t, err)
	piece := s.Board.Grid[core.Pos{X: 0, Y: 7}]
	assert.Equal(t, "rook", piece.Type)
	assert.Equal(t, "rook", s.Board.History[len(s.Board.History)-1].Promotion)
}

func TestSetupSupportsModernBoardSizes(t *testing.T) {
	tests := []struct {
		name         string
		size         int
		expectedRank []string
	}{
		{
			name:         "classic 8x8",
			size:         8,
			expectedRank: []string{"rook", "knight", "bishop", "queen", "king", "bishop", "knight", "rook"},
		},
		{
			name:         "modern 10x10",
			size:         10,
			expectedRank: []string{"rook", "knight", "knight", "bishop", "queen", "king", "bishop", "knight", "knight", "rook"},
		},
		{
			name:         "modern 12x12",
			size:         12,
			expectedRank: []string{"rook", "knight", "bishop", "bishop", "knight", "queen", "king", "knight", "bishop", "bishop", "knight", "rook"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			board := NewMode(tt.size).Setup()

			assert.Equal(t, tt.size, board.Width)
			assert.Equal(t, tt.size, board.Height)

			for x, expectedType := range tt.expectedRank {
				whiteBackRank := board.Grid[core.Pos{X: x, Y: 0}]
				blackBackRank := board.Grid[core.Pos{X: x, Y: tt.size - 1}]
				whitePawn := board.Grid[core.Pos{X: x, Y: 1}]
				blackPawn := board.Grid[core.Pos{X: x, Y: tt.size - 2}]

				assert.Equal(t, expectedType, whiteBackRank.Type)
				assert.Equal(t, core.White, whiteBackRank.Color)
				assert.Equal(t, expectedType, blackBackRank.Type)
				assert.Equal(t, core.Black, blackBackRank.Color)
				assert.Equal(t, "pawn", whitePawn.Type)
				assert.Equal(t, core.White, whitePawn.Color)
				assert.Equal(t, "pawn", blackPawn.Type)
				assert.Equal(t, core.Black, blackPawn.Color)
			}
		})
	}
}

func TestModernPawnRulesUseBoardHeight(t *testing.T) {
	mode := NewMode(10)
	board := mode.Setup()

	err := mode.ValidateMove(board, core.Black, core.Pos{X: 0, Y: 8}, core.Pos{X: 0, Y: 6})
	assert.NoError(t, err)

	board = core.NewBoard(10, 10)
	board.Grid[core.Pos{X: 0, Y: 8}] = core.Piece{Type: "pawn", Color: core.White, Meta: make(map[string]any)}

	mode.ApplyMoveSideEffects(board, core.Pos{X: 0, Y: 8}, core.Pos{X: 0, Y: 9}, core.MoveOptions{})

	promoted := board.Grid[core.Pos{X: 0, Y: 9}]
	assert.Equal(t, "queen", promoted.Type)
	assert.Equal(t, core.White, promoted.Color)
}

func TestModernCastlingUsesBoardWidth(t *testing.T) {
	mode := NewMode(10)
	board := core.NewBoard(10, 10)
	board.Grid[core.Pos{X: 5, Y: 0}] = core.Piece{Type: "king", Color: core.White, Meta: make(map[string]any)}
	board.Grid[core.Pos{X: 9, Y: 0}] = core.Piece{Type: "rook", Color: core.White, Meta: make(map[string]any)}

	err := mode.ValidateMove(board, core.White, core.Pos{X: 5, Y: 0}, core.Pos{X: 7, Y: 0})
	assert.NoError(t, err)

	mode.ApplyMoveSideEffects(board, core.Pos{X: 5, Y: 0}, core.Pos{X: 7, Y: 0}, core.MoveOptions{})

	king := board.Grid[core.Pos{X: 7, Y: 0}]
	rook := board.Grid[core.Pos{X: 6, Y: 0}]
	assert.Equal(t, "king", king.Type)
	assert.Equal(t, "rook", rook.Type)
	_, oldKingExists := board.Grid[core.Pos{X: 5, Y: 0}]
	_, oldRookExists := board.Grid[core.Pos{X: 9, Y: 0}]
	assert.False(t, oldKingExists)
	assert.False(t, oldRookExists)
}

func TestLongGamePlaythrough_WithStateValidation(t *testing.T) {
	s := setupTestSession()

	moves := []struct {
		from core.Pos
		to   core.Pos
		desc string
	}{
		// 1. e4 e5
		{core.Pos{X: 4, Y: 1}, core.Pos{X: 4, Y: 3}, "w: e4"},
		{core.Pos{X: 4, Y: 6}, core.Pos{X: 4, Y: 4}, "b: e5"},
		// 2. Bc4 Nf6
		{core.Pos{X: 5, Y: 0}, core.Pos{X: 2, Y: 3}, "w: Bc4"},
		{core.Pos{X: 6, Y: 7}, core.Pos{X: 5, Y: 5}, "b: Nf6"},
		// 3. d3 Bb4+
		{core.Pos{X: 3, Y: 1}, core.Pos{X: 3, Y: 2}, "w: d3"},
		{core.Pos{X: 5, Y: 7}, core.Pos{X: 1, Y: 3}, "b: Bb4+"},
		// 4. c3 Ba5
		{core.Pos{X: 2, Y: 1}, core.Pos{X: 2, Y: 2}, "w: c3"},
		{core.Pos{X: 1, Y: 3}, core.Pos{X: 0, Y: 4}, "b: Ba5"},
		// 5. f4 d6
		{core.Pos{X: 5, Y: 1}, core.Pos{X: 5, Y: 3}, "w: f4"},
		{core.Pos{X: 3, Y: 6}, core.Pos{X: 3, Y: 5}, "b: d6"},
		// 6. Nf3 Bg4
		{core.Pos{X: 6, Y: 0}, core.Pos{X: 5, Y: 2}, "w: Nf3"},
		{core.Pos{X: 2, Y: 7}, core.Pos{X: 6, Y: 3}, "b: Bg4"},
		// 7. fxe5 Bxf3
		{core.Pos{X: 5, Y: 3}, core.Pos{X: 4, Y: 4}, "w: fxe5"},
		{core.Pos{X: 6, Y: 3}, core.Pos{X: 5, Y: 2}, "b: Bxf3"},
		// 8. Qxf3 dxe5
		{core.Pos{X: 3, Y: 0}, core.Pos{X: 5, Y: 2}, "w: Qxf3"},
		{core.Pos{X: 3, Y: 5}, core.Pos{X: 4, Y: 4}, "b: dxe5"},
		// 9. Bg5 Nc6
		{core.Pos{X: 2, Y: 0}, core.Pos{X: 6, Y: 4}, "w: Bg5"},
		{core.Pos{X: 1, Y: 7}, core.Pos{X: 2, Y: 5}, "b: Nc6"},
		// 10. Nd2 Bb6
		{core.Pos{X: 1, Y: 0}, core.Pos{X: 3, Y: 1}, "w: Nd2"},
		{core.Pos{X: 0, Y: 4}, core.Pos{X: 1, Y: 5}, "b: Bb6"},
		// 11. Rf1 O-O (Короткая рокировка черных)
		{core.Pos{X: 7, Y: 0}, core.Pos{X: 5, Y: 0}, "w: Rf1"},
		{core.Pos{X: 4, Y: 7}, core.Pos{X: 6, Y: 7}, "b: O-O"},
		// 12. O-O-O Nd7 (Длинная рокировка белых)
		{core.Pos{X: 4, Y: 0}, core.Pos{X: 2, Y: 0}, "w: O-O-O"},
		{core.Pos{X: 5, Y: 5}, core.Pos{X: 3, Y: 6}, "b: Nd7"},
		// 13. Bxd8 Raxd8
		{core.Pos{X: 6, Y: 4}, core.Pos{X: 3, Y: 7}, "w: Bxd8"},
		{core.Pos{X: 0, Y: 7}, core.Pos{X: 3, Y: 7}, "b: Raxd8"},
		// 14. Qh3 Nf6
		{core.Pos{X: 5, Y: 2}, core.Pos{X: 7, Y: 2}, "w: Qh3"},
		{core.Pos{X: 3, Y: 6}, core.Pos{X: 5, Y: 5}, "b: Nf6"},
		// 15. Rxf6 gxf6
		{core.Pos{X: 5, Y: 0}, core.Pos{X: 5, Y: 5}, "w: Rxf6"},
		{core.Pos{X: 6, Y: 6}, core.Pos{X: 5, Y: 5}, "b: gxf6"},
		// 16. Rf1 Bc5
		{core.Pos{X: 3, Y: 0}, core.Pos{X: 5, Y: 0}, "w: Rf1"},
		{core.Pos{X: 1, Y: 5}, core.Pos{X: 2, Y: 4}, "b: Bc5"},
		// 17. Nf3 Be7
		{core.Pos{X: 3, Y: 1}, core.Pos{X: 5, Y: 2}, "w: Nf3"},
		{core.Pos{X: 2, Y: 4}, core.Pos{X: 4, Y: 6}, "b: Be7"},
		// 18. Nh4 a6
		{core.Pos{X: 5, Y: 2}, core.Pos{X: 7, Y: 3}, "w: Nh4"},
		{core.Pos{X: 0, Y: 6}, core.Pos{X: 0, Y: 5}, "b: a6"},
		// 19. Nf5 Bc5
		{core.Pos{X: 7, Y: 3}, core.Pos{X: 5, Y: 4}, "w: Nf5"},
		{core.Pos{X: 4, Y: 6}, core.Pos{X: 2, Y: 4}, "b: Bc5"},
		// 20. Qg4+
		{core.Pos{X: 7, Y: 2}, core.Pos{X: 6, Y: 3}, "w: Qg4+"},
	}

	for i, m := range moves {
		// Ожидаемая длина истории до хода
		historyLenBefore := len(s.Board.History)

		err := s.MakeMove(m.from, m.to)
		assert.NoError(t, err, "Move %s should be valid", m.desc)

		// 1. Проверяем, что фигура покинула старую клетку
		_, oldExists := s.Board.Grid[m.from]
		assert.False(t, oldExists, "Square %v should be empty after move %s", m.from, m.desc)

		// 2. Проверяем, что фигура прибыла на новую клетку
		newPiece, newExists := s.Board.Grid[m.to]
		assert.True(t, newExists, "Square %v should have a piece after move %s", m.to, m.desc)

		// Проверяем, что в мету записался флаг "moved"
		moved, _ := newPiece.Meta["moved"].(bool)
		assert.True(t, moved, "Piece should have 'moved' meta flag set to true after move %s", m.desc)

		// 3. Проверяем историю
		assert.Equal(t, historyLenBefore+1, len(s.Board.History), "History should increment by 1 after move %s", m.desc)
		lastRecord := s.Board.History[len(s.Board.History)-1]
		assert.Equal(t, m.from, lastRecord.From, "History From pos mismatch on move %s", m.desc)
		assert.Equal(t, m.to, lastRecord.To, "History To pos mismatch on move %s", m.desc)

		// 4. Специфичные проверки побочных эффектов (рокировки)
		if m.desc == "b: O-O" {
			// Ладья должна переместиться с h8 (7,7) на f8 (5,7)
			rook, ok := s.Board.Grid[core.Pos{X: 5, Y: 7}]
			assert.True(t, ok, "Black rook should be at f8 after short castle")
			assert.Equal(t, "rook", rook.Type)
			_, oldRookExists := s.Board.Grid[core.Pos{X: 7, Y: 7}]
			assert.False(t, oldRookExists, "Black rook should not be at h8 after short castle")
		}

		if m.desc == "w: O-O-O" {
			// Ладья должна переместиться с a1 (0,0) на d1 (3,0)
			rook, ok := s.Board.Grid[core.Pos{X: 3, Y: 0}]
			assert.True(t, ok, "White rook should be at d1 after long castle")
			assert.Equal(t, "rook", rook.Type)
			_, oldRookExists := s.Board.Grid[core.Pos{X: 0, Y: 0}]
			assert.False(t, oldRookExists, "White rook should not be at a1 after long castle")
		}

		// 5. Убеждаемся, что статус остается "active" на протяжении всех этих 20 ходов
		// (Только если твоя реализация не выставляет шах как отдельный статус)
		if i < len(moves)-1 {
			assert.Equal(t, "active", s.Status, "Status should remain active after %s", m.desc)
		}
	}

	// Финальная проверка шаха
	engine := &Mode{}
	isCheck := engine.isKingInCheck(s.Board, core.Black)
	assert.True(t, isCheck, "Black king should be in check after 20. Qg4+")
}
