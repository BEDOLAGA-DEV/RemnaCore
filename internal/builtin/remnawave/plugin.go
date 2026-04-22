package remnawave

import (
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

// PluginSlug is the canonical slug for the remnawave-provider plugin.
// It must match plugin.BuiltInSlugRemnawaveProvider.
const PluginSlug = plugin.BuiltInSlugRemnawaveProvider

func init() {
	builtin.RegisterPlugin(Plugin())
}

// Plugin returns the expanded built-in plugin definition for remnawave-provider.
// This overrides the basic definition in plugin.BuiltInPlugins() with full
// pages, routes, and config fields.
func Plugin() plugin.BuiltInPluginDef {
	return plugin.BuiltInPluginDef{
		Slug:         PluginSlug,
		Name:         "Remnawave Provider",
		Version:      "2.0.0",
		Description:  "Multi-panel VPN provider management with node monitoring, user management, traffic analytics, drift reconciliation, and geo routing.",
		Author:       "RemnaCore",
		ConfigFields: remnawaveConfigFields(),
		Pages:        remnawavePages(),
		Routes:       remnawaveRoutes(),
	}
}

// RegisterRoutes maps the plugin's manifest route functions to native Go handlers.
func RegisterRoutes(registry *gateway.BuiltinRouteRegistry, h *Handler) {
	r := func(fn string, handler func(http.ResponseWriter, *http.Request)) {
		registry.Register(PluginSlug, fn, handler)
	}

	// Panels
	r("list_panels", h.ListPanels)
	r("create_panel", h.CreatePanel)
	r("update_panel", h.UpdatePanel)
	r("delete_panel", h.DeletePanel)
	r("test_panel", h.TestPanel)
	r("set_panel_maintenance", h.SetPanelMaintenance)
	r("get_panel_stats", h.GetPanelStats)

	// Nodes
	r("list_nodes", h.ListNodes)
	r("get_node", h.GetNode)
	r("restart_node", h.RestartNode)
	r("disable_node", h.DisableNode)
	r("enable_node", h.EnableNode)
	r("get_node_stats", h.GetNodeStats)
	r("get_node_health", h.GetNodeHealth)
	r("get_node_users", h.GetNodeUsers)
	r("bulk_node_action", h.BulkNodeAction)
	r("create_node", h.CreateNode)
	r("update_node", h.UpdateNode)
	r("delete_node", h.DeleteNode)
	r("reset_node_traffic", h.ResetNodeTraffic)
	r("restart_all_nodes", h.RestartAllNodes)
	r("reorder_nodes", h.ReorderNodes)
	r("get_node_tags", h.GetNodeTags)
	r("get_system_info", h.GetSystemInfo)

	// Users
	r("search_users", h.SearchUsers)
	r("get_user", h.GetUser)
	r("update_user", h.UpdateUser)
	r("reset_user_traffic", h.ResetUserTraffic)
	r("revoke_user_subscription", h.RevokeUserSubscription)
	r("disconnect_user", h.DisconnectUser)
	r("get_user_sessions", h.GetUserSessions)
	r("get_user_hwid", h.GetUserHWID)
	r("delete_user_hwid", h.DeleteUserHWID)
	r("get_user_subscription_url", h.GetUserSubscriptionURL)
	r("enable_user", h.EnableUser)
	r("disable_user", h.DisableUser)
	r("delete_user", h.DeleteUser)
	r("get_user_tags", h.GetUserTags)
	r("get_user_bandwidth", h.GetUserBandwidth)
	r("get_ip_fetch_result", h.GetIPFetchResult)

	// Squads
	r("list_internal_squads", h.ListInternalSquads)
	r("list_external_squads", h.ListExternalSquads)
	r("create_squad", h.CreateSquad)
	r("update_squad", h.UpdateSquad)
	r("delete_squad", h.DeleteSquad)
	r("add_nodes_to_squad", h.AddNodesToSquad)
	r("remove_nodes_from_squad", h.RemoveNodesFromSquad)
	r("reorder_squad", h.ReorderSquad)

	// Dashboard & Analytics
	r("get_dashboard_overview", h.GetDashboardOverview)
	r("get_dashboard_realtime", h.GetDashboardRealtime)
	r("get_traffic_top_users", h.GetTrafficTopUsers)
	r("get_traffic_by_node", h.GetTrafficByNode)
	r("get_traffic_by_country", h.GetTrafficByCountry)
	r("get_bandwidth_trends", h.GetBandwidthTrends)
	r("get_user_retention", h.GetUserRetention)
	r("get_hwid_distribution", h.GetHWIDDistribution)
	r("get_client_app_distribution", h.GetClientAppDistribution)

	// Drift & Reconciliation
	r("list_drift_events", h.ListDriftEvents)
	r("trigger_reconciliation", h.TriggerReconciliation)
	r("resolve_drift_event", h.ResolveDriftEvent)
	r("heal_drift_event", h.HealDriftEvent)

	// Geo Routing
	r("list_routing_rules", h.ListRoutingRules)
	r("create_routing_rule", h.CreateRoutingRule)
	r("update_routing_rule", h.UpdateRoutingRule)
	r("delete_routing_rule", h.DeleteRoutingRule)
	r("simulate_routing", h.SimulateRouting)

	// Hosts
	r("list_hosts", h.ListHosts)
	r("get_host", h.GetHost)
	r("create_host", h.CreateHost)
	r("update_host", h.UpdateHost)
	r("delete_host", h.DeleteHost)
	r("reorder_hosts", h.ReorderHosts)
	r("get_host_tags", h.GetHostTags)
	r("bulk_host_action", h.BulkHostAction)

	// Users Bulk
	r("bulk_delete_users_by_status", h.BulkDeleteUsersByStatus)
	r("bulk_update_users", h.BulkUpdateUsers)
	r("bulk_reset_traffic", h.BulkResetTraffic)
	r("bulk_revoke_subscription", h.BulkRevokeSubscription)
	r("bulk_delete_users", h.BulkDeleteUsers)
	r("bulk_update_squads", h.BulkUpdateSquads)
	r("bulk_extend_expiration", h.BulkExtendExpiration)
	r("bulk_update_all_users", h.BulkUpdateAllUsers)
	r("bulk_reset_all_traffic", h.BulkResetAllTraffic)
	r("bulk_extend_all_expiration", h.BulkExtendAllExpiration)

	// Config Profiles
	r("list_config_profiles", h.ListConfigProfiles)
	r("get_config_profile", h.GetConfigProfile)
	r("create_config_profile", h.CreateConfigProfile)
	r("update_config_profile", h.UpdateConfigProfile)
	r("delete_config_profile", h.DeleteConfigProfile)
	r("get_config_profile_inbounds", h.GetConfigProfileInbounds)
	r("get_all_inbounds", h.GetAllInbounds)
	r("reorder_config_profiles", h.ReorderConfigProfiles)

	// Node Plugins
	r("list_node_plugins", h.ListNodePlugins)
	r("get_node_plugin", h.GetNodePlugin)
	r("create_node_plugin", h.CreateNodePlugin)
	r("update_node_plugin", h.UpdateNodePlugin)
	r("delete_node_plugin", h.DeleteNodePlugin)
	r("reorder_node_plugins", h.ReorderNodePlugins)
	r("clone_node_plugin", h.CloneNodePlugin)

	// Subscriptions
	r("list_subscription_page_configs", h.ListSubscriptionPageConfigs)
	r("list_subscriptions", h.ListSubscriptions)
	r("get_subscription_by_uuid", h.GetSubscriptionByUUID)
	r("get_subscription_by_short_uuid", h.GetSubscriptionByShortUUID)
	r("get_subscription_by_username", h.GetSubscriptionByUsername)

	// External Squads
	r("create_external_squad", h.CreateExternalSquad)
	r("update_external_squad", h.UpdateExternalSquad)
	r("delete_external_squad", h.DeleteExternalSquad)
	r("get_squad_by_uuid", h.GetSquadByUUID)
	r("get_squad_nodes", h.GetSquadNodes)
	r("add_users_to_external_squad", h.AddUsersToExternalSquad)
	r("remove_users_from_external_squad", h.RemoveUsersFromExternalSquad)
	r("reorder_external_squads", h.ReorderExternalSquads)

	// HWID expanded
	r("list_all_hwid_devices", h.ListAllHWIDDevices)
	r("create_hwid_device", h.CreateHWIDDevice)
	r("delete_all_user_hwid", h.DeleteAllUserHWID)

	// IP Control node-level
	r("fetch_node_users_ips", h.FetchNodeUsersIPs)
	r("get_fetch_node_users_ips_result", h.GetFetchNodeUsersIPsResult)

	// Metadata
	r("get_user_metadata", h.GetUserMetadata)
	r("upsert_user_metadata", h.UpsertUserMetadata)
	r("get_node_metadata", h.GetNodeMetadata)
	r("upsert_node_metadata", h.UpsertNodeMetadata)
}

// remnawaveRoutes returns all HTTP routes for the remnawave-provider plugin.
func remnawaveRoutes() []plugin.ManifestRoute {
	return []plugin.ManifestRoute{
		// --- Panels ---
		{Method: "GET", Path: "/api/remnawave/panels", Function: "list_panels", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/panels", Function: "create_panel", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/panels/{panelID}/test", Function: "test_panel", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/panels/{panelID}/maintenance", Function: "set_panel_maintenance", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/panels/{panelID}/stats", Function: "get_panel_stats", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/panels/{panelID}", Function: "update_panel", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/panels/{panelID}", Function: "delete_panel", AdminOnly: true},

		// --- Nodes ---
		{Method: "GET", Path: "/api/remnawave/nodes", Function: "list_nodes", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/bulk/{panelID}", Function: "bulk_node_action", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}", Function: "get_node", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/restart", Function: "restart_node", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/disable", Function: "disable_node", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/enable", Function: "enable_node", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/stats", Function: "get_node_stats", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/health", Function: "get_node_health", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/users", Function: "get_node_users", AdminOnly: true},

		// --- Users ---
		{Method: "GET", Path: "/api/remnawave/users", Function: "search_users", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/{userUUID}", Function: "get_user", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/users/{panelID}/{userUUID}", Function: "update_user", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/{panelID}/{userUUID}/reset-traffic", Function: "reset_user_traffic", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/{panelID}/{userUUID}/revoke-subscription", Function: "revoke_user_subscription", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/{panelID}/{userUUID}/disconnect", Function: "disconnect_user", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/{userUUID}/sessions", Function: "get_user_sessions", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/{userUUID}/hwid", Function: "get_user_hwid", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/users/{panelID}/{userUUID}/hwid/{hwidID}", Function: "delete_user_hwid", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/{userUUID}/subscription-url", Function: "get_user_subscription_url", AdminOnly: true},

		// --- Squads ---
		{Method: "GET", Path: "/api/remnawave/squads/internal", Function: "list_internal_squads", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/squads/external", Function: "list_external_squads", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/{panelID}", Function: "create_squad", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/{panelID}/{squadUUID}/add-nodes", Function: "add_nodes_to_squad", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/{panelID}/{squadUUID}/remove-nodes", Function: "remove_nodes_from_squad", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/{panelID}/reorder", Function: "reorder_squad", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/squads/{panelID}/{squadUUID}", Function: "update_squad", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/squads/{panelID}/{squadUUID}", Function: "delete_squad", AdminOnly: true},

		// --- Dashboard & Analytics ---
		{Method: "GET", Path: "/api/remnawave/dashboard/overview", Function: "get_dashboard_overview", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/dashboard/realtime", Function: "get_dashboard_realtime", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/traffic-top-users", Function: "get_traffic_top_users", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/traffic-by-node", Function: "get_traffic_by_node", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/traffic-by-country", Function: "get_traffic_by_country", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/bandwidth-trends", Function: "get_bandwidth_trends", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/user-retention", Function: "get_user_retention", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/hwid-distribution", Function: "get_hwid_distribution", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/analytics/client-app-distribution", Function: "get_client_app_distribution", AdminOnly: true},

		// --- Drift & Reconciliation ---
		{Method: "GET", Path: "/api/remnawave/drift/events", Function: "list_drift_events", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/drift/reconcile", Function: "trigger_reconciliation", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/drift/events/{eventID}/resolve", Function: "resolve_drift_event", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/drift/events/{eventID}/heal", Function: "heal_drift_event", AdminOnly: true},

		// --- Geo Routing ---
		{Method: "GET", Path: "/api/remnawave/routing/rules", Function: "list_routing_rules", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/routing/rules", Function: "create_routing_rule", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/routing/simulate", Function: "simulate_routing", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/routing/rules/{ruleID}", Function: "update_routing_rule", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/routing/rules/{ruleID}", Function: "delete_routing_rule", AdminOnly: true},

		// --- Nodes expanded ---
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}", Function: "create_node", AdminOnly: true},
		{Method: "PATCH", Path: "/api/remnawave/nodes/{panelID}", Function: "update_node", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}", Function: "delete_node", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}/{nodeUUID}/reset-traffic", Function: "reset_node_traffic", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}/restart-all", Function: "restart_all_nodes", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/nodes/{panelID}/reorder", Function: "reorder_nodes", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/nodes/{panelID}/tags", Function: "get_node_tags", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/nodes/{panelID}/system-info", Function: "get_system_info", AdminOnly: true},

		// --- Users expanded ---
		{Method: "POST", Path: "/api/remnawave/users/{panelID}/{userUUID}/enable", Function: "enable_user", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/{panelID}/{userUUID}/disable", Function: "disable_user", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/users/{panelID}/{userUUID}", Function: "delete_user", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/tags", Function: "get_user_tags", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/{userUUID}/bandwidth", Function: "get_user_bandwidth", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/users/{panelID}/ip-fetch-result/{jobID}", Function: "get_ip_fetch_result", AdminOnly: true},

		// --- Hosts ---
		{Method: "GET", Path: "/api/remnawave/hosts", Function: "list_hosts", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/hosts/{panelID}/{hostUUID}", Function: "get_host", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/hosts/{panelID}", Function: "create_host", AdminOnly: true},
		{Method: "PATCH", Path: "/api/remnawave/hosts/{panelID}", Function: "update_host", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/hosts/{panelID}/{hostUUID}", Function: "delete_host", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/hosts/{panelID}/reorder", Function: "reorder_hosts", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/hosts/{panelID}/tags", Function: "get_host_tags", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/hosts/{panelID}/bulk", Function: "bulk_host_action", AdminOnly: true},

		// --- Users bulk ---
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/delete-by-status", Function: "bulk_delete_users_by_status", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/update", Function: "bulk_update_users", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/reset-traffic", Function: "bulk_reset_traffic", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/revoke-subscription", Function: "bulk_revoke_subscription", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/delete", Function: "bulk_delete_users", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/update-squads", Function: "bulk_update_squads", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/extend-expiration", Function: "bulk_extend_expiration", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/all/update", Function: "bulk_update_all_users", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/all/reset-traffic", Function: "bulk_reset_all_traffic", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/users/bulk/{panelID}/all/extend-expiration", Function: "bulk_extend_all_expiration", AdminOnly: true},

		// --- Config profiles ---
		{Method: "GET", Path: "/api/remnawave/config-profiles/{panelID}", Function: "list_config_profiles", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/config-profiles/{panelID}/{profileUUID}", Function: "get_config_profile", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/config-profiles/{panelID}", Function: "create_config_profile", AdminOnly: true},
		{Method: "PATCH", Path: "/api/remnawave/config-profiles/{panelID}", Function: "update_config_profile", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/config-profiles/{panelID}/{profileUUID}", Function: "delete_config_profile", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/config-profiles/{panelID}/{profileUUID}/inbounds", Function: "get_config_profile_inbounds", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/config-profiles/{panelID}/inbounds", Function: "get_all_inbounds", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/config-profiles/{panelID}/reorder", Function: "reorder_config_profiles", AdminOnly: true},

		// --- Node plugins ---
		{Method: "GET", Path: "/api/remnawave/node-plugins/{panelID}", Function: "list_node_plugins", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/node-plugins/{panelID}/{pluginUUID}", Function: "get_node_plugin", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/node-plugins/{panelID}", Function: "create_node_plugin", AdminOnly: true},
		{Method: "PATCH", Path: "/api/remnawave/node-plugins/{panelID}", Function: "update_node_plugin", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/node-plugins/{panelID}/{pluginUUID}", Function: "delete_node_plugin", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/node-plugins/{panelID}/reorder", Function: "reorder_node_plugins", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/node-plugins/{panelID}/{pluginUUID}/clone", Function: "clone_node_plugin", AdminOnly: true},

		// --- Subscriptions ---
		{Method: "GET", Path: "/api/remnawave/subscription-page-configs", Function: "list_subscription_page_configs", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/subscriptions/{panelID}", Function: "list_subscriptions", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/subscriptions/{panelID}/by-uuid/{subUUID}", Function: "get_subscription_by_uuid", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/subscriptions/{panelID}/by-short-uuid/{shortUUID}", Function: "get_subscription_by_short_uuid", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/subscriptions/{panelID}/by-username/{username}", Function: "get_subscription_by_username", AdminOnly: true},

		// --- External Squads ---
		{Method: "POST", Path: "/api/remnawave/squads/external/{panelID}", Function: "create_external_squad", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/squads/external/{panelID}/{squadUUID}", Function: "update_external_squad", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/squads/external/{panelID}/{squadUUID}", Function: "delete_external_squad", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/squads/{panelID}/{squadUUID}", Function: "get_squad_by_uuid", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/squads/{panelID}/{squadUUID}/nodes", Function: "get_squad_nodes", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/external/{panelID}/{squadUUID}/add-users", Function: "add_users_to_external_squad", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/external/{panelID}/{squadUUID}/remove-users", Function: "remove_users_from_external_squad", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/squads/external/{panelID}/reorder", Function: "reorder_external_squads", AdminOnly: true},

		// --- HWID expanded ---
		{Method: "GET", Path: "/api/remnawave/hwid/{panelID}/devices", Function: "list_all_hwid_devices", AdminOnly: true},
		{Method: "POST", Path: "/api/remnawave/hwid/{panelID}/devices", Function: "create_hwid_device", AdminOnly: true},
		{Method: "DELETE", Path: "/api/remnawave/hwid/{panelID}/{userUUID}/all", Function: "delete_all_user_hwid", AdminOnly: true},

		// --- IP Control node-level ---
		{Method: "POST", Path: "/api/remnawave/ip-control/{panelID}/node/{nodeUUID}/fetch", Function: "fetch_node_users_ips", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/ip-control/{panelID}/node/result/{jobID}", Function: "get_fetch_node_users_ips_result", AdminOnly: true},

		// --- Metadata ---
		{Method: "GET", Path: "/api/remnawave/metadata/{panelID}/user/{userUUID}", Function: "get_user_metadata", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/metadata/{panelID}/user/{userUUID}", Function: "upsert_user_metadata", AdminOnly: true},
		{Method: "GET", Path: "/api/remnawave/metadata/{panelID}/node/{nodeUUID}", Function: "get_node_metadata", AdminOnly: true},
		{Method: "PUT", Path: "/api/remnawave/metadata/{panelID}/node/{nodeUUID}", Function: "upsert_node_metadata", AdminOnly: true},
	}
}

// remnawavePages returns the 8 admin pages for the plugin.
func remnawavePages() []plugin.ManifestPage {
	return []plugin.ManifestPage{
		{
			Path:       "panels",
			Title:      "Panels",
			Icon:       "Server",
			Menu:       plugin.PageMenuAdmin,
			Collection: CollectionPanelConnections,
			Fields: []plugin.ManifestPageField{
				{Key: "slug", Label: "Slug", Type: "text", Required: true},
				{Key: "url", Label: "Panel URL", Type: "text", Required: true},
				{Key: "api_token", Label: "API Token", Type: "secret", Required: true},
				{Key: "region", Label: "Region", Type: "text"},
				{Key: "priority", Label: "Priority", Type: "number", Default: "0"},
				{Key: "status", Label: "Status", Type: "select", Default: "active", Options: []string{"active", "readonly", "disabled", "maintenance"}},
				{Key: "tls_skip_verify", Label: "Skip TLS Verify", Type: "boolean", Default: "false"},
			},
		},
		// Custom pages — rendered by dedicated React components, not generic CRUD.
		// Collection is set to a dummy value to prevent empty JSON editors;
		// the frontend detects these via the custom page registry and renders
		// specialized components instead.
		{Path: "dashboard", Title: "Dashboard", Icon: "LayoutDashboard", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "nodes", Title: "Nodes", Icon: "Network", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "users", Title: "VPN Users", Icon: "Users", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "ip-management", Title: "IP Management", Icon: "Globe", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "squads-internal", Title: "Internal Squads", Icon: "Boxes", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "squads-external", Title: "External Squads", Icon: "Boxes", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "hosts", Title: "Hosts", Icon: "Server", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{Path: "subscription-configs", Title: "Sub Page Configs", Icon: "FileText", Menu: plugin.PageMenuAdmin, Collection: "_custom"},
		{
			Path:       "geo-routing",
			Title:      "Geo Routing",
			Icon:       "Globe",
			Menu:       plugin.PageMenuAdmin,
			Collection: CollectionGeoRoutingRules,
			Fields: []plugin.ManifestPageField{
				{Key: "country_code", Label: "Country (ISO-3166)", Type: "text", Required: true},
				{Key: "panel_id", Label: "Panel ID", Type: "text", Required: true},
				{Key: "priority", Label: "Priority", Type: "number", Default: "0"},
				{Key: "is_active", Label: "Active", Type: "boolean", Default: "true"},
			},
		},
	}
}

// remnawaveConfigFields returns the 23+ config fields for the plugin.
func remnawaveConfigFields() map[string]plugin.ManifestConfigField {
	return map[string]plugin.ManifestConfigField{
		// Webhook secret for verifying incoming panel webhooks.
		plugin.RemnawaveConfigKeyWebhookSecret: {
			Type:  "secret",
			Label: "Webhook Signing Secret",
		},

		// --- Provisioning ---
		"default_squad_strategy": {
			Type:    "select",
			Label:   "Default Squad Assignment",
			Options: []string{"all_internal", "round_robin", "geo_based", "manual"},
			Default: "all_internal",
		},
		"inactive_user_grace_period_hours": {
			Type:    "number",
			Label:   "Grace period before disabling (hours)",
			Default: "24",
		},
		"auto_delete_after_days": {
			Type:    "number",
			Label:   "Delete user after N days inactive",
			Default: "90",
		},

		// --- Traffic ---
		"traffic_strategy_default": {
			Type:    "select",
			Label:   "Default Traffic Reset Strategy",
			Options: []string{"no_reset", "day", "week", "month"},
			Default: "month",
		},
		"oversell_protection_pct": {
			Type:    "number",
			Label:   "Oversell alert threshold (%)",
			Default: "80",
		},

		// --- HTTP client tuning ---
		"http_timeout_ms": {
			Type:    "number",
			Label:   "HTTP Request Timeout (ms)",
			Default: "30000",
		},
		"http_retry_max_attempts": {
			Type:    "number",
			Label:   "Max Retry Attempts",
			Default: "3",
		},
		"http_retry_backoff_ms": {
			Type:    "number",
			Label:   "Retry Backoff Base (ms)",
			Default: "500",
		},
		"http_max_concurrent_requests": {
			Type:    "number",
			Label:   "Max Concurrent Requests per Panel",
			Default: "20",
		},
		"http_tls_skip_verify": {
			Type:    "boolean",
			Label:   "Skip TLS Verification (dev only!)",
			Default: "false",
		},

		// --- Circuit breaker ---
		"cb_consecutive_failures": {
			Type:    "number",
			Label:   "Circuit Breaker Failure Threshold",
			Default: "5",
		},
		"cb_timeout_seconds": {
			Type:    "number",
			Label:   "Circuit Breaker Open → Half-Open Timeout (s)",
			Default: "30",
		},

		// --- Drift ---
		"reconciliation_enabled": {
			Type:    "boolean",
			Label:   "Enable Periodic Drift Reconciliation",
			Default: "true",
		},
		"reconciliation_interval_minutes": {
			Type:    "number",
			Label:   "Reconciliation Interval (minutes)",
			Default: "60",
		},
		"drift_policy": {
			Type:    "select",
			Label:   "Drift Policy",
			Options: []string{"observe", "auto_heal", "alert_only"},
			Default: "alert_only",
		},

		// --- Health monitoring ---
		"node_health_check_interval_seconds": {
			Type:    "number",
			Label:   "Node Health Check Interval (s)",
			Default: "60",
		},

		// --- Traffic sync ---
		"traffic_sync_enabled": {
			Type:    "boolean",
			Label:   "Sync User Traffic Stats",
			Default: "true",
		},
		"traffic_sync_interval_minutes": {
			Type:    "number",
			Label:   "Traffic Sync Interval (minutes)",
			Default: "15",
		},

		// --- Feature toggles ---
		"feature_hwid_enforcement": {
			Type:    "boolean",
			Label:   "Enforce HWID limits",
			Default: "false",
		},
		"feature_bandwidth_alerts": {
			Type:    "boolean",
			Label:   "Emit bandwidth threshold events",
			Default: "true",
		},

		// --- Subscription URL ---
		"subscription_page_url": {
			Type:  "string",
			Label: "Subscription Page Public URL",
		},
		"subscription_preferred_client": {
			Type:    "select",
			Label:   "Preferred Client App",
			Options: []string{"auto", "v2raytun", "happ", "streisand", "singbox", "karing"},
			Default: "auto",
		},
	}
}
