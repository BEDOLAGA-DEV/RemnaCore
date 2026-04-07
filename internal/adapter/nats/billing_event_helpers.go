package nats

import "encoding/json"

// extractString extracts a string field from event data.
// Data is expected to be map[string]any (from JSON unmarshal of NATS messages).
func extractString(data any, key string) string {
	m, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// extractInt64 extracts a numeric field as int64 from event data. JSON numbers
// unmarshal as float64 in map[string]any; this helper handles the conversion.
// Returns 0 if the key is missing, not a number, or the data is not a map.
func extractInt64(data any, key string) int64 {
	m, ok := data.(map[string]any)
	if !ok {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0
		}
		return i
	default:
		return 0
	}
}

// extractInt extracts a numeric field as int from event data. See extractInt64
// for details on JSON number handling.
func extractInt(data any, key string) int {
	return int(extractInt64(data, key))
}
