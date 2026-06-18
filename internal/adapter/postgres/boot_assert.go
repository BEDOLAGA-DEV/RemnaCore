package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// bootRoleAssertionQuery returns TRUE when the current runtime role is a
// superuser OR has effective (including inherited) BYPASSRLS — either of which
// silently bypasses FORCE ROW LEVEL SECURITY and voids tenant isolation.
const bootRoleAssertionQuery = `SELECT current_setting('is_superuser') = 'on'
    OR bool_or(rolbypassrls) FROM pg_roles WHERE pg_has_role(current_user, oid, 'member')`

// pgxRow is the minimal subset of pgx.Row used by the boot assertion, kept as
// an interface so the assertion is unit-testable without a real database.
type pgxRow interface {
	Scan(dest ...any) error
}

// pgxQuerier is the minimal querier surface needed by the boot assertion.
// *pgxpool.Pool satisfies it.
type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgxRow
}

// assertNonBypassRLSRole fails boot when the runtime role bypasses RLS. The
// platform deploys a NOSUPERUSER, NOBYPASSRLS role; this guards against a
// misconfiguration that would void every Phase-C isolation policy.
func assertNonBypassRLSRole(ctx context.Context, q pgxQuerier) error {
	var bypasses bool
	if err := q.QueryRow(ctx, bootRoleAssertionQuery).Scan(&bypasses); err != nil {
		return fmt.Errorf("boot assertion: query runtime role privileges: %w", err)
	}
	if bypasses {
		return fmt.Errorf("boot assertion failed: runtime DB role is superuser or has BYPASSRLS — FORCE ROW LEVEL SECURITY would be bypassed, voiding tenant isolation; deploy a NOSUPERUSER NOBYPASSRLS role")
	}
	return nil
}

// poolQuerier adapts *pgxpool.Pool to pgxQuerier; pgx.Row satisfies pgxRow.
type poolQuerier struct{ pool *pgxpool.Pool }

func (q poolQuerier) QueryRow(ctx context.Context, sql string, args ...any) pgxRow {
	return q.pool.QueryRow(ctx, sql, args...)
}
