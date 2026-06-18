//go:build integration

package postgres_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// inlineSetUpdatedAt is a sentinel passed to setupTestDBWith in place of a
// migration filename. It applies a standalone set_updated_at() stub (in BOTH
// the identity and public schemas, mirroring 001 and 019) so that migrations
// whose triggers reference set_updated_at() — e.g. 004 (identity.set_updated_at)
// and 034 (public.set_updated_at) — can be applied in isolation without pulling
// in the full migration chain. The schemas are created if absent so the stub is
// self-contained regardless of which migrations precede it.
const inlineSetUpdatedAt = "__inline_set_updated_at"

// inlineSetUpdatedAtSQL is the body applied for the inlineSetUpdatedAt sentinel.
const inlineSetUpdatedAtSQL = `
CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS plugins;
CREATE OR REPLACE FUNCTION public.set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION identity.set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
`

// setupTestDBWith boots a Postgres testcontainer and applies the given migration
// files (in order) from internal/adapter/postgres/migrations, returning the admin
// pool and its connection string. The inlineSetUpdatedAt sentinel may be passed in
// place of a filename to apply an inline set_updated_at() stub.
//
// It SKIPS only when the container cannot start (Docker unavailable). Any
// migration error is a hard failure (t.Fatal) — never a skip — so a test can
// never pass vacuously against a half-applied schema.
func setupTestDBWith(t *testing.T, files ...string) (*pgxpool.Pool, string) {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(testContainerStartupTimeout),
		),
	)
	if err != nil {
		t.Skipf("skipping integration test: could not start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	migrationPath, err := filepath.Abs("migrations")
	require.NoError(t, err)

	for _, f := range files {
		sql := inlineSetUpdatedAtSQL
		if f != inlineSetUpdatedAt {
			b, readErr := os.ReadFile(filepath.Join(migrationPath, f))
			require.NoError(t, readErr, "read migration %s", f)
			sql = string(b)
		}
		// Migration errors fail the test (never skip): a half-applied schema
		// would let downstream assertions pass vacuously.
		_, execErr := pool.Exec(ctx, sql)
		require.NoError(t, execErr, "apply migration %s", f)
	}

	return pool, connStr
}

func setupRBACTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, _ := setupTestDBWith(t, "001_identity.sql", "006_reseller.sql", "038_rbac.sql")
	return pool
}

// insertLegacyUser inserts a row into identity.platform_users with the given
// legacy role and returns the generated UUID. Column list matches 001_identity.sql's
// NOT-NULL columns: id, email, password_hash, role, email_verified.
func insertLegacyUser(t *testing.T, pool *pgxpool.Pool, email, role string) string {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.platform_users (id, email, password_hash, role, email_verified)
		 VALUES ($1, $2, 'x', $3, true)`, id, email, role)
	require.NoError(t, err)
	return id
}

// roleIDByKey returns the UUID of a system role by its catalog key.
func roleIDByKey(t *testing.T, pool *pgxpool.Pool, key string) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM identity.roles WHERE key = $1`, key).Scan(&id))
	return id
}

func TestRBACRepo_SyncAndResolveBindings(t *testing.T) {
	pool := setupRBACTestDB(t)
	repo := postgres.NewRBACRepository(pool)
	ctx := context.Background()

	// Seed a legacy admin user so backfill has something to do.
	adminID := insertLegacyUser(t, pool, "admin@test.com", "admin")
	custID := insertLegacyUser(t, pool, "cust@test.com", "customer")

	require.NoError(t, repo.SyncCatalog(ctx, rbac.Catalog(), rbac.SystemRoles()))

	// Admin got a global platform_admin binding.
	adminBindings, err := repo.ListBindingsForUser(ctx, adminID)
	require.NoError(t, err)
	require.Len(t, adminBindings, 1)
	assert.Equal(t, rbac.RolePlatformAdmin, adminBindings[0].RoleKey)
	assert.Nil(t, adminBindings[0].TenantID)

	// Customer got a global customer binding.
	custBindings, err := repo.ListBindingsForUser(ctx, custID)
	require.NoError(t, err)
	require.Len(t, custBindings, 1)
	assert.Equal(t, rbac.RoleCustomer, custBindings[0].RoleKey)

	// Sync is idempotent: a second run does not duplicate assignments.
	require.NoError(t, repo.SyncCatalog(ctx, rbac.Catalog(), rbac.SystemRoles()))
	adminBindings2, err := repo.ListBindingsForUser(ctx, adminID)
	require.NoError(t, err)
	assert.Len(t, adminBindings2, 1)

	// shop_owner permissions are resolvable and non-empty.
	ownerRoleID := roleIDByKey(t, pool, rbac.RoleShopOwner)
	perms, err := repo.PermissionsForRoles(ctx, []string{ownerRoleID})
	require.NoError(t, err)
	assert.Contains(t, perms[ownerRoleID], rbac.TariffsWrite)

	// platform_admin is allow-all and MUST carry NO explicit role_permissions rows
	// (single-sourced in code; see spec §5). Guards against drift where SyncCatalog
	// accidentally seeds join rows for it.
	var paPermCount int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM identity.role_permissions
		 WHERE role_id = (SELECT id FROM identity.roles WHERE key = $1)`,
		rbac.RolePlatformAdmin).Scan(&paPermCount))
	assert.Equal(t, int64(0), paPermCount, "platform_admin must have no explicit role_permissions rows")
}
