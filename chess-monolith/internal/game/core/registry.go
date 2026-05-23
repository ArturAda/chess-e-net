package core

import (
	"fmt"
	"sync"
)

// GameMode диктует правила
type GameMode interface {
	Setup() *Board
	ValidateMove(b *Board, turn Color, from, to Pos) error
	ApplyMoveSideEffects(b *Board, from, to Pos)
	CheckState(b *Board, turn Color) string
}

type Registry struct {
	modes map[string]GameMode
	mu    sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{modes: make(map[string]GameMode)}
}

func (r *Registry) Register(name string, mode GameMode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modes[name] = mode
}

func (r *Registry) Get(name string) (GameMode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	mode, ok := r.modes[name]
	if !ok {
		return nil, fmt.Errorf("mode %s not found", name)
	}
	return mode, nil
}
