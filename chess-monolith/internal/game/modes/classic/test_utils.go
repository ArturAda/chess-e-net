package classic

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
)

// Helper: превращает "e2e4" или "e2-e4" в координаты
func parseMove(m string) (core.Pos, core.Pos) {
	// Упрощенный парсер: берет первые 2 символа как FROM, вторые 2 как TO
	// В полноценном проекте тут будет логика перевода "e4" в координаты
	return core.Pos{X: int(m[0] - 'a'), Y: int(m[1] - '1')},
		core.Pos{X: int(m[2] - 'a'), Y: int(m[3] - '1')}
}

func playMoves(s *session.GameSession, moves []string) error {
	for _, m := range moves {
		from, to := parseMove(m)
		if err := s.MakeMove(from, to); err != nil {
			return err
		}
	}
	return nil
}
