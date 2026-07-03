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
	if key == "" {
		return rbac.Role{}, rbac.ErrRoleNotFound
	}
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

// --- Custom-role CRUD (Phase D) ---

// optUUID builds a pgtype.UUID that is NULL when s is nil.
func optUUID(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return pgutil.UUIDToPgtype(*s)
}

// CreateCustomRole inserts a non-system role plus its permission rows and returns
// the new role ID. Must run inside a transaction (the service wraps it).
func (r *RBACRepository) CreateCustomRole(ctx context.Context, name, description, scopeKind string, tenantID *string, perms []rbac.Permission) (string, error) {
	db := DBFromContext(ctx, r.pool)
	var id pgtype.UUID
	err := db.QueryRow(ctx,
		`INSERT INTO identity.roles (name, description, is_system, scope_kind, tenant_id)
		 VALUES ($1, $2, false, $3, $4) RETURNING id`,
		name, description, scopeKind, optUUID(tenantID),
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert custom role: %w", err)
	}
	roleID := pgutil.PgtypeToUUID(id)
	for _, p := range perms {
		if _, err := db.Exec(ctx,
			`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			id, string(p),
		); err != nil {
			return "", fmt.Errorf("insert custom role permission %q: %w", p, err)
		}
	}
	return roleID, nil
}

// permissionsByRole loads permission keys for the given role IDs, grouped by ID.
func (r *RBACRepository) permissionsByRole(ctx context.Context, roleIDs []pgtype.UUID) (map[string][]rbac.Permission, error) {
	out := make(map[string][]rbac.Permission)
	if len(roleIDs) == 0 {
		return out, nil
	}
	rows, err := DBFromContext(ctx, r.pool).Query(ctx,
		`SELECT role_id, permission_key FROM identity.role_permissions WHERE role_id = ANY($1)`,
		roleIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rid pgtype.UUID
		var key string
		if err := rows.Scan(&rid, &key); err != nil {
			return nil, fmt.Errorf("scan role permission: %w", err)
		}
		id := pgutil.PgtypeToUUID(rid)
		out[id] = append(out[id], rbac.Permission(key))
	}
	return out, rows.Err()
}

// ListCustomRoles returns non-system roles for the given scope.
func (r *RBACRepository) ListCustomRoles(ctx context.Context, tenantID *string) ([]rbac.CustomRole, error) {
	rows, err := DBFromContext(ctx, r.pool).Query(ctx,
		`SELECT id, name, description, scope_kind, tenant_id
		 FROM identity.roles
		 WHERE is_system = false
		   AND (($1::uuid IS NULL AND tenant_id IS NULL) OR tenant_id = $1)
		 ORDER BY name`,
		optUUID(tenantID),
	)
	if err != nil {
		return nil, fmt.Errorf("list custom roles: %w", err)
	}
	defer rows.Close()

	var roles []rbac.CustomRole
	var ids []pgtype.UUID
	for rows.Next() {
		var (
			id       pgtype.UUID
			name     string
			desc     string
			scope    string
			tenantPg pgtype.UUID
		)
		if err := rows.Scan(&id, &name, &desc, &scope, &tenantPg); err != nil {
			return nil, fmt.Errorf("scan custom role: %w", err)
		}
		roles = append(roles, rbac.CustomRole{
			ID:          pgutil.PgtypeToUUID(id),
			Name:        name,
			Description: desc,
			ScopeKind:   scope,
			TenantID:    pgutil.PgtypeUUIDToOptStr(tenantPg),
		})
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	permsByID, err := r.permissionsByRole(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		roles[i].Permissions = permsByID[roles[i].ID]
	}
	return roles, nil
}

// GetCustomRole returns a non-system role by ID.
func (r *RBACRepository) GetCustomRole(ctx context.Context, roleID string) (rbac.CustomRole, error) {
	db := DBFromContext(ctx, r.pool)
	var (
		id       pgtype.UUID
		name     string
		desc     string
		scope    string
		tenantPg pgtype.UUID
	)
	err := db.QueryRow(ctx,
		`SELECT id, name, description, scope_kind, tenant_id
		 FROM identity.roles WHERE id = $1 AND is_system = false`,
		pgutil.UUIDToPgtype(roleID),
	).Scan(&id, &name, &desc, &scope, &tenantPg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rbac.CustomRole{}, fmt.Errorf("get custom role %q: %w", roleID, rbac.ErrRoleNotFound)
		}
		return rbac.CustomRole{}, fmt.Errorf("get custom role %q: %w", roleID, err)
	}
	permsByID, err := r.permissionsByRole(ctx, []pgtype.UUID{id})
	if err != nil {
		return rbac.CustomRole{}, err
	}
	return rbac.CustomRole{
		ID:          pgutil.PgtypeToUUID(id),
		Name:        name,
		Description: desc,
		ScopeKind:   scope,
		TenantID:    pgutil.PgtypeUUIDToOptStr(tenantPg),
		Permissions: permsByID[pgutil.PgtypeToUUID(id)],
	}, nil
}

// UpdateCustomRole replaces a non-system role's name, description, and
// permission set. Must run inside a transaction (the service wraps it).
func (r *RBACRepository) UpdateCustomRole(ctx context.Context, roleID, name, description string, perms []rbac.Permission) error {
	db := DBFromContext(ctx, r.pool)
	tag, err := db.Exec(ctx,
		`UPDATE identity.roles SET name = $2, description = $3, updated_at = now()
		 WHERE id = $1 AND is_system = false`,
		pgutil.UUIDToPgtype(roleID), name, description,
	)
	if err != nil {
		return fmt.Errorf("update custom role %q: %w", roleID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update custom role %q: %w", roleID, rbac.ErrRoleNotFound)
	}
	if _, err := db.Exec(ctx,
		`DELETE FROM identity.role_permissions WHERE role_id = $1`,
		pgutil.UUIDToPgtype(roleID),
	); err != nil {
		return fmt.Errorf("clear custom role permissions %q: %w", roleID, err)
	}
	for _, p := range perms {
		if _, err := db.Exec(ctx,
			`INSERT INTO identity.role_permissions (role_id, permission_key) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING`,
			pgutil.UUIDToPgtype(roleID), string(p),
		); err != nil {
			return fmt.Errorf("insert custom role permission %q: %w", p, err)
		}
	}
	return nil
}

// DeleteCustomRole removes a non-system role by ID. Permission rows and
// assignments cascade via FK. Returns rows removed.
func (r *RBACRepository) DeleteCustomRole(ctx context.Context, roleID string) (int64, error) {
	tag, err := DBFromContext(ctx, r.pool).Exec(ctx,
		`DELETE FROM identity.roles WHERE id = $1 AND is_system = false`,
		pgutil.UUIDToPgtype(roleID),
	)
	if err != nil {
		return 0, fmt.Errorf("delete custom role %q: %w", roleID, err)
	}
	return tag.RowsAffected(), nil
}

// Compile-time check that the adapter satisfies the port.
var _ rbac.Repository = (*RBACRepository)(nil)
