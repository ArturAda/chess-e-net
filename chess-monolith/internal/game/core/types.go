package core

type Color string

const (
	White Color = "white"
	Black Color = "black"
)

type Pos struct {
	X, Y int
}

// Piece теперь обычная структура. JSON прочитает её без ошибок.
type Piece struct {
	Type  string
	Color Color
	Meta  map[string]any // Для флагов вроде "moved" или "mana"
}
