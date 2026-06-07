package classic

import (
	"chess-monolith/internal/game/core"
	"errors"
)

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

type Mode struct {
	boardSize int
}

func NewMode(boardSize int) *Mode {
	if boardSize != 8 && boardSize != 10 && boardSize != 12 {
		boardSize = 8
	}
	return &Mode{boardSize: boardSize}
}

func Register(r *core.Registry) {
	r.Register("classic", NewMode(8))
}

func (m *Mode) Setup() *core.Board {
	size := m.size()
	b := core.NewBoard(size, size)

	newPiece := func(figureType string, color core.Color) core.Piece {
		return core.Piece{Type: figureType, Color: color, Meta: make(map[string]any)}
	}

	for i := 0; i < size; i++ {
		b.Grid[core.Pos{X: i, Y: 1}] = newPiece("pawn", core.White)
		b.Grid[core.Pos{X: i, Y: size - 2}] = newPiece("pawn", core.Black)
	}

	order := backRankForSize(size)
	for i, ft := range order {
		b.Grid[core.Pos{X: i, Y: 0}] = newPiece(ft, core.White)
		b.Grid[core.Pos{X: i, Y: size - 1}] = newPiece(ft, core.Black)
	}

	return b
}

func (m *Mode) validateGeometry(board *core.Board, piece core.Piece, from, to core.Pos, targetOk bool) error {
	dx, dy := to.X-from.X, to.Y-from.Y
	adx, ady := abs(dx), abs(dy)

	switch piece.Type {
	case "pawn":
		dir, startY := 1, 1
		if piece.Color == core.Black {
			dir, startY = -1, board.Height-2
		}

		if dx == 0 && dy == dir && !targetOk {
			return nil
		} // Обычный шаг

		if dx == 0 && dy == 2*dir && from.Y == startY && !targetOk { // Прыжок со старта
			if _, block := board.Grid[core.Pos{X: from.X, Y: from.Y + dir}]; !block {
				return nil
			}
		}

		if adx == 1 && dy == dir { // Взятие по диагонали
			if targetOk {
				return nil
			}
			// Взятие на проходе
			if len(board.History) > 0 {
				last := board.History[len(board.History)-1]
				if last.Piece.Type == "pawn" && last.To.X == to.X && last.To.Y == from.Y && abs(last.From.Y-last.To.Y) == 2 {
					return nil
				}
			}
		}
	case "knight":
		if (adx == 1 && ady == 2) || (adx == 2 && ady == 1) {
			return nil
		}
	case "rook":
		if (dx == 0 || dy == 0) && board.IsPathClear(from, to) {
			return nil
		}
	case "bishop":
		if adx == ady && board.IsPathClear(from, to) {
			return nil
		}
	case "queen":
		if (dx == 0 || dy == 0 || adx == ady) && board.IsPathClear(from, to) {
			return nil
		}
	case "king":
		if adx <= 1 && ady <= 1 {
			return nil
		}

		// Рокировка (если король и ладья не ходили)
		moved, _ := piece.Meta["moved"].(bool)
		if !moved && dy == 0 && adx == 2 {
			rookX := board.Width - 1
			if to.X < from.X {
				rookX = 0
			}

			rook, ok := board.Grid[core.Pos{X: rookX, Y: from.Y}]
			rMoved, _ := rook.Meta["moved"].(bool)

			if ok && rook.Type == "rook" && !rMoved && board.IsPathClear(from, core.Pos{X: rookX, Y: from.Y}) {
				if !m.isKingInCheck(board, piece.Color) {
					step := 1

					if to.X < from.X {
						step = -1
					}

					if m.isMoveSafe(board, piece.Color, from, core.Pos{X: from.X + step, Y: from.Y}) {
						return nil
					}
				}
			}
		}
	}
	return errors.New("invalid piece geometry")
}

func (m *Mode) findKing(board *core.Board, color core.Color) core.Pos {
	for pos, piece := range board.Grid {
		if piece.Type == "king" && piece.Color == color {
			return pos
		}
	}
	return core.Pos{X: -1, Y: -1}
}

func (m *Mode) checkRawGeometry(b *core.Board, pc core.Piece, from, to core.Pos) bool {
	dx, dy := to.X-from.X, to.Y-from.Y
	adx, ady := abs(dx), abs(dy)

	switch pc.Type {
	case "pawn":
		dir := 1
		if pc.Color == core.Black {
			dir = -1
		}
		return adx == 1 && dy == dir
	case "knight":
		return (adx == 1 && ady == 2) || (adx == 2 && ady == 1)
	case "rook":
		return (dx == 0 || dy == 0) && b.IsPathClear(from, to)
	case "bishop":
		return adx == ady && b.IsPathClear(from, to)
	case "queen":
		return (dx == 0 || dy == 0 || adx == ady) && b.IsPathClear(from, to)
	case "king":
		return adx <= 1 && ady <= 1 // Никаких рокировок здесь!
	}
	return false
}

