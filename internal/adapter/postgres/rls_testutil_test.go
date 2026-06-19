//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	// Shared across all postgres_test RLS integration tests (declared once here).
	testCollectionPlugin = "tariff-manager"
	testCollectionName   = "tariffs"

	testRLSRole     = "rls_app"
	testRLSRolePass = "rls_app_pw"
)

// connectAsRLSApp creates a NON-superuser, NON-BYPASSRLS role and returns a pool
// connected as that role. It is the SINGLE shared helper for exercising FORCE
// ROW LEVEL SECURITY in postgres_test (C2/C3/C4 all call it; none redefine it).
//
// connStr is the testcontainers admin DSN; the returned pool reuses admin's
// connection config but switches the role to a freshly granted rls_app. The
// helper FAILS (t.Fatal) — never skips — if the connecting role can bypass RLS
// (current_setting('is_superuser')='on' OR pg_roles.rolbypassrls), because a
// bypassing role would make every RLS assertion pass vacuously.
func connectAsRLSApp(t *testing.T, admin *pgxpool.Pool, connStr string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	// The role may already exist from a prior helper call in the same DB.
	stmts := []string{
		fmt.Sprintf("DROP ROLE IF EXISTS %s", testRLSRole),
		fmt.Sprintf("CREATE ROLE %s NOSUPERUSER NOBYPASSRLS LOGIN PASSWORD '%s'", testRLSRole, testRLSRolePass),
		fmt.Sprintf("GRANT USAGE ON SCHEMA plugins TO %s", testRLSRole),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA plugins TO %s", testRLSRole),
	}
	for _, s := range stmts {
		_, err := admin.Exec(ctx, s)
		require.NoError(t, err)
	}

	cfg, err := pgxpool.ParseConfig(connStr)
	require.NoError(t, err)
	cfg.ConnConfig.User = testRLSRole
	cfg.ConnConfig.Password = testRLSRolePass
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	// Guard: refuse to run if this role can bypass RLS — otherwise the whole
	// suite would pass vacuously. Fail hard, do not skip.
	var isSuper, bypassRLS bool
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT current_setting('is_superuser')::bool,
		        (SELECT rolbypassrls FROM pg_roles WHERE rolname = current_user)`,
	).Scan(&isSuper, &bypassRLS))
	if isSuper || bypassRLS {
		t.Fatalf("connectAsRLSApp: role %q bypasses RLS (is_superuser=%v rolbypassrls=%v); RLS tests would pass vacuously",
			testRLSRole, isSuper, bypassRLS)
	}
	return pool
}
