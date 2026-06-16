package postgres

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RBACRepository implements rbac.Repository backed by PostgreSQL.
type RBACRepository struct {
	pool *pgxpool.Pool
}

// NewRBACRepository returns an RBACRepository using the given pool.
func NewRBACRepository(pool *pgxpool.Pool) *RBACRepository {
	return &RBACRepository{pool: pool}
}

func (r *RBACRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

func (r *RBACRepository) ListBindingsForUser(ctx context.Context, userID string) ([]rbac.Binding, error) {
	rows, err := r.q(ctx).ListAssignmentsForUser(ctx, pgutil.UUIDToPgtype(userID))
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	out := make([]rbac.Binding, 0, len(rows))
	for _, row := range rows {
		out = append(out, rbac.Binding{
			RoleID:    pgutil.PgtypeToUUID(row.RoleID),
			RoleKey:   pgutil.DerefStr(row.Key),
			ScopeKind: row.ScopeKind,
			TenantID:  pgutil.PgtypeUUIDToOptStr(row.TenantID),
		})
	}
	return out, nil
}

func (r *RBACRepository) PermissionsForRoles(ctx context.Context, roleIDs []string) (map[string][]rbac.Permission, error) {
	pgIDs := make([]pgtype.UUID, len(roleIDs))
	for i, id := range roleIDs {
		pgIDs[i] = pgutil.UUIDToPgtype(id)
	}
	rows, err := r.q(ctx).ListPermissionsForRoles(ctx, pgIDs)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	out := map[string][]rbac.Permission{}
	for _, row := range rows {
		rid := pgutil.PgtypeToUUID(row.RoleID)
		out[rid] = append(out[rid], rbac.Permission(row.PermissionKey))
	}
	return out, nil
}

// legacyRoleForSystemRole maps a system role key to the legacy
// platform_users.role string it backfills from. Returns "" for roles with no
// legacy source (reseller users get no binding in Phase A; intent is preserved
// in the deprecated platform_users.role column until Phase B/C).
func legacyRoleForSystemRole(key string) string {
	switch key {
	case rbac.RolePlatformAdmin:
		return "admin"
	case rbac.RoleCustomer:
		return "customer"
	default:
		return ""
	}
}

func (r *RBACRepository) SyncCatalog(ctx context.Context, perms []rbac.Definition, roles []rbac.SystemRole) error {
	q := r.q(ctx)
	for _, d := range perms {
		if err := q.UpsertPermission(ctx, gen.UpsertPermissionParams{
			Key:         string(d.Key),
			Resource:    d.Resource(),
			Action:      d.Action(),
			Description: d.Description,
		}); err != nil {
			return fmt.Errorf("upsert permission %s: %w", d.Key, err)
		}
	}
	for _, role := range roles {
		roleID, err := q.UpsertSystemRole(ctx, gen.UpsertSystemRoleParams{
			Key:         pgutil.StrPtrOrNil(role.Key),
			Name:        role.Name,
			Description: role.Description,
			ScopeKind:   role.ScopeKind,
		})
		if err != nil {
			return fmt.Errorf("upsert role %s: %w", role.Key, err)
		}
		if err := q.ReplaceRolePermissions(ctx, roleID); err != nil {
			return fmt.Errorf("clear role perms %s: %w", role.Key, err)
		}
		for _, p := range role.Permissions {
			if err := q.AddRolePermission(ctx, gen.AddRolePermissionParams{
				RoleID:        roleID,
				PermissionKey: string(p),
			}); err != nil {
				return fmt.Errorf("add role perm %s/%s: %w", role.Key, p, err)
			}
		}
		if legacy := legacyRoleForSystemRole(role.Key); legacy != "" {
			if err := q.BackfillGlobalAssignment(ctx, gen.BackfillGlobalAssignmentParams{
				RoleID:     roleID,
				LegacyRole: legacy,
			}); err != nil {
				return fmt.Errorf("backfill %s: %w", role.Key, err)
			}
		}
	}
	return nil
}

// Compile-time check that the adapter satisfies the port.
var _ rbac.Repository = (*RBACRepository)(nil)
