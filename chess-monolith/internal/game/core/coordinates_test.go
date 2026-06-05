package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSquare(t *testing.T) {
	tests := []struct {
		name   string
		square string
		want   Pos
	}{
		{name: "classic first square", square: "a1", want: Pos{X: 0, Y: 0}},
		{name: "classic middle square", square: "e2", want: Pos{X: 4, Y: 1}},
		{name: "classic upper square", square: "h8", want: Pos{X: 7, Y: 7}},
		{name: "ten by ten rank", square: "j10", want: Pos{X: 9, Y: 9}},
		{name: "twelve by twelve rank", square: "l12", want: Pos{X: 11, Y: 11}},
		{name: "uppercase is accepted", square: "A10", want: Pos{X: 0, Y: 9}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSquare(tt.square)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseSquare_Invalid(t *testing.T) {
	tests := []string{"", "a", "11", "aa1", "a0", "a-1", "_1"}

	for _, square := range tests {
		t.Run(square, func(t *testing.T) {
			got, err := ParseSquare(square)
			require.Error(t, err)
			assert.Equal(t, Pos{X: -1, Y: -1}, got)
		})
	}
}

func TestFormatSquare(t *testing.T) {
	assert.Equal(t, "a1", FormatSquare(Pos{X: 0, Y: 0}))
	assert.Equal(t, "e2", FormatSquare(Pos{X: 4, Y: 1}))
	assert.Equal(t, "h8", FormatSquare(Pos{X: 7, Y: 7}))
	assert.Equal(t, "j10", FormatSquare(Pos{X: 9, Y: 9}))
	assert.Equal(t, "l12", FormatSquare(Pos{X: 11, Y: 11}))
	assert.Empty(t, FormatSquare(Pos{X: -1, Y: 0}))
	assert.Empty(t, FormatSquare(Pos{X: 0, Y: -1}))
}
