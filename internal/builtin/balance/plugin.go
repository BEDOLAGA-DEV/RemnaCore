package balance

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

// Plugin returns the built-in plugin definition for user-balance.
func Plugin() plugin.BuiltInPluginDef {
	return plugin.BuiltInPluginDef{
		Slug:         PluginSlug,
		Name:         "User Balance",
		Version:      "1.0.0",
		Description:  "Two-wallet user balance (money + bonus) with ledger, cashback, and payment integration.",
		Author:       "RemnaCore",
		ConfigFields: balanceConfigFields(),
		Pages:        balancePages(),
		Routes:       balanceRoutes(),
	}
}

// RegisterRoutes maps route functions to native Go handlers.
func RegisterRoutes(registry *gateway.BuiltinRouteRegistry, h *Handler) {
	r := func(fn string, handler func(http.ResponseWriter, *http.Request)) {
		registry.Register(PluginSlug, fn, handler)
	}

	// User-facing
	r("get_my_balance", h.GetMyBalance)
	r("list_my_transactions", h.ListMyTransactions)
	r("create_topup", h.CreateTopUp)
	r("get_topup", h.GetTopUp)
	r("cancel_topup", h.CancelTopUp)

	// Admin
	r("admin_list_wallets", h.AdminListWallets)
	r("admin_get_user_balance", h.AdminGetUserBalance)
	r("admin_adjust", h.AdminAdjust)
	r("admin_transfer", h.AdminTransfer)
	r("admin_user_ledger", h.AdminUserLedger)
	r("admin_ledger", h.AdminLedger)
	r("admin_list_topups", h.AdminListTopUps)
	r("admin_approve_topup", h.AdminApproveTopUp)
	r("admin_reject_topup", h.AdminRejectTopUp)
	r("admin_analytics", h.AdminAnalytics)
	r("admin_export_csv", h.AdminExportCSV)
}

