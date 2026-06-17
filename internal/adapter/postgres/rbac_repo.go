package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/jackc/pgx/v5"
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

// SyncCatalog implements rbac.Repository. Callers MUST provide a transaction via
// context; see rbac.Repository.SyncCatalog for the full transactional contract.
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

// AssignRole grants roleID to userID scoped to tenantID (nil = global). Idempotent.
func (r *RBACRepository) AssignRole(ctx context.Context, userID, roleID string, tenantID *string, grantedBy string) error {
	if tenantID == nil {
		err := r.q(ctx).InsertGlobalRoleAssignment(ctx, gen.InsertGlobalRoleAssignmentParams{
			UserID:    pgutil.UUIDToPgtype(userID),
			RoleID:    pgutil.UUIDToPgtype(roleID),
			GrantedBy: pgutil.UUIDToPgtype(grantedBy),
		})
		if err != nil {
			return fmt.Errorf("assign global role: %w", err)
		}
		return nil
	}
	err := r.q(ctx).InsertShopRoleAssignment(ctx, gen.InsertShopRoleAssignmentParams{
		UserID:    pgutil.UUIDToPgtype(userID),
		RoleID:    pgutil.UUIDToPgtype(roleID),
		TenantID:  pgutil.UUIDToPgtype(*tenantID),
		GrantedBy: pgutil.UUIDToPgtype(grantedBy),
	})
	if err != nil {
		return fmt.Errorf("assign shop role: %w", err)
	}
	return nil
}

// RevokeRole removes the (userID, roleID, tenantID) binding. Returns count removed.
func (r *RBACRepository) RevokeRole(ctx context.Context, userID, roleID string, tenantID *string) (int64, error) {
	if tenantID == nil {
		n, err := r.q(ctx).DeleteGlobalRoleAssignment(ctx, gen.DeleteGlobalRoleAssignmentParams{
			UserID: pgutil.UUIDToPgtype(userID),
			RoleID: pgutil.UUIDToPgtype(roleID),
		})
		if err != nil {
			return 0, fmt.Errorf("revoke global role: %w", err)
		}
		return n, nil
	}
	n, err := r.q(ctx).DeleteShopRoleAssignment(ctx, gen.DeleteShopRoleAssignmentParams{
		UserID:   pgutil.UUIDToPgtype(userID),
		RoleID:   pgutil.UUIDToPgtype(roleID),
		TenantID: pgutil.UUIDToPgtype(*tenantID),
	})
	if err != nil {
		return 0, fmt.Errorf("revoke shop role: %w", err)
	}
	return n, nil
}

// GetRole returns the role identified by key. Returns rbac.ErrRoleNotFound when missing.
func (r *RBACRepository) GetRole(ctx context.Context, key string) (rbac.Role, error) {
	row, err := r.q(ctx).GetRoleByKey(ctx, pgutil.StrPtrOrNil(key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rbac.Role{}, fmt.Errorf("get role %q: %w", key, rbac.ErrRoleNotFound)
		}
		return rbac.Role{}, fmt.Errorf("get role %q: %w", key, err)
	}
	return rbac.Role{
		ID:        pgutil.PgtypeToUUID(row.ID),
		Key:       pgutil.DerefStr(row.Key),
		ScopeKind: row.ScopeKind,
		TenantID:  pgutil.PgtypeUUIDToOptStr(row.TenantID),
	}, nil
}

// CountPlatformAdmins returns the number of distinct global platform_admin holders.
func (r *RBACRepository) CountPlatformAdmins(ctx context.Context) (int, error) {
	count, err := r.q(ctx).CountPlatformAdmins(ctx)
	if err != nil {
		return 0, fmt.Errorf("count platform admins: %w", err)
	}
	return int(count), nil
}

// Compile-time check that the adapter satisfies the port.
var _ rbac.Repository = (*RBACRepository)(nil)
