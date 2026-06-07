package modern

import (
	"chess-monolith/internal/game/core"
	"chess-monolith/internal/game/session"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterModernModes(t *testing.T) {
	reg := core.NewRegistry()
	Register(reg)

	tests := []struct {
		modeName string
		size     int
	}{
		{modeName: "modern10", size: 10},
		{modeName: "modern12", size: 12},
	}

	for _, tt := range tests {
		t.Run(tt.modeName, func(t *testing.T) {
			s, err := session.NewSession(reg, tt.modeName, 10*time.Minute)
			require.NoError(t, err)

			assert.Equal(t, tt.size, s.Board.Width)
			assert.Equal(t, tt.size, s.Board.Height)
			assert.Equal(t, "active", s.Status)
			assert.Equal(t, core.White, s.Turn)
		})
	}
}
