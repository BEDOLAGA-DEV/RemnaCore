package remnawave

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildUsername(t *testing.T) {
	t.Run("truncates long user ID to 8 chars", func(t *testing.T) {
		got := BuildUsername("abcdefgh-1234-5678-9012-ijklmnopqrst", "main", 0)
		assert.Equal(t, "p_abcdefgh_main_0", got)
	})

	t.Run("preserves short user ID", func(t *testing.T) {
		got := BuildUsername("abc", "main", 0)
		assert.Equal(t, "p_abc_main_0", got)
	})

	t.Run("exactly 8 char ID", func(t *testing.T) {
		got := BuildUsername("12345678", "test", 5)
		assert.Equal(t, "p_12345678_test_5", got)
	})

	t.Run("multiple indexes", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			got := BuildUsername("user1234abcd", "vpn", i)
			assert.Contains(t, got, "p_user1234_vpn_")
		}
		assert.Equal(t, "p_user1234_vpn_0", BuildUsername("user1234abcd", "vpn", 0))
		assert.Equal(t, "p_user1234_vpn_1", BuildUsername("user1234abcd", "vpn", 1))
		assert.Equal(t, "p_user1234_vpn_2", BuildUsername("user1234abcd", "vpn", 2))
	})
}

func TestMapWebhookEvent_Known(t *testing.T) {
	tests := []struct {
		scope    string
		event    string
		expected string
	}{
		{"user", "created", "remnawave.user.synced"},
		{"user", "limited", "binding.traffic_exceeded"},
		{"user", "expired", "subscription.remnawave_expired"},
		{"user", "disabled", "subscription.binding_disabled"},
		{"user", "enabled", "subscription.binding_enabled"},
		{"user", "traffic_reset", "subscription.traffic_cycle_reset"},
		{"user", "first_connected", "subscription.first_use"},
		{"user", "bandwidth_usage_threshold_reached", "subscription.traffic_warning"},
		{"user", "expires_in_72_hours", "subscription.expiring_soon"},
		{"user", "expires_in_48_hours", "subscription.expiring_soon"},
		{"user", "expires_in_24_hours", "subscription.expiring_soon"},
		{"node", "connection_lost", "infra.node_down"},
		{"node", "connection_restored", "infra.node_up"},
		{"service", "panel_started", "infra.remnawave_restarted"},
	}

	for _, tt := range tests {
		t.Run(tt.scope+"."+tt.event, func(t *testing.T) {
			got := MapWebhookEvent(tt.scope, tt.event)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMapWebhookEvent_Unknown(t *testing.T) {
	t.Run("falls back to remnawave prefix", func(t *testing.T) {
		got := MapWebhookEvent("billing", "invoice_paid")
		assert.Equal(t, "remnawave.billing.invoice_paid", got)
	})

	t.Run("unknown user event", func(t *testing.T) {
		got := MapWebhookEvent("user", "unknown_event")
		assert.Equal(t, "remnawave.user.unknown_event", got)
	})
}
