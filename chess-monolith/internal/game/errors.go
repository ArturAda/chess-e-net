package game

import "errors"

var (
	ErrGameNotFound = errors.New("game not found")
	ErrDatabase     = errors.New("database operation failed")
)
