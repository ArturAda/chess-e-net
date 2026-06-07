package ws

import "encoding/json"

const EmptyVisualStateJSON = "{}"

func NormalizeVisualState(raw json.RawMessage) string {
	if len(raw) == 0 {
		return EmptyVisualStateJSON
	}
	return normalizeVisualStateBytes(raw)
}

func NormalizeVisualStateString(raw string) string {
	if raw == "" {
		return EmptyVisualStateJSON
	}
	return normalizeVisualStateBytes([]byte(raw))
}

func normalizeVisualStateBytes(raw []byte) string {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return EmptyVisualStateJSON
	}
	if value == nil {
		return EmptyVisualStateJSON
	}

	normalized, err := json.Marshal(value)
	if err != nil {
		return EmptyVisualStateJSON
	}
	return string(normalized)
}