func (m *Mode) isKingInCheck(board *core.Board, color core.Color) bool {
	kingPos := m.findKing(board, color)
	if kingPos.X == -1 {
		return false
	} // Страховка

	oppositeColor := core.White
	if color == core.White {
		oppositeColor = core.Black
	}

	for pos, piece := range board.Grid {
		if piece.Color == oppositeColor {
			// Если фигура противника может атаковать клетку короля - это шах
			if m.checkRawGeometry(board, piece, pos, kingPos) {
				return true
			}
		}
	}
	return false
}

func (m *Mode) isMoveSafe(board *core.Board, turn core.Color, from, to core.Pos) bool {
	piece := board.Grid[from]
	target, targetOk := board.Grid[to]

	board.Grid[to] = piece
	delete(board.Grid, from)

	inCheck := m.isKingInCheck(board, turn)

	// Откат
	board.Grid[from] = piece
	if targetOk {
		board.Grid[to] = target
	} else {
		delete(board.Grid, to)
	}

	return !inCheck
}

func (m *Mode) ValidateMove(board *core.Board, turn core.Color, from, to core.Pos) error {
	if from.X < 0 || from.X >= board.Width || from.Y < 0 || from.Y >= board.Height {
		return errors.New("out of bounds")
	}

	if to.X < 0 || to.X >= board.Width || to.Y < 0 || to.Y >= board.Height {
		return errors.New("out of bounds")
	}

	piece, ok := board.Grid[from]
	if !ok || piece.Color != turn {
		return errors.New("not your piece")
	}

	target, targetOk := board.Grid[to]
	if targetOk && target.Color == turn {
		return errors.New("friendly fire")
	}

	if err := m.validateGeometry(board, piece, from, to, targetOk); err != nil {
		return err
	}

	// Проверяем, не открыли ли мы своего короля
	if !m.isMoveSafe(board, turn, from, to) {
		return errors.New("king is in check")
	}

	return nil
}

func (m *Mode) ApplyMoveSideEffects(board *core.Board, from, to core.Pos, options core.MoveOptions) {
	piece := board.Grid[from]
	targetOk := false
	var captured *core.Piece
	promotion := ""
	if target, ok := board.Grid[to]; ok {
		targetOk = true
		capturedPiece := target
		captured = &capturedPiece
	}

	// 1. Рокировка (сдвиг ладьи)
	if piece.Type == "king" && abs(from.X-to.X) == 2 {
		rookFrom := core.Pos{X: board.Width - 1, Y: from.Y}
		rookTo := core.Pos{X: to.X - 1, Y: from.Y}
		if to.X < from.X {
			rookFrom = core.Pos{X: 0, Y: from.Y}
			rookTo = core.Pos{X: to.X + 1, Y: from.Y}
		}

		rook := board.Grid[rookFrom]
		delete(board.Grid, rookFrom)
		board.Grid[rookTo] = rook
	}

	// 2. Взятие на проходе (удаление пешки противника)
	if piece.Type == "pawn" && from.X != to.X && !targetOk {
		enPassantPos := core.Pos{X: to.X, Y: from.Y}
		if enPassantTarget, ok := board.Grid[enPassantPos]; ok {
			capturedPiece := enPassantTarget
			captured = &capturedPiece
		}
		delete(board.Grid, enPassantPos)
	}

	// Основной сдвиг
	board.Grid[to] = piece
	delete(board.Grid, from)

	// 3. Превращение пешки в выбранную фигуру
	if piece.Type == "pawn" && (to.Y == 0 || to.Y == board.Height-1) {
		promotion = options.Promotion
		if promotion == "" {
			promotion = "queen"
		}
		piece.Type = promotion
	}

	// 4. Запись метаданных и истории
	if piece.Meta == nil {
		piece.Meta = make(map[string]any)
	}

	piece.Meta["moved"] = true
	board.Grid[to] = piece // Обновляем структуру на доске

	board.History = append(board.History, core.MoveRecord{From: from, To: to, Piece: piece, Captured: captured, Promotion: promotion})
}

func (m *Mode) CheckState(board *core.Board, turn core.Color) string {
	hasMoves := false
	for from, piece := range board.Grid {
		if piece.Color != turn {
			continue
		}

		for x := 0; x < board.Width; x++ {
			for y := 0; y < board.Height; y++ {
				if err := m.ValidateMove(board, turn, from, core.Pos{X: x, Y: y}); err == nil {
					hasMoves = true
					break
				}
			}
			if hasMoves {
				break
			}
		}

		if hasMoves {
			break
		}
	}

	if !hasMoves {
		if m.isKingInCheck(board, turn) {
			if turn == core.White {
				return "black_won"
			}
			return "white_won"
		}
		return "draw" // Нет ходов, но и нет шаха = Пат
	}
	return "active"
}

func (m *Mode) size() int {
	if m == nil || m.boardSize <= 0 {
		return 8
	}
	return m.boardSize
}

func backRankForSize(size int) []string {
	switch size {
	case 10:
		return []string{"rook", "knight", "knight", "bishop", "queen", "king", "bishop", "knight", "knight", "rook"}
	case 12:
		return []string{"rook", "knight", "bishop", "bishop", "knight", "queen", "king", "knight", "bishop", "bishop", "knight", "rook"}
	default:
		return []string{"rook", "knight", "bishop", "queen", "king", "bishop", "knight", "rook"}
	}
}
