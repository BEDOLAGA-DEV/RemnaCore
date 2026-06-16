package checkout

import (
	"net/http"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
)

func init() {
	builtin.RegisterPlugin(Plugin())
}

// Plugin returns the built-in plugin definition for checkout.
func Plugin() plugin.BuiltInPluginDef {
	return plugin.BuiltInPluginDef{
		Slug:         PluginSlug,
		Name:         "Unified Checkout",
		Version:      "1.0.0",
		Description:  "Unified checkout with split-payment, saved methods, promo codes, and abandonment tracking.",
		Author:       "RemnaCore",
		ConfigFields: checkoutConfigFields(),
		Pages:        checkoutPages(),
		Routes:       checkoutRoutes(),
	}
}

// RegisterRoutes maps route functions to native Go handlers.
func RegisterRoutes(registry *gateway.BuiltinRouteRegistry, h *Handler) {
	r := func(fn string, handler func(http.ResponseWriter, *http.Request)) {
		registry.Register(PluginSlug, fn, handler)
	}

	// Session flow
	r("create_session", h.CreateSession)
	r("get_session", h.GetSession)
	r("list_sources", h.ListSources)
	r("validate_session", h.ValidateSession)
	r("confirm_session", h.ConfirmSession)
	r("cancel_session", h.CancelSession)
	r("apply_promo", h.ApplyPromo)

	// Saved methods
	r("list_saved", h.ListSavedMethods)
	r("set_default", h.SetDefaultMethod)
	r("delete_saved", h.DeleteSavedMethod)

	// Admin
	r("admin_list_sessions", h.AdminListSessions)
	r("admin_get_session", h.AdminGetSession)
	r("admin_abandonment", h.AdminAbandonment)
	r("admin_funnel", h.AdminFunnel)
	r("admin_methods_breakdown", h.AdminMethodsBreakdown)
}

func checkoutRoutes() []plugin.ManifestRoute {
	routes := []plugin.ManifestRoute{
		// Session flow
		{Method: "POST", Path: "/api/checkout/session", Function: "create_session", Public: false},
		{Method: "GET", Path: "/api/checkout/saved-methods", Function: "list_saved", Public: false},
		{Method: "GET", Path: "/api/checkout/{sessionID}", Function: "get_session", Public: false},
		{Method: "GET", Path: "/api/checkout/{sessionID}/sources", Function: "list_sources", Public: false},
		{Method: "POST", Path: "/api/checkout/{sessionID}/validate", Function: "validate_session", Public: false},
		{Method: "POST", Path: "/api/checkout/{sessionID}/confirm", Function: "confirm_session", Public: false},
		{Method: "POST", Path: "/api/checkout/{sessionID}/cancel", Function: "cancel_session", Public: false},
		{Method: "POST", Path: "/api/checkout/{sessionID}/apply-promo", Function: "apply_promo", Public: false},

		// Saved methods
		{Method: "POST", Path: "/api/checkout/saved-methods/{id}/default", Function: "set_default", Public: false},
		{Method: "DELETE", Path: "/api/checkout/saved-methods/{id}", Function: "delete_saved", Public: false},

		// Admin
		{Method: "GET", Path: "/api/checkout/admin/sessions", Function: "admin_list_sessions", AdminOnly: true},
		{Method: "GET", Path: "/api/checkout/admin/sessions/{id}", Function: "admin_get_session", AdminOnly: true},
		{Method: "GET", Path: "/api/checkout/admin/abandonment", Function: "admin_abandonment", AdminOnly: true},
		{Method: "GET", Path: "/api/checkout/admin/analytics/funnel", Function: "admin_funnel", AdminOnly: true},
		{Method: "GET", Path: "/api/checkout/admin/analytics/methods", Function: "admin_methods_breakdown", AdminOnly: true},
	}

	// Assign RBAC permissions to all AdminOnly routes (Phase A §11 gap-close):
	//   - All 5 admin routes are GET-only (read/analytics on checkout sessions and
	//     payment methods) → billing.read is the correct gate.
	//   - No mutation routes exist in the admin surface of this plugin, so
	//     billing.manage is not assigned here.
	for i := range routes {
		if !routes[i].AdminOnly {
			continue
		}
		if routes[i].Method == http.MethodGet {
			routes[i].RequiredPermission = string(rbac.BillingRead)
		} else {
			routes[i].RequiredPermission = string(rbac.BillingManage)
		}
	}
	return routes
}

func checkoutPages() []plugin.ManifestPage {
	return []plugin.ManifestPage{
		{
			Path: "checkout-sessions", Title: "Checkout Sessions", Icon: "ShoppingCart", Menu: plugin.PageMenuAdmin,
			Collection: collectionSessions,
			Fields: []plugin.ManifestPageField{
				{Key: "user_id", Label: "User ID", Type: "text"},
				{Key: "tariff_id", Label: "Tariff ID", Type: "text"},
				{Key: "final_price_cents", Label: "Price (cents)", Type: "number"},
				{Key: "currency", Label: "Currency", Type: "text"},
				{Key: "status", Label: "Status", Type: "select", Options: []string{"pending", "processing", "completed", "failed", "expired", "abandoned", "cancelled"}},
				{Key: "payment_mode", Label: "Mode", Type: "select", Options: []string{"single", "split"}},
				{Key: "promo_code", Label: "Promo Code", Type: "text"},
			},
		},
		{
			Path: "saved-methods", Title: "Saved Methods", Icon: "CreditCard", Menu: plugin.PageMenuAdmin,
			Collection: collectionSavedMethods,
			Fields: []plugin.ManifestPageField{
				{Key: "user_id", Label: "User ID", Type: "text"},
				{Key: "provider", Label: "Provider", Type: "text"},
				{Key: "last4", Label: "Last 4", Type: "text"},
				{Key: "brand", Label: "Brand", Type: "text"},
				{Key: "is_default", Label: "Default", Type: "boolean"},
			},
		},
	}
}

func checkoutConfigFields() map[string]plugin.ManifestConfigField {
	return map[string]plugin.ManifestConfigField{
		"session_ttl_minutes":           {Type: "number", Label: "Checkout Session TTL (minutes)", Default: "30"},
		"abandonment_reminder_minutes":  {Type: "number", Label: "Abandonment Reminder Delay (minutes, 0=off)", Default: "60"},
		"split_payment_enabled":         {Type: "boolean", Label: "Enable Split Payment", Default: "true"},
		"split_max_sources":             {Type: "number", Label: "Max Sources in Split", Default: "3"},
		"save_payment_methods":          {Type: "boolean", Label: "Allow Saving Payment Methods", Default: "true"},
		"prefer_saved_method":           {Type: "boolean", Label: "Prefer Saved Method by Default", Default: "true"},
		"balance_first_strategy":        {Type: "select", Label: "Balance Usage Strategy", Options: []string{"never_auto", "prefer_balance", "prefer_card", "max_balance_then_card"}, Default: "prefer_card"},
		"show_price_breakdown":          {Type: "boolean", Label: "Show Detailed Price Breakdown", Default: "true"},
		"require_terms_acceptance":      {Type: "boolean", Label: "Require Terms Acceptance", Default: "true"},
		"enable_guest_checkout":         {Type: "boolean", Label: "Enable Guest Checkout", Default: "false"},
		"post_checkout_redirect_url":    {Type: "string", Label: "Post-Checkout Success URL", Default: "/cabinet/subscriptions"},
	}
}
