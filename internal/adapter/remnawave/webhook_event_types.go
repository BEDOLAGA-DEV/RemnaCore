package remnawave

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Webhook-originated EventType constants. These events are produced by the
// Remnawave VPN panel and translated through the ACL webhook mapping. They
// use map[string]any payloads (no typed structs) because the payload schema
// is defined by Remnawave, not by RemnaCore domain logic.
//
// The constants are organised by the Remnawave scope they originate from.

// --- Remnawave user scope ---

const (
	// EventRemnawaveUserModified is published when Remnawave reports a user modification.
	EventRemnawaveUserModified domainevent.EventType = "remnawave.user.modified"

	// EventRemnawaveUserDeleted is published when Remnawave reports a user deletion.
	EventRemnawaveUserDeleted domainevent.EventType = "remnawave.user.deleted"
)

// --- Subscription scope (webhook-originated) ---

const (
	// EventSubBindingRevoked is published when Remnawave revokes a user's access.
	EventSubBindingRevoked domainevent.EventType = "subscription.binding_revoked"

	// EventSubExpired24hAgo is published when a user expired more than 24 hours ago.
	EventSubExpired24hAgo domainevent.EventType = "subscription.expired_24h_ago"

	// EventSubUserNotConnected is published when a user has never connected.
	EventSubUserNotConnected domainevent.EventType = "subscription.user_not_connected"

	// EventSubTrafficWarningMaxReached is published when bandwidth notifications
	// have reached the maximum threshold.
	EventSubTrafficWarningMaxReached domainevent.EventType = "subscription.traffic_warning_max_reached"
)

// --- Security scope ---

const (
	// EventSecurityHWIDDeviceAdded is published when a new HWID device is registered.
	EventSecurityHWIDDeviceAdded domainevent.EventType = "security.hwid_device_added"

	// EventSecurityHWIDDeviceDeleted is published when an HWID device is removed.
	EventSecurityHWIDDeviceDeleted domainevent.EventType = "security.hwid_device_deleted"

	// EventSecurityLoginAttemptFailed is published on a failed panel login attempt.
	EventSecurityLoginAttemptFailed domainevent.EventType = "security.login_attempt_failed"

	// EventSecurityLoginAttemptSuccess is published on a successful panel login.
	EventSecurityLoginAttemptSuccess domainevent.EventType = "security.login_attempt_success"

	// EventSecurityTorrentBlockerReport is published when the torrent blocker detects activity.
	EventSecurityTorrentBlockerReport domainevent.EventType = "security.torrent_blocker_report"
)

// --- Infra scope (webhook-originated) ---

const (
	// EventInfraNodeCreated is published when a new node is added to Remnawave.
	EventInfraNodeCreated domainevent.EventType = "infra.node_created"

	// EventInfraNodeModified is published when a node configuration is changed.
	EventInfraNodeModified domainevent.EventType = "infra.node_modified"

	// EventInfraNodeDisabled is published when a node is disabled in Remnawave.
	EventInfraNodeDisabled domainevent.EventType = "infra.node_disabled"

	// EventInfraNodeEnabled is published when a node is enabled in Remnawave.
	EventInfraNodeEnabled domainevent.EventType = "infra.node_enabled"

	// EventInfraNodeDeleted is published when a node is removed from Remnawave.
	EventInfraNodeDeleted domainevent.EventType = "infra.node_deleted"

	// EventInfraNodeTrafficNotify is published when a node traffic threshold is reached.
	EventInfraNodeTrafficNotify domainevent.EventType = "infra.node_traffic_notify"

	// EventInfraSubpageConfigChanged is published when the subscription page configuration changes.
	EventInfraSubpageConfigChanged domainevent.EventType = "infra.subpage_config_changed"
)

// --- Billing scope (CRM node payment reminders) ---

const (
	// EventBillingNodePaymentDue7d is published 7 days before a node payment is due.
	EventBillingNodePaymentDue7d domainevent.EventType = "billing.node_payment_due_7d"

	// EventBillingNodePaymentDue48h is published 48 hours before a node payment is due.
	EventBillingNodePaymentDue48h domainevent.EventType = "billing.node_payment_due_48h"

	// EventBillingNodePaymentDue24h is published 24 hours before a node payment is due.
	EventBillingNodePaymentDue24h domainevent.EventType = "billing.node_payment_due_24h"

	// EventBillingNodePaymentDueToday is published on the day a node payment is due.
	EventBillingNodePaymentDueToday domainevent.EventType = "billing.node_payment_due_today"

	// EventBillingNodePaymentOverdue24h is published 24 hours after a node payment was due.
	EventBillingNodePaymentOverdue24h domainevent.EventType = "billing.node_payment_overdue_24h"

	// EventBillingNodePaymentOverdue48h is published 48 hours after a node payment was due.
	EventBillingNodePaymentOverdue48h domainevent.EventType = "billing.node_payment_overdue_48h"

	// EventBillingNodePaymentOverdue7d is published 7 days after a node payment was due.
	EventBillingNodePaymentOverdue7d domainevent.EventType = "billing.node_payment_overdue_7d"
)
