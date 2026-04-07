package nats

// extractStringFromData extracts a string field from event data.
// Data is expected to be map[string]any (from JSON unmarshal of NATS messages).
// This is used only for backward-compatible entity ID resolution in
// handleMessage, where the event may lack EntityID and the typed payload
// extraction has not yet occurred.
func extractStringFromData(data any, key string) string {
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
