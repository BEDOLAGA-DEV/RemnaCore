package remnawave

// VPN user events.
const (
	EventVPNUserCreated             = "vpn.user.created"
	EventVPNUserUpdated             = "vpn.user.updated"
	EventVPNUserDeleted             = "vpn.user.deleted"
	EventVPNUserEnabled             = "vpn.user.enabled"
	EventVPNUserDisabled            = "vpn.user.disabled"
	EventVPNUserTrafficReset        = "vpn.user.traffic_reset"
	EventVPNUserSubscriptionRevoked = "vpn.user.subscription_revoked"
	EventVPNUserLimited             = "vpn.user.limited"
)

// VPN node events.
const (
	EventVPNNodeRegistered       = "vpn.node.registered"
	EventVPNNodeUnregistered     = "vpn.node.unregistered"
	EventVPNNodeHealthChanged    = "vpn.node.health_changed"
	EventVPNNodeRestarted        = "vpn.node.restarted"
	EventVPNNodeTrafficThreshold = "vpn.node.traffic_threshold_reached"
)

// VPN panel events.
const (
	EventVPNPanelAdded             = "vpn.panel.added"
	EventVPNPanelRemoved           = "vpn.panel.removed"
	EventVPNPanelHealthChanged     = "vpn.panel.health_changed"
	EventVPNPanelFailoverTriggered = "vpn.panel.failover_triggered"
)

// VPN drift events.
const (
	EventVPNDriftDetected = "vpn.drift.detected"
	EventVPNDriftResolved = "vpn.drift.resolved"
)

// VPN HWID events.
const (
	EventVPNHWIDDeviceAdded   = "vpn.hwid.device_added"
	EventVPNHWIDDeviceRemoved = "vpn.hwid.device_removed"
	EventVPNHWIDLimitReached  = "vpn.hwid.limit_reached"
)

// VPN routing events.
const (
	EventVPNRoutingRuleChanged = "vpn.routing.rule_changed"
)

// Sync hook names — blocking hooks on the checkout/provisioning path.
const (
	HookVPNUserCreating           = "vpn.user.creating"
	HookVPNUserDeleting           = "vpn.user.deleting"
	HookVPNUserEnabling           = "vpn.user.enabling"
	HookVPNUserDisabling          = "vpn.user.disabling"
	HookVPNUserFetching           = "vpn.user.fetching"
	HookVPNUserFetched            = "vpn.user.fetched"
	HookVPNUserUpdating           = "vpn.user.updating"
	HookVPNUserResetTraffic       = "vpn.user.reset_traffic"
	HookVPNUserRevokeSubscription = "vpn.user.revoke_subscription"
	HookVPNPanelSelect            = "vpn.panel.select"
	HookVPNNodeRestart            = "vpn.node.restart"
	HookVPNHWIDValidate           = "vpn.hwid.validate"
	HookVPNSubscriptionURLBuild   = "vpn.subscription_url.build"
)

// Async hook names — fire-and-forget for observability and side effects.
const (
	HookVPNUserCreated          = "vpn.user.created"
	HookVPNUserDeleted          = "vpn.user.deleted"
	HookVPNUserDisabled         = "vpn.user.disabled"
	HookVPNUserTrafficReset     = "vpn.user.traffic_reset"
	HookVPNUserLimited          = "vpn.user.limited"
	HookVPNNodeHealthChanged    = "vpn.node.health_changed"
	HookVPNNodeTrafficThreshold = "vpn.node.traffic_threshold"
	HookVPNDriftDetected        = "vpn.drift.detected"
	HookVPNBandwidthSnapshot    = "vpn.bandwidth.snapshot"
	HookVPNHWIDDeviceAdded      = "vpn.hwid.device_added"
	HookVPNHWIDDeviceRemoved    = "vpn.hwid.device_removed"
)
