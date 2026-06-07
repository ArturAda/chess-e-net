package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Мок для тестирования интерфейса GameMode
type MockMode struct{}

func (m *MockMode) Setup() *Board                                          { return nil }
func (m *MockMode) ValidateMove(_ *Board, _ Color, _, _ Pos) error         { return nil }
func (m *MockMode) ApplyMoveSideEffects(_ *Board, _, _ Pos, _ MoveOptions) {}
func (m *MockMode) CheckState(_ *Board, _ Color) string                    { return "" }

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	mode := &MockMode{}

	// Регистрация
	reg.Register("test_mode", mode)

	// Получение
	m, err := reg.Get("test_mode")
	assert.NoError(t, err)
	assert.Equal(t, mode, m)

	// Ошибка при отсутствии
	_, err = reg.Get("missing")
	assert.Error(t, err)
}