func balanceRoutes() []plugin.ManifestRoute {
	routes := []plugin.ManifestRoute{
		// User-facing
		{Method: "GET", Path: "/api/balance/me", Function: "get_my_balance", Public: false},
		{Method: "GET", Path: "/api/balance/me/transactions", Function: "list_my_transactions", Public: false},
		{Method: "POST", Path: "/api/balance/topup", Function: "create_topup", Public: false},
		{Method: "GET", Path: "/api/balance/topups/{id}", Function: "get_topup", Public: false},
		{Method: "POST", Path: "/api/balance/topups/{id}/cancel", Function: "cancel_topup", Public: false},

		// Admin
		{Method: "GET", Path: "/api/balance/admin/wallets", Function: "admin_list_wallets", AdminOnly: true},
		{Method: "GET", Path: "/api/balance/admin/users/{userID}", Function: "admin_get_user_balance", AdminOnly: true},
		{Method: "POST", Path: "/api/balance/admin/users/{userID}/adjust", Function: "admin_adjust", AdminOnly: true},
		{Method: "POST", Path: "/api/balance/admin/users/{userID}/transfer", Function: "admin_transfer", AdminOnly: true},
		{Method: "GET", Path: "/api/balance/admin/users/{userID}/ledger", Function: "admin_user_ledger", AdminOnly: true},
		{Method: "GET", Path: "/api/balance/admin/ledger", Function: "admin_ledger", AdminOnly: true},
		{Method: "GET", Path: "/api/balance/admin/topups", Function: "admin_list_topups", AdminOnly: true},
		{Method: "POST", Path: "/api/balance/admin/topups/{id}/approve", Function: "admin_approve_topup", AdminOnly: true},
		{Method: "POST", Path: "/api/balance/admin/topups/{id}/reject", Function: "admin_reject_topup", AdminOnly: true},
		{Method: "GET", Path: "/api/balance/admin/analytics/overview", Function: "admin_analytics", AdminOnly: true},
		{Method: "POST", Path: "/api/balance/admin/export", Function: "admin_export_csv", AdminOnly: true},
	}

	// Assign RBAC permissions to all AdminOnly routes (Phase A §11 gap-close):
	//   - GET routes expose financial read data → billing.read
	//   - All mutations (POST) → billing.manage (platform-admin-only financial ops)
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

func balancePages() []plugin.ManifestPage {
	return []plugin.ManifestPage{
		{
			Path: "wallets", Title: "User Wallets", Icon: "Wallet", Menu: plugin.PageMenuAdmin,
			Collection: CollectionWallets,
			Fields: []plugin.ManifestPageField{
				{Key: "user_id", Label: "User ID", Type: "text", Required: true},
				{Key: "kind", Label: "Wallet", Type: "select", Required: true, Options: []string{"money", "bonus"}},
				{Key: "currency", Label: "Currency", Type: "text", Default: "RUB"},
				{Key: "balance_cents", Label: "Balance (cents)", Type: "number", Default: "0"},
				{Key: "hold_cents", Label: "Hold (cents)", Type: "number", Default: "0"},
			},
		},
		{
			Path: "topups", Title: "Top-ups", Icon: "ArrowUpCircle", Menu: plugin.PageMenuAdmin,
			Collection: CollectionTopUps,
			Fields: []plugin.ManifestPageField{
				{Key: "user_id", Label: "User ID", Type: "text", Required: true},
				{Key: "amount_cents", Label: "Amount (cents)", Type: "number", Required: true},
				{Key: "currency", Label: "Currency", Type: "text", Default: "RUB"},
				{Key: "status", Label: "Status", Type: "select", Default: "pending", Options: []string{"pending", "processing", "completed", "failed", "cancelled"}},
				{Key: "payment_method", Label: "Method", Type: "text"},
			},
		},
	}
}

func balanceConfigFields() map[string]plugin.ManifestConfigField {
	return map[string]plugin.ManifestConfigField{
		"enabled":            {Type: "boolean", Label: "Enable Balance Plugin", Default: "true"},
		"default_currency":   {Type: "select", Label: "Default Currency", Options: []string{"USD", "EUR", "RUB", "GBP"}, Default: "RUB"},
		"money_min_topup_cents":   {Type: "number", Label: "Min Top-up Amount (cents)", Default: "10000"},
		"money_max_topup_cents":   {Type: "number", Label: "Max Top-up Amount (cents)", Default: "10000000"},
		"money_max_balance_cents": {Type: "number", Label: "Max Wallet Balance (cents, 0=unlimited)", Default: "0"},
		"bonus_enabled":           {Type: "boolean", Label: "Enable Bonus Wallet", Default: "true"},
		"bonus_expiry_days":       {Type: "number", Label: "Bonus Expiry (days, 0=never)", Default: "90"},
		"bonus_max_pct_of_invoice": {Type: "number", Label: "Max Bonus Usage (% of invoice)", Default: "50"},
		"cashback_enabled":        {Type: "boolean", Label: "Auto-Cashback on Purchase", Default: "false"},
		"cashback_percent":        {Type: "number", Label: "Cashback % (basis points, 500=5%)", Default: "500"},
		"cashback_min_invoice_cents": {Type: "number", Label: "Min Invoice for Cashback (cents)", Default: "50000"},
		"topup_providers": {Type: "multiselect", Label: "Allowed Top-up Providers", Options: []string{"stripe", "yookassa", "crypto", "manual"}, Default: "stripe,yookassa,manual"},
		"user_can_view_transactions": {Type: "boolean", Label: "Show Transaction History to Users", Default: "true"},
		"user_can_initiate_topup":    {Type: "boolean", Label: "Allow User Self Top-up", Default: "true"},
		"topup_presets_cents":        {Type: "text", Label: "Quick Top-up Presets (comma-sep cents)", Default: "50000,100000,200000,500000,1000000"},
		"notify_admin_on_large_topup":   {Type: "boolean", Label: "Notify Admin on Large Top-up", Default: "true"},
		"large_topup_threshold_cents":   {Type: "number", Label: "Large Top-up Threshold (cents)", Default: "5000000"},
		"require_kyc_for_topup_above_cents": {Type: "number", Label: "Require KYC Above Amount (cents, 0=never)", Default: "0"},
	}
}
