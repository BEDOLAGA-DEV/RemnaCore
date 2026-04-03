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
var webhookEventMap = map[string]string{
	"user.created":                                    "remnawave.user.synced",
	"user.limited":                                    "binding.traffic_exceeded",
	"user.expired":                                    "subscription.remnawave_expired",
	"user.disabled":                                   "subscription.binding_disabled",
	"user.enabled":                                    "subscription.binding_enabled",
	"user.traffic_reset":                              "subscription.traffic_cycle_reset",
	"user.first_connected":                            "subscription.first_use",
	"user.bandwidth_usage_threshold_reached":          "subscription.traffic_warning",
	"user.expires_in_72_hours":                        "subscription.expiring_soon",
	"user.expires_in_48_hours":                        "subscription.expiring_soon",
	"user.expires_in_24_hours":                        "subscription.expiring_soon",
	"node.connection_lost":                            "infra.node_down",
	"node.connection_restored":                        "infra.node_up",
	"service.panel_started":                           "infra.remnawave_restarted",
}

// BuildUsername constructs a deterministic Remnawave username from a platform
// user ID, a purpose label (e.g. "main"), and a numeric index.
// Format: p_{first8chars}_{purpose}_{index}
//
// Delegates to pkg/naming.BuildRemnawaveUsername to avoid import cycles.
func BuildUsername(platformUserID, purpose string, index int) string {
	return naming.BuildRemnawaveUsername(platformUserID, purpose, index)
}

// MapWebhookEvent translates a Remnawave webhook scope and event into a domain
// event type. Unknown combinations fall back to "remnawave.{scope}.{event}".
func MapWebhookEvent(scope, event string) string {
	key := scope + "." + event
	if domainEvent, ok := webhookEventMap[key]; ok {
		return domainEvent
	}
	return fmt.Sprintf("remnawave.%s.%s", scope, event)
}
