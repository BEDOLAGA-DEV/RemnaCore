package rbac

import (
	"context"
	"errors"
)

// ErrRoleNotFound is returned when a role lookup by key yields no result.
var ErrRoleNotFound = errors.New("rbac: role not found")

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

// CustomRole is a tenant-defined (is_system=false) role with its permission set.
// Custom roles carry no key (the DB key column is NULL); they are referenced by ID.
type CustomRole struct {
	ID          string
	Name        string
	Description string
	ScopeKind   string  // ScopeGlobal | ScopeShop
	TenantID    *string // non-nil only for shop-scoped custom roles
	Permissions []Permission
}

// Repository is the persistence port for RBAC reads and catalog sync.
type Repository interface {
	// ListBindingsForUser returns every role assignment for the user.
	ListBindingsForUser(ctx context.Context, userID string) ([]Binding, error)
	// PermissionsForRoles returns the set of permission keys for the given roles.
	PermissionsForRoles(ctx context.Context, roleIDs []string) (map[string][]Permission, error)
	// SyncCatalog idempotently upserts permissions, system roles, and their
	// permission rows, then backfills global assignments from legacy roles.
	//
	// Transactional contract: callers MUST invoke SyncCatalog within a
	// transaction. For each role the method deletes all existing permission rows
	// (ReplaceRolePermissions) and then re-inserts them (AddRolePermission). A
	// non-transactional call that fails between the delete and the inserts will
	// leave that role with no permissions. RBACCatalogSync.Run satisfies this
	// requirement by wrapping the call in RunInTx.
	SyncCatalog(ctx context.Context, perms []Definition, roles []SystemRole) error

	// AssignRole grants roleID to userID scoped to tenantID (nil = global). Idempotent.
	AssignRole(ctx context.Context, userID, roleID string, tenantID *string, grantedBy string) error
	// RevokeRole removes the (userID, roleID, tenantID) binding. Returns count removed.
	RevokeRole(ctx context.Context, userID, roleID string, tenantID *string) (int64, error)
	// GetRole returns the role identified by key. Returns ErrRoleNotFound when missing.
	GetRole(ctx context.Context, key string) (Role, error)
	// CountPlatformAdmins returns the number of distinct global platform_admin holders.
	CountPlatformAdmins(ctx context.Context) (int, error)

	// --- Custom-role CRUD (Phase D) ---

	// CreateCustomRole inserts a non-system role and its permission rows, returning
	// the new role ID. Callers MUST wrap this in a transaction (role + permission
	// rows must be atomic).
	CreateCustomRole(ctx context.Context, name, description, scopeKind string, tenantID *string, perms []Permission) (string, error)
	// ListCustomRoles returns non-system roles for the given scope: tenantID nil
	// selects global custom roles; non-nil selects that tenant's.
	ListCustomRoles(ctx context.Context, tenantID *string) ([]CustomRole, error)
	// GetCustomRole returns the non-system role by ID. Returns ErrRoleNotFound when
	// missing or when the ID names a system role.
	GetCustomRole(ctx context.Context, roleID string) (CustomRole, error)
	// DeleteCustomRole removes a non-system role by ID (permission rows and
	// assignments cascade). Returns the number of rows removed.
	DeleteCustomRole(ctx context.Context, roleID string) (int64, error)
}
