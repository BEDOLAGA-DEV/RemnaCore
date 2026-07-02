package remnawave

import (
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/naming"
)

const (
	// PlatformTag is used to identify platform-managed users in Remnawave.
	PlatformTag = naming.PlatformTag

	// UsernamePrefix is prepended to all generated usernames.
	UsernamePrefix = naming.UsernamePrefix

	// UsernameShortIDLen is the number of characters taken from the platform
	// user ID when building a Remnawave username.
	UsernameShortIDLen = naming.UsernameShortIDLen
)

// webhookEventMap maps Remnawave "scope.event" strings to domain event types.
// Entries are grouped by Remnawave scope and sorted alphabetically within each group.
var webhookEventMap = map[string]string{
	// --- user scope ---
	"user.bandwidth_usage_threshold_reached": "subscription.traffic_warning",
	"user.created":                           "remnawave.user.synced",
	"user.deleted":                           "remnawave.user.deleted",
	"user.disabled":                          "subscription.binding_disabled",
	"user.enabled":                           "subscription.binding_enabled",
	"user.expired":                           "subscription.remnawave_expired",
	// NOTE: the per-threshold user.expired_24_hours_ago / user.expires_in_{24,48,72}_hours
	// keys were REMOVED in Remnawave 2.8.0. They are replaced by the unified
	// user.expiration event, handled specially in MapWebhookEvent by the sign of
	// meta.expiration (see below) — not via this map.
	"user.first_connected": "subscription.first_use",
	"user.limited":                           "binding.traffic_exceeded",
	"user.modified":                          "remnawave.user.modified",
	"user.not_connected":                     "subscription.user_not_connected",
	"user.revoked":                           "subscription.binding_revoked",
	"user.traffic_reset":                     "subscription.traffic_cycle_reset",

	// --- user_hwid_devices scope ---
	"user_hwid_devices.added":   "security.hwid_device_added",
	"user_hwid_devices.deleted": "security.hwid_device_deleted",

	// --- node scope ---
	"node.connection_lost":     "infra.node_down",
	"node.connection_restored": "infra.node_up",
	"node.created":             "infra.node_created",
	"node.deleted":             "infra.node_deleted",
	"node.disabled":            "infra.node_disabled",
	"node.enabled":             "infra.node_enabled",
	"node.modified":            "infra.node_modified",
	"node.traffic_notify":      "infra.node_traffic_notify",

	// --- service scope ---
	"service.login_attempt_failed":  "security.login_attempt_failed",
	"service.login_attempt_success": "security.login_attempt_success",
	"service.panel_started":         "infra.remnawave_restarted",
	"service.subpage_config_changed": "infra.subpage_config_changed",

	// --- errors scope ---
	"errors.bandwidth_usage_threshold_reached_max_notifications": "subscription.traffic_warning_max_reached",

	// --- crm scope ---
	"crm.infra_billing_node_payment_due_today":     "billing.node_payment_due_today",
	"crm.infra_billing_node_payment_in_24hrs":      "billing.node_payment_due_24h",
	"crm.infra_billing_node_payment_in_48hrs":      "billing.node_payment_due_48h",
	"crm.infra_billing_node_payment_in_7_days":     "billing.node_payment_due_7d",
	"crm.infra_billing_node_payment_overdue_24hrs": "billing.node_payment_overdue_24h",
	"crm.infra_billing_node_payment_overdue_48hrs": "billing.node_payment_overdue_48h",
	"crm.infra_billing_node_payment_overdue_7_days": "billing.node_payment_overdue_7d",

	// --- torrent_blocker scope ---
	"torrent_blocker.report": "security.torrent_blocker_report",
}

// BuildUsername constructs a deterministic Remnawave username from a platform
// user ID, a purpose label (e.g. "main"), and a numeric index.
// Format: p_{first8chars}_{purpose}_{index}
//
// Delegates to pkg/naming.BuildRemnawaveUsername to avoid import cycles.
func BuildUsername(platformUserID, purpose string, index int) string {
	return naming.BuildRemnawaveUsername(platformUserID, purpose, index)
}

// MapWebhookEvent translates a Remnawave webhook payload into a domain event
// type. Most events map through webhookEventMap keyed by "scope.event"; the
// unified 2.8.0 user.expiration event is special-cased on the sign of
// meta.expiration (signed hours: negative = hours before expiry → expiring
// soon; positive = hours after expiry → expired N hours ago). Unknown
// combinations fall back to "remnawave.{scope}.{event}".
func MapWebhookEvent(p WebhookPayload) string {
	if p.Scope == "user" && p.Event == "expiration" {
		if p.Meta.Expiration != nil && *p.Meta.Expiration > 0 {
			return "subscription.expired_24h_ago"
		}
		return "subscription.expiring_soon"
	}
	key := p.Scope + "." + p.Event
	if domainEvent, ok := webhookEventMap[key]; ok {
		return domainEvent
	}
	return fmt.Sprintf("remnawave.%s.%s", p.Scope, p.Event)
}
