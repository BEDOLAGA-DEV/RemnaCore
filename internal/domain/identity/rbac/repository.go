package rbac

import "context"

// Binding is one resolved role assignment for a user.
//
// Type distinctions within this package:
//   - Binding: a resolved role assignment for a user (this type).
//   - Role (binding_validator.go): the minimal role shape for scope validation —
//     a role's scope projection used by ValidateBinding.
//   - SystemRole (catalog.go): a seeded, immutable built-in role definition
//     including its name, description, and permission set.
type Binding struct {
	RoleID    string
	RoleKey   string // "" for custom roles
	ScopeKind string // ScopeGlobal | ScopeShop
	TenantID  *string
}

// Repository is the persistence port for RBAC reads and catalog sync.
type Repository interface {
	// ListBindingsForUser returns every role assignment for the user.
	ListBindingsForUser(ctx context.Context, userID string) ([]Binding, error)
	// PermissionsForRoles returns the set of permission keys for the given roles.
	PermissionsForRoles(ctx context.Context, roleIDs []string) (map[string][]Permission, error)
	// SyncCatalog idempotently upserts permissions, system roles, and their
	// permission rows, then backfills global assignments from legacy roles.
	SyncCatalog(ctx context.Context, perms []Definition, roles []SystemRole) error
}
