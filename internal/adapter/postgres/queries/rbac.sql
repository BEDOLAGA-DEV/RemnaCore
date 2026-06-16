-- name: UpsertPermission :exec
INSERT INTO identity.permissions (key, resource, action, description)
VALUES ($1, $2, $3, $4)
ON CONFLICT (key) DO UPDATE
SET resource = EXCLUDED.resource, action = EXCLUDED.action, description = EXCLUDED.description;

-- name: UpsertSystemRole :one
INSERT INTO identity.roles (key, name, description, is_system, scope_kind)
VALUES ($1, $2, $3, true, $4)
ON CONFLICT (key) DO UPDATE
SET name = EXCLUDED.name, description = EXCLUDED.description, scope_kind = EXCLUDED.scope_kind, updated_at = now()
RETURNING id;

-- name: ReplaceRolePermissions :exec
-- Deletes then is followed by AddRolePermission calls in the same tx.
DELETE FROM identity.role_permissions WHERE role_id = $1;

-- name: AddRolePermission :exec
INSERT INTO identity.role_permissions (role_id, permission_key)
VALUES ($1, $2)
ON CONFLICT (role_id, permission_key) DO NOTHING;

-- name: GetRoleIDByKey :one
SELECT id FROM identity.roles WHERE key = $1;

-- name: BackfillGlobalAssignment :exec
-- Idempotently assigns a global (NULL-tenant) role to every user whose legacy
-- platform_users.role maps to roleKey and who has no global binding yet.
-- Positional params are aliased with sqlc.arg() so the generated struct fields
-- are deterministic (RoleID, LegacyRole) instead of fragile Column1/Role.
INSERT INTO identity.role_assignments (user_id, role_id)
SELECT u.id, sqlc.arg(role_id)::uuid
FROM identity.platform_users u
WHERE u.role = sqlc.arg(legacy_role)
  AND NOT EXISTS (
    SELECT 1 FROM identity.role_assignments ra
    WHERE ra.user_id = u.id AND ra.tenant_id IS NULL
  )
-- The (user_id, role_id) target matches the partial unique index
-- idx_role_assignments_global (WHERE tenant_id IS NULL); "ON CONFLICT DO NOTHING"
-- with no target is invalid Postgres and must name the index columns.
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: ListAssignmentsForUser :many
SELECT ra.role_id, ra.tenant_id, r.key, r.scope_kind
FROM identity.role_assignments ra
JOIN identity.roles r ON r.id = ra.role_id
WHERE ra.user_id = $1;

-- name: ListPermissionsForRoles :many
SELECT role_id, permission_key
FROM identity.role_permissions
WHERE role_id = ANY($1::uuid[]);
