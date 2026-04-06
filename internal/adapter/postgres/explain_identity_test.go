//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

const (
	// identityEmailExprIndex is the expression index on lower(email) created
	// by migration 011 after dropping the stored generated column email_lower.
	identityEmailExprIndex = "idx_users_email_lower"

	// identitySessionRefreshTokenIndex is the unique index on refresh_token
	// created by migration 001.
	identitySessionRefreshTokenIndex = "idx_sessions_refresh_token"

	// seedIdentityUserCount is the number of test users inserted to
	// encourage the planner to prefer index scans over sequential scans.
	seedIdentityUserCount = 300

	// seedIdentitySessionCount is the number of test sessions inserted to
	// encourage the planner to prefer index scans over sequential scans.
	seedIdentitySessionCount = 300
)

// seedIdentityUsers inserts n platform users with unique emails. Returns
// the email of the last inserted user for querying.
func seedIdentityUsers(t *testing.T, pool *pgxpool.Pool, n int) (lastEmail string) {
	t.Helper()
	ctx := context.Background()

	for i := range n {
		userID := uuid.Must(uuid.NewV7()).String()
		email := fmt.Sprintf("user%d@example.com", i)

		_, err := pool.Exec(ctx, `
			INSERT INTO identity.platform_users (
				id, email, password_hash, role
			) VALUES ($1, $2, 'hash_placeholder', 'customer')
		`, userID, email)
		require.NoError(t, err, "failed to seed user %d", i)

		lastEmail = email
	}

	_, err := pool.Exec(ctx, "ANALYZE identity.platform_users")
	require.NoError(t, err, "failed to ANALYZE identity.platform_users")

	return lastEmail
}

// seedIdentitySessions inserts n sessions across n users. Returns the
// refresh_token of the last inserted session for querying.
func seedIdentitySessions(t *testing.T, pool *pgxpool.Pool, n int) (lastRefreshToken string) {
	t.Helper()
	ctx := context.Background()

	for i := range n {
		userID := uuid.Must(uuid.NewV7()).String()
		sessionID := uuid.Must(uuid.NewV7()).String()
		refreshToken := fmt.Sprintf("rt_%s_%d", uuid.Must(uuid.NewV7()).String(), i)

		_, err := pool.Exec(ctx, `
			INSERT INTO identity.platform_users (
				id, email, password_hash, role
			) VALUES ($1, $2, 'hash_placeholder', 'customer')
		`, userID, fmt.Sprintf("session_user%d@example.com", i))
		require.NoError(t, err, "failed to seed user for session %d", i)

		_, err = pool.Exec(ctx, `
			INSERT INTO identity.sessions (
				id, user_id, refresh_token, expires_at
			) VALUES ($1, $2, $3, now() + interval '7 days')
		`, sessionID, userID, refreshToken)
		require.NoError(t, err, "failed to seed session %d", i)

		lastRefreshToken = refreshToken
	}

	_, err := pool.Exec(ctx, "ANALYZE identity.sessions")
	require.NoError(t, err, "failed to ANALYZE identity.sessions")

	return lastRefreshToken
}

// TestExplainIdentityEmailLookup verifies that the expression index on
// lower(email) is used for case-insensitive email lookups. This matches
// the GetUserByEmail sqlc query.
func TestExplainIdentityEmailLookup(t *testing.T) {
	pool := setupFullDB(t)
	lastEmail := seedIdentityUsers(t, pool, seedIdentityUserCount)

	emailLookupSQL := `
		SELECT id, email, password_hash, display_name, email_verified,
		       telegram_id, role, tenant_id, created_at, updated_at
		FROM identity.platform_users
		WHERE lower(email) = lower($1)
	`

	plan := ExplainPlan(t, pool, emailLookupSQL, lastEmail)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, identityEmailExprIndex)
	AssertNoSeqScan(t, plan)
}

// TestExplainSessionRefreshToken verifies that the unique index on
// refresh_token is used for session lookups. This matches the
// GetSessionByRefreshToken sqlc query.
func TestExplainSessionRefreshToken(t *testing.T) {
	pool := setupFullDB(t)
	lastRefreshToken := seedIdentitySessions(t, pool, seedIdentitySessionCount)

	sessionLookupSQL := `
		SELECT id, user_id, refresh_token, expires_at, created_at
		FROM identity.sessions
		WHERE refresh_token = $1
	`

	plan := ExplainPlan(t, pool, sessionLookupSQL, lastRefreshToken)

	t.Logf("Query plan:\n%s", plan)

	nodeType, execTime := PlanSummary(t, plan)
	t.Logf("Top node: %s, execution time: %.3fms", nodeType, execTime)

	AssertIndexUsedStrict(t, plan, identitySessionRefreshTokenIndex)
	AssertNoSeqScan(t, plan)
}
