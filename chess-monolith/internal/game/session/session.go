package session

import (
	"chess-monolith/internal/game/core"
	"errors"
	"sync"
)

type GameSession struct {
	Mode   core.GameMode
	Board  *core.Board
	Turn   core.Color
	Status string
	Mu     sync.Mutex
}

// Создаем партию, просто передав строку "classic"
func NewSession(registry *core.Registry, modeName string) (*GameSession, error) {
	mode, err := registry.Get(modeName)
	if err != nil {
		return nil, err
	}

	return &GameSession{
		Mode:   mode,
		Board:  mode.Setup(),
		Turn:   core.White,
		Status: "active",
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

	s.Mode.ApplyMoveSideEffects(s.Board, from, to)

	if s.Turn == core.White {
		s.Turn = core.Black
	} else {
		s.Turn = core.White
	}

	s.Status = s.Mode.CheckState(s.Board, s.Turn)
	return nil
}
