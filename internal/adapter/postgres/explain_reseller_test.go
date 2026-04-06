//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	// resellerCommissionResellerStatusIndex is the composite index on
	// (reseller_id, status) created by migration 011 and recreated
	// with CONCURRENTLY in migration 017.
	resellerCommissionResellerStatusIndex = "idx_commissions_reseller_status"

	// resellerTenantDomainIndex is the unique partial index on domain
	// WHERE domain IS NOT NULL created by migration 006.
	resellerTenantDomainIndex = "idx_tenant_domain"

	// seedCommissionCount is the number of test commissions inserted to
	// encourage the planner to prefer index scans over sequential scans.
	seedCommissionCount = 300

	// testCommissionPendingStatus is the status used for seeded commissions
	// that should be returned by the pending commissions query.
	testCommissionPendingStatus = "pending"

	// testCommissionCurrency is the currency used for seeded commissions.
	testCommissionCurrency = "usd"

	// testCommissionAmount is the amount (in cents) used for seeded commissions.
	testCommissionAmount = 500
)

// seedResellerCommissions creates a tenant, reseller account, and n commissions.
// Returns the reseller account ID of the last seeded reseller.
func seedResellerCommissions(t *testing.T, pool *pgxpool.Pool, n int) (lastResellerID string) {
	t.Helper()
	ctx := context.Background()

	// Create a single tenant for all reseller accounts.
	tenantID := uuid.Must(uuid.NewV7()).String()
	ownerID := uuid.Must(uuid.NewV7()).String()

	_, err := pool.Exec(ctx, `
		INSERT INTO identity.platform_users (
			id, email, password_hash, role
		) VALUES ($1, $2, 'hash_placeholder', 'reseller')
	`, ownerID, fmt.Sprintf("owner_%s@example.com", tenantID))
	require.NoError(t, err, "failed to seed tenant owner")

	_, err = pool.Exec(ctx, `
		INSERT INTO reseller.tenants (
			id, name, owner_user_id, is_active
		) VALUES ($1, 'Test Tenant', $2, true)
	`, tenantID, ownerID)
	require.NoError(t, err, "failed to seed tenant")

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range n {
		resellerID := uuid.Must(uuid.NewV7()).String()
		userID := uuid.Must(uuid.NewV7()).String()
		commissionID := uuid.Must(uuid.NewV7()).String()

		_, err := pool.Exec(ctx, `
			INSERT INTO identity.platform_users (
				id, email, password_hash, role
			) VALUES ($1, $2, 'hash_placeholder', 'reseller')
		`, userID, fmt.Sprintf("reseller%d@example.com", i))
		require.NoError(t, err, "failed to seed reseller user %d", i)

		_, err = pool.Exec(ctx, `
			INSERT INTO reseller.reseller_accounts (
				id, tenant_id, user_id, commission_rate, balance
			) VALUES ($1, $2, $3, 10, 0)
		`, resellerID, tenantID, userID)
		require.NoError(t, err, "failed to seed reseller account %d", i)

		createdAt := baseTime.Add(time.Duration(i) * time.Hour)
		saleID := fmt.Sprintf("sale_%s", uuid.Must(uuid.NewV7()).String())

		_, err = pool.Exec(ctx, `
			INSERT INTO reseller.commissions (
				id, reseller_id, sale_id, amount, currency, status, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, commissionID, resellerID, saleID, testCommissionAmount, testCommissionCurrency, testCommissionPendingStatus, createdAt)
		require.NoError(t, err, "failed to seed commission %d", i)

		lastResellerID = resellerID
	}

	_, err = pool.Exec(ctx, "ANALYZE reseller.commissions")
	require.NoError(t, err, "failed to ANALYZE reseller.commissions")

	_, err = pool.Exec(ctx, "ANALYZE reseller.reseller_accounts")
	require.NoError(t, err, "failed to ANALYZE reseller.reseller_accounts")

	return lastResellerID
}

// seedResellerTenants creates n tenants with unique domains. Returns the
// domain of the last inserted tenant.
func seedResellerTenants(t *testing.T, pool *pgxpool.Pool, n int) (lastDomain string) {
	t.Helper()
	ctx := context.Background()

	for i := range n {
		tenantID := uuid.Must(uuid.NewV7()).String()
		ownerID := uuid.Must(uuid.NewV7()).String()
		domain := fmt.Sprintf("tenant%d.example.com", i)

		_, err := pool.Exec(ctx, `
			INSERT INTO identity.platform_users (
				id, email, password_hash, role
			) VALUES ($1, $2, 'hash_placeholder', 'reseller')
		`, ownerID, fmt.Sprintf("tenant_owner%d@example.com", i))
		require.NoError(t, err, "failed to seed tenant owner %d", i)

		_, err = pool.Exec(ctx, `
			INSERT INTO reseller.tenants (
				id, name, domain, owner_user_id, is_active
			) VALUES ($1, $2, $3, $4, true)
		`, tenantID, fmt.Sprintf("Tenant %d", i), domain, ownerID)
		require.NoError(t, err, "failed to seed tenant %d", i)

		lastDomain = domain
	}

	_, err := pool.Exec(ctx, "ANALYZE reseller.tenants")
	require.NoError(t, err, "failed to ANALYZE reseller.tenants")

	return lastDomain
}

// TestExplainCommissionResellerStatus verifies that the composite index on
// (reseller_id, status) is used for queries filtering pending commissions
// by reseller. This matches the GetPendingCommissions sqlc query.
func TestExplainCommissionResellerStatus(t *testing.T) {
	pool := setupFullDB(t)
	lastResellerID := seedResellerCommissions(t, pool, seedCommissionCount)

	commissionSQL := `
		SELECT id, reseller_id, sale_id, amount, currency, status, created_at, paid_at
		FROM reseller.commissions
		WHERE reseller_id = $1 AND status = 'pending'
		ORDER BY created_at DESC
	`

	plan := ExplainPlan(t, pool, commissionSQL, lastResellerID)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, resellerCommissionResellerStatusIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainTenantDomainLookup verifies that the unique partial index on
// domain is used for tenant lookups by domain. This matches the
// GetTenantByDomain sqlc query.
func TestExplainTenantDomainLookup(t *testing.T) {
	pool := setupFullDB(t)
	lastDomain := seedResellerTenants(t, pool, seedCommissionCount)

	tenantDomainSQL := `
		SELECT id, name, domain, owner_user_id, branding_config,
		       api_key_hash, is_active, created_at, updated_at
		FROM reseller.tenants
		WHERE domain = $1
	`

	plan := ExplainPlan(t, pool, tenantDomainSQL, lastDomain)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, resellerTenantDomainIndex)
	AssertNoSeqScan(t, plan)
}
