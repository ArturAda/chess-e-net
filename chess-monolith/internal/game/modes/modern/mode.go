package modern

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/modes/classic"
)

func Register(r *core.Registry) {
	r.Register("modern10", classic.NewMode(10))
	r.Register("modern12", classic.NewMode(12))
}
