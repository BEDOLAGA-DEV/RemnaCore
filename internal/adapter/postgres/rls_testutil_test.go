//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// allMigrationScripts returns the absolute paths of EVERY migration, sorted by
// filename (== apply order). Harnesses pass these to tcpostgres.WithInitScripts
// so the test schema always matches the current migration chain — hard-coding a
// subset silently rots as new columns/tables land (the cause of the mass
// "column does not exist" integration failures). WithInitScripts runs each file
// via the postgres entrypoint (psql), so CREATE INDEX CONCURRENTLY is fine here
// (unlike a single pool.Exec of a whole file, which wraps it in a transaction).
func allMigrationScripts(t *testing.T) []string {
	t.Helper()
	dir, err := filepath.Abs("migrations")
	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var scripts []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			scripts = append(scripts, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(scripts)
	require.NotEmpty(t, scripts, "no migration scripts found under %s", dir)
	return scripts
}

// failOrSkip aborts a container-backed test when Docker is unavailable. In CI
// (REMNACORE_REQUIRE_INTEGRATION set) it FAILS so the integration suite can
// never go green vacuously by skipping every test; locally it SKIPS so devs
// without Docker are not blocked.
func failOrSkip(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("REMNACORE_REQUIRE_INTEGRATION") != "" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

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
