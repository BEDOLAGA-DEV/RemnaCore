//go:build integration

package postgres_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	idservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// grantIdentitySchemaToRLSApp grants the rls_app role (created by connectAsRLSApp)
// USAGE + DML on the identity schema so the telegram-registration integration test
// can read and write platform_users rows without hitting "permission denied for schema".
func grantIdentitySchemaToRLSApp(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	for _, stmt := range []string{
		fmt.Sprintf("GRANT USAGE ON SCHEMA identity TO %s", testRLSRole),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA identity TO %s", testRLSRole),
	} {
		_, err := admin.Exec(ctx, stmt)
		require.NoError(t, err)
	}
}

// newTestJWTIssuer generates a throw-away ECDSA key pair and wraps it in a JWTIssuer.
func newTestJWTIssuer(t *testing.T) *authutil.JWTIssuer {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return authutil.NewJWTIssuer(priv, &priv.PublicKey)
}

// TestRegisterViaTelegram_TenantIsolation asserts three properties against a
// real Postgres instance with FORCE ROW LEVEL SECURITY enforced:
//
//  1. Idempotency: registering the same (telegramID=555, tenantA) twice yields
//     the same user ID — no second INSERT is issued.
//  2. Per-tenant isolation: registering (555, tenantB) produces a DISTINCT user
//     with tenantB as its tenant_id.
//  3. Correctness: each row's tenant_id matches the tenant that registered it.
//
// The test relies on migration 046 (composite unique index on (tenant_id, telegram_id))
// for race-safe find-or-create semantics and on migration 040 (RLS WITH CHECK) to
// reject cross-tenant writes — both present in auditMigrations.
func TestRegisterViaTelegram_TenantIsolation(t *testing.T) {
	admin, connStr := setupTestDBWith(t, auditMigrations...)
	ctx := context.Background()

	// Non-superuser, non-BYPASSRLS pool; RLS is enforced on every query.
	rlsPool := connectAsRLSApp(t, admin, connStr)
	grantIdentitySchemaToRLSApp(t, ctx, admin)

	// Wire up a real Service with the RLS-restricted pool.
	tm := postgres.NewTxManager(rlsPool)
	repo := postgres.NewIdentityRepository(rlsPool)
	pub := noopEventPublisher{}
	jwtIssuer := newTestJWTIssuer(t)
	clk := clock.NewReal()
	sessions := idservice.NewSessionIssuer(repo, pub, jwtIssuer, 15*time.Minute, 7*24*time.Hour)
	svc := idservice.NewService(repo, pub, tm, jwtIssuer, clk, 15*time.Minute, 7*24*time.Hour, sessions)

	// Use stable UUID-format tenant IDs so they satisfy the UUID column type.
	tenantA := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	tenantB := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	const telegramID = int64(555)

	// ── First registration: creates the user. ────────────────────────────────
	u1, err := svc.RegisterViaTelegram(ctx, telegramID, tenantA, "Alice")
	require.NoError(t, err, "first RegisterViaTelegram must succeed")
	require.NotNil(t, u1)
	assert.Equal(t, telegramID, *u1.TelegramID)
	assert.Equal(t, tenantA, *u1.TenantID)

	// ── Second registration: idempotent — returns the same user. ─────────────
	u2, err := svc.RegisterViaTelegram(ctx, telegramID, tenantA, "Alice")
	require.NoError(t, err, "second RegisterViaTelegram must be idempotent")
	require.NotNil(t, u2)
	assert.Equal(t, u1.ID, u2.ID, "repeated call must return the same user ID")

	// ── Different tenant: distinct user for the same telegram_id. ────────────
	u3, err := svc.RegisterViaTelegram(ctx, telegramID, tenantB, "Alice")
	require.NoError(t, err, "RegisterViaTelegram for tenantB must succeed")
	require.NotNil(t, u3)
	assert.NotEqual(t, u1.ID, u3.ID, "different tenant must produce a distinct user")
	assert.Equal(t, tenantB, *u3.TenantID, "user must carry tenantB as its tenant_id")
}
