package core

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSquare converts algebraic board coordinates like "e2" or "a10" to Pos.
func ParseSquare(square string) (Pos, error) {
	normalized := strings.ToLower(strings.TrimSpace(square))
	if len(normalized) < 2 {
		return Pos{X: -1, Y: -1}, fmt.Errorf("invalid square %q", square)
	}

	file := normalized[0]
	if file < 'a' || file > 'z' {
		return Pos{X: -1, Y: -1}, fmt.Errorf("invalid square file %q", square)
	}

	rank, err := strconv.Atoi(normalized[1:])
	if err != nil || rank < 1 {
		return Pos{X: -1, Y: -1}, fmt.Errorf("invalid square rank %q", square)
	}

	return Pos{
		X: int(file - 'a'),
		Y: rank - 1,
	}, nil
}

// FormatSquare converts Pos back to algebraic coordinates.
func FormatSquare(pos Pos) string {
	if pos.X < 0 || pos.X > 25 || pos.Y < 0 {
		return ""
	}
	return fmt.Sprintf("%c%d", 'a'+pos.X, pos.Y+1)
}
