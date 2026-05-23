package core

type MoveRecord struct {
	From, To Pos
	Piece    Piece
}

// Board хранит состояние доски в виде карты
type Board struct {
	Width, Height int
	Grid          map[Pos]Piece
	History       []MoveRecord
}

func NewBoard(w, h int) *Board {
	return &Board{
		Width:   w,
		Height:  h,
		Grid:    make(map[Pos]Piece),
		History: make([]MoveRecord, 0),
	}
}

func (b *Board) IsPathClear(from, to Pos) bool {
	dx, dy := 0, 0
	if to.X > from.X {
		dx = 1
	} else if to.X < from.X {
		dx = -1
	}
	if to.Y > from.Y {
		dy = 1
	} else if to.Y < from.Y {
		dy = -1
	}

	curr := Pos{X: from.X + dx, Y: from.Y + dy}
	for curr != to {
		if _, occupied := b.Grid[curr]; occupied {
			return false
		}
		curr.X += dx
		curr.Y += dy
	}
	return true
}
