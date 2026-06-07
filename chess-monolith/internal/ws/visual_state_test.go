package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeVisualState(t *testing.T) {
	assert.JSONEq(t, `{"pieces":{"white":"pixel"}}`, NormalizeVisualState(json.RawMessage(`{"pieces":{"white":"pixel"}}`)))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualState(nil))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualState(json.RawMessage(`null`)))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualState(json.RawMessage(`[]`)))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualState(json.RawMessage(`"bad"`)))
	assert.Equal(t, EmptyVisualStateJSON, NormalizeVisualState(json.RawMessage(`{bad`)))
}
