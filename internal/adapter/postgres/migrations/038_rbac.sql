-- ============================================================================
-- Migration 038: RBAC core (permissions, roles, role assignments)
-- ============================================================================
-- Permission-based, shop-scoped authorization. This migration creates STRUCTURE
-- ONLY. The permission catalog, system roles, role->permission rows, and the
-- backfill of legacy platform_users.role into role_assignments are applied by
-- an idempotent Go boot sync (service.RBACCatalogSync), so the catalog stays
-- single-sourced in Go (internal/domain/identity/rbac).
--
-- Cross-schema references (user_id, tenant_id) are PLAIN UUID columns enforced
-- by the application, matching the established no-cross-schema-FK convention
-- (cf. balance.wallets). Only within-identity-schema FKs are used.
--
-- These tables are GLOBAL config / cross-tenant resolution data (the resolver
-- must see all of a user's bindings regardless of active tenant), so they carry
-- NO row-level security.
-- ============================================================================

-- Bypass RLS for the duration of this migration so any DML/constraint validation
-- against FORCE-RLS tables (e.g. identity.platform_users, hardened in 018) sees
-- all rows. The session variable resets at transaction end (spec §10). This
-- migration is pure DDL today, but the boot-sync backfill runs the same DML
-- shape; keeping the migration RLS-agnostic is forward-safe.
SET row_security = off;

CREATE TABLE IF NOT EXISTS identity.permissions (
    key         TEXT PRIMARY KEY,
    resource    TEXT NOT NULL,
    action      TEXT NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS identity.roles (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    key         TEXT        UNIQUE,
    name        TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    is_system   BOOLEAN     NOT NULL DEFAULT false,
    scope_kind  TEXT        NOT NULL CHECK (scope_kind IN ('global', 'shop')),
    tenant_id   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT roles_system_is_global
        CHECK (NOT is_system OR (tenant_id IS NULL AND key IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_identity_roles_tenant
    ON identity.roles (tenant_id) WHERE tenant_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS identity.role_permissions (
    role_id        UUID NOT NULL REFERENCES identity.roles(id) ON DELETE CASCADE,
    permission_key TEXT NOT NULL REFERENCES identity.permissions(key) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_key)
);

CREATE TABLE IF NOT EXISTS identity.role_assignments (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id    UUID        NOT NULL,
    role_id    UUID        NOT NULL REFERENCES identity.roles(id) ON DELETE CASCADE,
    tenant_id  UUID,
    granted_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, role_id, tenant_id)
);

-- Postgres treats NULLs as distinct in composite UNIQUE, so the global
-- (NULL-tenant) case needs its own partial unique index (idiom from 006).
CREATE UNIQUE INDEX IF NOT EXISTS idx_role_assignments_global
    ON identity.role_assignments (user_id, role_id) WHERE tenant_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_role_assignments_user
    ON identity.role_assignments (user_id);
CREATE INDEX IF NOT EXISTS idx_role_assignments_user_tenant
    ON identity.role_assignments (user_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_role_assignments_role
    ON identity.role_assignments (role_id);
CREATE INDEX IF NOT EXISTS idx_role_assignments_tenant
    ON identity.role_assignments (tenant_id) WHERE tenant_id IS NOT NULL;
