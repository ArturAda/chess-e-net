package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoard_IsPathClear(t *testing.T) {
	b := NewBoard(8, 8)

	// Ставим препятствие на пути
	b.Grid[Pos{X: 2, Y: 0}] = Piece{Type: "pawn", Color: White}

	// 1. Путь чист (горизонталь)
	assert.True(t, b.IsPathClear(Pos{X: 0, Y: 0}, Pos{X: 1, Y: 0}))

	// 2. Путь заблокирован (горизонталь)
	assert.False(t, b.IsPathClear(Pos{X: 0, Y: 0}, Pos{X: 3, Y: 0}))

	// 3. Путь чист (диагональ)
	assert.True(t, b.IsPathClear(Pos{X: 0, Y: 0}, Pos{X: 1, Y: 1}))

	// 4. Путь заблокирован (диагональ)
	b.Grid[Pos{X: 1, Y: 1}] = Piece{Type: "pawn", Color: Black}
	assert.False(t, b.IsPathClear(Pos{X: 0, Y: 0}, Pos{X: 2, Y: 2}))
}
