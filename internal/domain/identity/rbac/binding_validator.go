package rbac

import "errors"

// ErrInvalidBindingScope is returned when a role assignment violates the
// scope/tenant invariant (spec §4.4).
var ErrInvalidBindingScope = errors.New("rbac: invalid binding scope/tenant combination")

// Role is the minimal role shape the validator needs: its scope kind and, for a
// shop-local custom role, the tenant it is pinned to (nil for system/global roles).
type Role struct {
	ScopeKind string  // ScopeGlobal | ScopeShop
	TenantID  *string // non-nil only for shop-local custom roles
}

// ValidateBinding enforces the spec §4.4 invariant for assigning `role` to a
// user scoped to `tenantID` (nil = global binding):
//   - a global-scoped role MUST bind with tenantID == nil;
//   - a shop-scoped role MUST bind with tenantID != nil;
//   - a shop-local custom role (role.TenantID != nil) MUST bind to that same tenant.
//
// Takes the tenant pointer (not the full Binding) so it has no dependency on the
// repository's Binding type, which is defined later in Task 4. Phase B assignment
// endpoints and any backfill that writes shop bindings call this.
func ValidateBinding(role Role, tenantID *string) error {
	switch role.ScopeKind {
	case ScopeGlobal:
		if tenantID != nil {
			return ErrInvalidBindingScope
		}
	case ScopeShop:
		if tenantID == nil {
			return ErrInvalidBindingScope
		}
		if role.TenantID != nil && *role.TenantID != *tenantID {
			return ErrInvalidBindingScope
		}
	default:
		return ErrInvalidBindingScope
	}
	return nil
}
