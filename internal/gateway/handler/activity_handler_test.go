package handler

import "testing"

func TestLevelAndMessage(t *testing.T) {
	tests := []struct {
		eventType   string
		wantLevel   string
		wantMessage string
	}{
		{"invoice.paid", "ok", "Invoice paid"},
		{"subscription.trial_started", "ok", "Subscription trial started"},
		{"subscription.cancelled", "warn", "Subscription cancelled"},
		{"binding.traffic_exceeded", "warn", "Binding traffic exceeded"},
		{"plugin.some_thing.happened", "info", "Plugin some thing happened"},
		{"", "info", ""},
	}
	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			level, message := levelAndMessage(tt.eventType)
			if level != tt.wantLevel {
				t.Errorf("level = %q, want %q", level, tt.wantLevel)
			}
			if message != tt.wantMessage {
				t.Errorf("message = %q, want %q", message, tt.wantMessage)
			}
		})
	}
}

func TestHumanizeEventType_EmptyDoesNotPanic(t *testing.T) {
	if got := humanizeEventType(""); got != "" {
		t.Errorf("humanizeEventType(\"\") = %q, want empty", got)
	}
}
