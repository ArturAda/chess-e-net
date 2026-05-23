package elo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestElo_Calculate_WhiteWins(t *testing.T) {
	newR1, newR2 := Calculate(1200, 1200, 1.0)

	assert.Greater(t, newR1, 1200, "Победитель должен получить очки")
	assert.Less(t, newR2, 1200, "Проигравший должен потерять очки")
}

func TestElo_Calculate_Draw(t *testing.T) {
	newR1, newR2 := Calculate(1600, 1200, 0.5)

	assert.Less(t, newR1, 1600, "Сильный игрок теряет очки при ничьей со слабым")
	assert.Greater(t, newR2, 1200, "Слабый игрок получает очки при ничьей с сильным")
}
