package nats

// ExtractTraceParent exports extractTraceParent for unit tests in the
// nats_test package.
func ExtractTraceParent(payload []byte) string {
	return extractTraceParent(payload)
}
