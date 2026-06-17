package rbac

import "errors"

// ErrGrantNotAllowed is returned when an actor may not grant the requested role
// in the requested scope.
var ErrGrantNotAllowed = errors.New("rbac: grant not allowed")

// Actor is the minimal projection of a granting principal's resolved access.
// (Constructed from service.EffectiveAccess by the caller, so this package has
// no dependency on the service layer.)
type Actor struct {
	IsPlatformAdmin bool
	Permissions     map[Permission]struct{}
	AllowedTenants  map[string]struct{}
}

// GrantTarget describes the role being granted.
type GrantTarget struct {
	RoleKey     string // system or custom role key (never empty)
	ScopeKind   string // ScopeGlobal | ScopeShop
	Permissions []Permission
}

// CanGrant reports whether actor may assign target scoped to scopeTenantID
// (nil = global). Pure; no I/O. Pairs with ValidateBinding (binding integrity).
func CanGrant(actor Actor, target GrantTarget, scopeTenantID *string) error {
	if actor.IsPlatformAdmin {
		return nil
	}
	// Rule 2: a non-admin must independently hold the grant-authorization
	// permission (defense-in-depth; the HTTP route gate also enforces this).
	if _, ok := actor.Permissions[UsersAssignRole]; !ok {
		return ErrGrantNotAllowed
	}
	// Non-admins may only grant shop-scoped roles within a shop they belong to.
	if target.ScopeKind != ScopeShop {
		return ErrGrantNotAllowed
	}
	// Non-admins may never create platform admins or shop owners.
	if target.RoleKey == RolePlatformAdmin || target.RoleKey == RoleShopOwner {
		return ErrGrantNotAllowed
	}
	if scopeTenantID == nil {
		return ErrGrantNotAllowed
	}
	if _, ok := actor.AllowedTenants[*scopeTenantID]; !ok {
		return ErrGrantNotAllowed
	}
	// No escalation: every permission of the target must be held by the actor.
	for _, p := range target.Permissions {
		if _, ok := actor.Permissions[p]; !ok {
			return ErrGrantNotAllowed
		}
	}
	return nil
}
