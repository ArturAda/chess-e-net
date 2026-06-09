package session

import (
	"chess-monolith/internal/game/core"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePromotionChoice(t *testing.T) {
	board := core.NewBoard(8, 8)
	pawn := core.Piece{Type: "pawn", Color: core.White}

	choice, err := normalizePromotionChoice(board, pawn, core.Pos{X: 0, Y: 7}, "")
	require.NoError(t, err)
	assert.Equal(t, "queen", choice)

	choice, err = normalizePromotionChoice(board, pawn, core.Pos{X: 0, Y: 7}, " ROOK ")
	require.NoError(t, err)
	assert.Equal(t, "rook", choice)

	_, err = normalizePromotionChoice(board, pawn, core.Pos{X: 0, Y: 7}, "dragon")
	assert.EqualError(t, err, "invalid promotion piece")

	choice, err = normalizePromotionChoice(board, pawn, core.Pos{X: 0, Y: 6}, "")
	require.NoError(t, err)
	assert.Equal(t, "", choice)

	_, err = normalizePromotionChoice(board, core.Piece{Type: "king", Color: core.White}, core.Pos{X: 0, Y: 7}, "queen")
	assert.EqualError(t, err, "promotion is not available for this move")
}

func TestGameSession_RunTimerStopsWithoutTimeout(t *testing.T) {
	game := &GameSession{
		Status:    "active",
		StopTimer: make(chan struct{}),
		WhiteTime: time.Minute,
		BlackTime: time.Minute,
		LastMove:  time.Now(),
		Turn:      core.White,
		Board:     core.NewBoard(8, 8),
		Mode:      &MockMode{},
	}
	done := make(chan struct{})

	go func() {
		game.RunTimer(func(string) {
			t.Fatal("timeout callback should not run after StopTimer")
		})
		close(done)
	}()

	close(game.StopTimer)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timer did not stop")
	}
}

func TestGameSession_EndGameIsIdempotentAndClearsDrawOffer(t *testing.T) {
	calls := 0
	game := &GameSession{
		Status:    "active",
		DrawOffer: &DrawOfferState{ID: "offer-1"},
		OnGameEnd: func(finalStatus string) {
			calls++
			assert.Equal(t, "draw", finalStatus)
		},
	}

	game.EndGame("draw")
	game.EndGame("white_won")

	assert.Equal(t, "draw", game.Status)
	assert.Nil(t, game.DrawOffer)
	assert.Equal(t, 1, calls)
}

func TestGameSession_ExportStateClampsBlackClock(t *testing.T) {
	game := &GameSession{
		ID:        "game-black-clock",
		Status:    "active",
		Turn:      core.Black,
		Board:     core.NewBoard(8, 8),
		Mode:      &MockMode{},
		WhiteTime: time.Minute,
		BlackTime: time.Millisecond,
		LastMove:  time.Now().Add(-time.Second),
	}

	dto := game.ExportState()

	assert.Equal(t, "game-black-clock", dto.GameID)
	assert.Equal(t, int64(time.Minute/time.Millisecond), dto.WhiteTimeLeft)
	assert.Equal(t, int64(0), dto.BlackTimeLeft)
	assert.Equal(t, "black", dto.Turn)
}
