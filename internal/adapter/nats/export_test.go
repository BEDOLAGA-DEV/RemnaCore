package nats

import "time"

// ExtractTraceParent exports extractTraceParent for unit tests in the
// nats_test package.
func ExtractTraceParent(payload []byte) string {
	return extractTraceParent(payload)
}

// CurrentQuarterStart exports currentQuarterStart for unit tests in the
// nats_test package.
func CurrentQuarterStart(t time.Time) time.Time {
	return currentQuarterStart(t)
}
