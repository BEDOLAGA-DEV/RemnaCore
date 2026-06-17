// Package rbac is the single source of truth for the platform's authorization
// vocabulary: the catalog of atomic permissions and the built-in system roles.
// The identity.permissions / identity.roles / identity.role_permissions tables
// are a projection of this package, kept in sync by an idempotent boot upsert
// (see service.RBACCatalogSync). No other code may define permission strings.
package rbac

import "strings"

// Permission is a typed atomic capability of the form "resource.action".
type Permission string

// Resource returns the part before the dot (e.g. "tariffs" for "tariffs.write").
func (p Permission) Resource() string {
	r, _, _ := strings.Cut(string(p), ".")
	return r
}

// Action returns the part after the dot (e.g. "write" for "tariffs.write").
func (p Permission) Action() string {
	_, a, _ := strings.Cut(string(p), ".")
	return a
}

// Permission constants. Adding one here and to Catalog() is the ONLY way to
// introduce a new capability.
const (
	UsersRead           Permission = "users.read"
	UsersInvite         Permission = "users.invite"        // Phase B routes
	UsersAssignRole     Permission = "users.assign_role"   // Phase B routes
	RolesRead           Permission = "roles.read"
	RolesManage         Permission = "roles.manage"        // Phase B/D routes
	ShopsRead           Permission = "shops.read"
	ShopsManage         Permission = "shops.manage"
	ShopsBranding       Permission = "shops.branding"
	TariffsRead         Permission = "tariffs.read"
	TariffsWrite        Permission = "tariffs.write"
	CustomersRead       Permission = "customers.read"      // forward-looking (Phase C)
	CustomersManage     Permission = "customers.manage"    // forward-looking (Phase C)
	SubscriptionsRead   Permission = "subscriptions.read"
	SubscriptionsManage Permission = "subscriptions.manage" // forward-looking (Phase C)
	BillingRead         Permission = "billing.read"
	BillingRefund       Permission = "billing.refund"      // forward-looking (Phase C)
	PluginsRead         Permission = "plugins.read"
	PluginsManage       Permission = "plugins.manage"
	AnalyticsRead       Permission = "analytics.read"
	SessionsRead        Permission = "sessions.read"
	SettingsManage      Permission = "settings.manage"
	InfraRead           Permission = "infra.read"    // remnawave topology (platform-only)
	InfraManage         Permission = "infra.manage"  // remnawave mutations (platform-only)
	BillingManage       Permission = "billing.manage" // balance/financial ops (platform-only)
)

// Definition is catalog metadata for one permission.
type Definition struct {
	Key         Permission
	Description string
}

// Resource/Action are derived from Key; only the description needs declaring.
func (d Definition) Resource() string { return d.Key.Resource() }
func (d Definition) Action() string   { return d.Key.Action() }

// Catalog returns every permission with its human description. Drives the
// identity.permissions seed and the admin UI list.
func Catalog() []Definition {
	return []Definition{
		{UsersRead, "View platform users"},
		{UsersInvite, "Invite or create users"},
		{UsersAssignRole, "Assign or revoke a user's roles"},
		{RolesRead, "View roles and their permissions"},
		{RolesManage, "Create, edit, or delete custom roles"},
		{ShopsRead, "View shops (tenants)"},
		{ShopsManage, "Create or update shops"},
		{ShopsBranding, "Edit shop branding"},
		{TariffsRead, "View tariffs and pricing"},
		{TariffsWrite, "Create or modify tariffs and pricing"},
		{CustomersRead, "View a shop's customers"},
		{CustomersManage, "Manage a shop's customers"},
		{SubscriptionsRead, "View subscriptions"},
		{SubscriptionsManage, "Manage subscriptions"},
		{BillingRead, "View invoices and billing"},
		{BillingRefund, "Issue refunds"},
		{PluginsRead, "View plugin pages and metadata"},
		{PluginsManage, "Install, enable, configure, or remove plugins"},
		{AnalyticsRead, "View platform analytics and metrics"},
		{SessionsRead, "View active sessions and the activity feed"},
		{SettingsManage, "Change platform settings"},
		{InfraRead, "View Remnawave nodes, panels, and squads"},
		{InfraManage, "Manage Remnawave nodes, panels, and squads (mutations)"},
		{BillingManage, "Manage balances and financial operations (adjust, transfer, export)"},
	}
}

// Scope kinds for roles and bindings.
const (
	ScopeGlobal = "global"
	ScopeShop   = "shop"
)

// System role keys (stable identifiers; seeded, immutable).
const (
	RolePlatformAdmin = "platform_admin"
	RoleShopOwner     = "shop_owner"
	RoleShopStaff     = "shop_staff"
	RoleCustomer      = "customer"
)

// SystemRole defines a seeded, immutable role.
type SystemRole struct {
	Key         string
	Name        string
	Description string
	ScopeKind   string
	Permissions []Permission // empty for platform_admin (allow-all in the resolver)
}

// SystemRoleByKey returns the system role matching key. The second return value
// is false when no system role has that key (e.g. for custom roles).
func SystemRoleByKey(key string) (SystemRole, bool) {
	for _, sr := range SystemRoles() {
		if sr.Key == key {
			return sr, true
		}
	}
	return SystemRole{}, false
}

// SystemRoles returns the built-in roles. platform_admin is intentionally
// allow-all and carries NO explicit permissions (the resolver special-cases it,
// so newly added permissions never drift from a stale join table).
func SystemRoles() []SystemRole {
	return []SystemRole{
		{
			Key: RolePlatformAdmin, Name: "Platform Admin",
			Description: "Full platform access.", ScopeKind: ScopeGlobal,
			Permissions: nil,
		},
		{
			Key: RoleShopOwner, Name: "Shop Owner",
			Description: "Owns a shop; manages its staff, tariffs, and customers.",
			ScopeKind:   ScopeShop,
			Permissions: []Permission{
				ShopsRead, ShopsManage, ShopsBranding,
				TariffsRead, TariffsWrite,
				CustomersRead, CustomersManage,
				SubscriptionsRead, SubscriptionsManage,
				BillingRead, UsersInvite, UsersAssignRole,
				RolesRead, PluginsRead, AnalyticsRead,
			},
		},
		{
			Key: RoleShopStaff, Name: "Shop Staff",
			Description: "Operates a shop; cannot manage staff or settings.",
			ScopeKind:   ScopeShop,
			Permissions: []Permission{
				CustomersRead, CustomersManage,
				SubscriptionsRead, TariffsRead, BillingRead, PluginsRead,
			},
		},
		{
			Key: RoleCustomer, Name: "Customer",
			Description: "End user; no admin/panel permissions.",
			ScopeKind:   ScopeGlobal,
			Permissions: nil,
		},
	}
}
