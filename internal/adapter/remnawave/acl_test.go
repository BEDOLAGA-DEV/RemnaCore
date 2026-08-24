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
		for i := range 3 {
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
		// --- user scope ---
		{"user", "bandwidth_usage_threshold_reached", "subscription.traffic_warning"},
		{"user", "created", "remnawave.user.synced"},
		{"user", "deleted", "remnawave.user.deleted"},
		{"user", "disabled", "subscription.binding_disabled"},
		{"user", "enabled", "subscription.binding_enabled"},
		{"user", "expired", "subscription.remnawave_expired"},
		{"user", "first_connected", "subscription.first_use"},
		{"user", "limited", "binding.traffic_exceeded"},
		{"user", "modified", "remnawave.user.modified"},
		{"user", "not_connected", "subscription.user_not_connected"},
		{"user", "revoked", "subscription.binding_revoked"},
		{"user", "traffic_reset", "subscription.traffic_cycle_reset"},

		// --- user_hwid_devices scope ---
		{"user_hwid_devices", "added", "security.hwid_device_added"},
		{"user_hwid_devices", "deleted", "security.hwid_device_deleted"},

		// --- node scope ---
		{"node", "connection_lost", "infra.node_down"},
		{"node", "connection_restored", "infra.node_up"},
		{"node", "created", "infra.node_created"},
		{"node", "deleted", "infra.node_deleted"},
		{"node", "disabled", "infra.node_disabled"},
		{"node", "enabled", "infra.node_enabled"},
		{"node", "modified", "infra.node_modified"},
		{"node", "traffic_notify", "infra.node_traffic_notify"},

		// --- service scope ---
		{"service", "login_attempt_failed", "security.login_attempt_failed"},
		{"service", "login_attempt_success", "security.login_attempt_success"},
		{"service", "panel_started", "infra.remnawave_restarted"},
		{"service", "subpage_config_changed", "infra.subpage_config_changed"},

		// --- errors scope ---
		{"errors", "bandwidth_usage_threshold_reached_max_notifications", "subscription.traffic_warning_max_reached"},

		// --- crm scope ---
		{"crm", "infra_billing_node_payment_due_today", "billing.node_payment_due_today"},
		{"crm", "infra_billing_node_payment_in_24hrs", "billing.node_payment_due_24h"},
		{"crm", "infra_billing_node_payment_in_48hrs", "billing.node_payment_due_48h"},
		{"crm", "infra_billing_node_payment_in_7_days", "billing.node_payment_due_7d"},
		{"crm", "infra_billing_node_payment_overdue_24hrs", "billing.node_payment_overdue_24h"},
		{"crm", "infra_billing_node_payment_overdue_48hrs", "billing.node_payment_overdue_48h"},
		{"crm", "infra_billing_node_payment_overdue_7_days", "billing.node_payment_overdue_7d"},

		// --- torrent_blocker scope ---
		{"torrent_blocker", "report", "security.torrent_blocker_report"},
	}

	for _, tt := range tests {
		t.Run(tt.scope+"."+tt.event, func(t *testing.T) {
			got := MapWebhookEvent(WebhookPayload{Scope: tt.scope, Event: tt.event})
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestMapWebhookEvent_Unknown(t *testing.T) {
	t.Run("falls back to remnawave prefix", func(t *testing.T) {
		got := MapWebhookEvent(WebhookPayload{Scope: "billing", Event: "invoice_paid"})
		assert.Equal(t, "remnawave.billing.invoice_paid", got)
	})

	t.Run("unknown user event", func(t *testing.T) {
		got := MapWebhookEvent(WebhookPayload{Scope: "user", Event: "unknown_event"})
		assert.Equal(t, "remnawave.user.unknown_event", got)
	})
}

// TestMapWebhookEvent_UnifiedExpiration covers the 2.8.0 unified user.expiration
// event: the sign of meta.expiration selects expiring-soon vs expired-N-ago.
func TestMapWebhookEvent_UnifiedExpiration(t *testing.T) {
	hours := func(h int) *int { return &h }

	cases := []struct {
		name     string
		meta     WebhookMeta
		expected string
	}{
		{"negative hours = expiring soon", WebhookMeta{Expiration: hours(-24)}, "subscription.expiring_soon"},
		{"positive hours = expired N ago", WebhookMeta{Expiration: hours(24)}, "subscription.expired_24h_ago"},
		{"nil meta = expiring soon", WebhookMeta{}, "subscription.expiring_soon"},
		{"zero = expiring soon", WebhookMeta{Expiration: hours(0)}, "subscription.expiring_soon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapWebhookEvent(WebhookPayload{Scope: "user", Event: "expiration", Meta: tc.meta})
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestMapWebhookEvent_QualifiedEventNaming covers the Remnawave 3 payload, which
// qualifies the event with its scope ("user.created") where version 2 sent the
// bare name ("created") and relied on the separate scope field. Concatenating
// the two unconditionally produced "user.user.created", which matched no
// mapping and was published as a subject no stream subscribes to.
func TestMapWebhookEvent_QualifiedEventNaming(t *testing.T) {
	cases := []struct {
		name     string
		scope    string
		event    string
		expected string
	}{
		{"v3 qualified user event", "user", "user.created", "remnawave.user.synced"},
		{"v2 bare user event", "user", "created", "remnawave.user.synced"},
		{"v3 qualified node event", "node", "node.disabled", "infra.node_disabled"},
		{"v2 bare node event", "node", "disabled", "infra.node_disabled"},
		{"unmapped event keeps a single scope", "user", "user.something_new", "remnawave.user.something_new"},
		{"scope omitted entirely", "", "user.deleted", "remnawave.user.deleted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapWebhookEvent(WebhookPayload{Scope: tc.scope, Event: tc.event})
			assert.Equal(t, tc.expected, got)
		})
	}
}

// The unified expiry event must be recognised under the Remnawave 3 naming too,
// where it arrives as "user.expiration" rather than "expiration".
func TestMapWebhookEvent_UnifiedExpiration_QualifiedNaming(t *testing.T) {
	hours := func(h int) *int { return &h }

	got := MapWebhookEvent(WebhookPayload{
		Scope: "user", Event: "user.expiration", Meta: WebhookMeta{Expiration: hours(25)},
	})
	assert.Equal(t, "subscription.expired_24h_ago", got)

	got = MapWebhookEvent(WebhookPayload{
		Scope: "user", Event: "user.expiration", Meta: WebhookMeta{Expiration: hours(-6)},
	})
	assert.Equal(t, "subscription.expiring_soon", got)
}
