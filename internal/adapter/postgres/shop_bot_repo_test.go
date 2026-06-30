//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secretbox"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// shopBotMigrations is the minimal migration chain to stand up reseller.shop_bots
// with its RLS policy (045) and the trigger dependency (identity.set_updated_at
// created by 001).
var shopBotMigrations = []string{
	"001_identity.sql",
	"006_reseller.sql",
	"018_row_level_security.sql",
	"045_shop_bots.sql",
}

// grantShopBotSchemasToRLSApp grants the rls_app role (created by the shared
// connectAsRLSApp helper) USAGE + DML on the identity and reseller schemas so
// that the shop-bot integration test can seed and read rows without hitting
// "permission denied for schema" errors. Grants are applied via the admin pool.
func grantShopBotSchemasToRLSApp(t *testing.T, ctx context.Context, admin *pgxpool.Pool) {
	t.Helper()
	for _, schema := range []string{"identity", "reseller"} {
		stmts := []string{
			fmt.Sprintf("GRANT USAGE ON SCHEMA %s TO %s", schema, testRLSRole),
			fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA %s TO %s", schema, testRLSRole),
		}
		for _, s := range stmts {
			_, err := admin.Exec(ctx, s)
			require.NoError(t, err)
		}
	}
}

// seedShopBotTenant inserts a minimal identity.platform_users row and a
// reseller.tenants row for the given tenantID under the platform sentinel GUC.
// It uses the rls_app pool (which already has DML grants) inside a sentinel
// transaction — the same pattern used by TestResellerCommissionsRLS.
func seedShopBotTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	ownerID := uuid.Must(uuid.NewV7()).String()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantctx.PlatformScopeSentinel)
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		`INSERT INTO identity.platform_users (id, email, password_hash, role, email_verified)
		 VALUES ($1, $2, 'h', 'reseller', true)`,
		ownerID, ownerID+"@shopbot-test.io")
	require.NoError(t, err)
	_, err = tx.Exec(ctx,
		`INSERT INTO reseller.tenants (id, name, owner_user_id) VALUES ($1, 'shop', $2)`,
		tenantID, ownerID)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
}

// TestShopBotRepository_RLS verifies the following contracts:
//
//  1. Round-trip: Upsert + GetByTenant returns the decrypted plaintext token.
//  2. Ciphertext-at-rest: the bot_token_enc column is NOT the plaintext token.
//  3. Cross-tenant isolation: shop B's GUC cannot read shop A's row.
//  4. Sentinel list: ListEnabled (platform scope) sees shop A's enabled row.
func TestShopBotRepository_RLS(t *testing.T) {
	admin, connStr := setupTestDBWith(t, shopBotMigrations...)
	ctx := context.Background()

	// Connect as the shared NON-superuser rls_app role (rls_testutil_test.go, C2)
	// so FORCE RLS is genuinely enforced and assertions are not vacuous.
	rlsPool := connectAsRLSApp(t, admin, connStr)
	grantShopBotSchemasToRLSApp(t, ctx, admin)

	shopA := uuid.Must(uuid.NewV7()).String()
	shopB := uuid.Must(uuid.NewV7()).String()

	// Seed shop A tenant (shop B needs no row — it just provides an isolation GUC).
	seedShopBotTenant(t, ctx, rlsPool, shopA)

	// Build a valid Box for encryption. A 32-byte all-zero key is fine in tests.
	testKey := make([]byte, secretbox.KeySize)
	box, err := secretbox.New(testKey)
	require.NoError(t, err)

	tm := postgres.NewTxManager(rlsPool)
	repo := postgres.NewShopBotRepository(rlsPool, box)

	const plainToken = "123456:ABCDEF_test_token"
	cfg := vo.ShopBotConfig{
		Token:         config.NewSecretString(plainToken),
		WebhookSecret: "whsec_test",
		CabinetURL:    "https://cabinet.example.com",
		BotUsername:   "test_bot",
		Enabled:       true,
	}

	// -------------------------------------------------------------------------
	// 1. Upsert under shop A's GUC.
	// -------------------------------------------------------------------------
	upsertCtx := tenantctx.WithTenantID(ctx, shopA)
	require.NoError(t, tm.RunInTx(upsertCtx, func(txCtx context.Context) error {
		return repo.Upsert(txCtx, shopA, cfg)
	}), "Upsert must succeed under shop A GUC")

	// -------------------------------------------------------------------------
	// 2. Round-trip: GetByTenant under shop A returns the plaintext token.
	// -------------------------------------------------------------------------
	getCtx := tenantctx.WithTenantID(ctx, shopA)
	var got *vo.ShopBotConfig
	require.NoError(t, tm.RunInTx(getCtx, func(txCtx context.Context) error {
		var getErr error
		got, getErr = repo.GetByTenant(txCtx, shopA)
		return getErr
	}), "GetByTenant must succeed under shop A GUC")
	require.NotNil(t, got)
	assert.Equal(t, plainToken, got.Token.Expose(), "decrypted token must match plaintext")
	assert.Equal(t, cfg.WebhookSecret, got.WebhookSecret)
	assert.Equal(t, cfg.CabinetURL, got.CabinetURL)
	assert.Equal(t, cfg.BotUsername, got.BotUsername)
	assert.True(t, got.Enabled)

	// -------------------------------------------------------------------------
	// 3. Ciphertext-at-rest: the column value must NOT equal the plaintext token.
	// -------------------------------------------------------------------------
	var storedEnc string
	require.NoError(t, admin.QueryRow(ctx,
		`SELECT bot_token_enc FROM reseller.shop_bots WHERE tenant_id = $1`, shopA,
	).Scan(&storedEnc))
	assert.NotEqual(t, plainToken, storedEnc,
		"bot_token_enc must be ciphertext, not the plaintext token")

	// -------------------------------------------------------------------------
	// 4. Cross-tenant isolation: shop B's GUC cannot read shop A's row.
	// -------------------------------------------------------------------------
	isolCtx := tenantctx.WithTenantID(ctx, shopB)
	var isolErr error
	_ = tm.RunInTx(isolCtx, func(txCtx context.Context) error {
		_, isolErr = repo.GetByTenant(txCtx, shopA)
		return isolErr
	})
	assert.True(t, errors.Is(isolErr, reseller.ErrShopBotNotFound),
		"shop B GUC must not read shop A's row (got: %v)", isolErr)

	// -------------------------------------------------------------------------
	// 5. Sentinel ListEnabled sees shop A.
	// -------------------------------------------------------------------------
	sentinelCtx := tenantctx.WithPlatformScope(ctx)
	var listed []vo.ShopBotWithTenant
	require.NoError(t, tm.RunInTx(sentinelCtx, func(txCtx context.Context) error {
		var listErr error
		listed, listErr = repo.ListEnabled(txCtx)
		return listErr
	}), "ListEnabled must succeed under sentinel GUC")
	require.Len(t, listed, 1, "sentinel must see exactly one enabled bot (shop A)")
	assert.Equal(t, shopA, listed[0].TenantID)
	assert.Equal(t, plainToken, listed[0].Config.Token.Expose(),
		"ListEnabled must return decrypted token")
}

// TestShopBotRepository_NilBox verifies that a nil box returns
// ErrEncryptionNotConfigured on every method without panicking.
func TestShopBotRepository_NilBox(t *testing.T) {
	// Use a minimal DB — we only need the pool for construction; no queries run.
	pool, _ := setupTestDBWith(t, "001_identity.sql")
	repo := postgres.NewShopBotRepository(pool, nil)
	ctx := context.Background()

	cfg := vo.ShopBotConfig{Token: config.NewSecretString("tok")}

	err := repo.Upsert(ctx, uuid.Must(uuid.NewV7()).String(), cfg)
	assert.ErrorIs(t, err, reseller.ErrEncryptionNotConfigured, "Upsert: nil box must return ErrEncryptionNotConfigured")

	_, err = repo.GetByTenant(ctx, uuid.Must(uuid.NewV7()).String())
	assert.ErrorIs(t, err, reseller.ErrEncryptionNotConfigured, "GetByTenant: nil box must return ErrEncryptionNotConfigured")

	_, err = repo.ListEnabled(ctx)
	assert.ErrorIs(t, err, reseller.ErrEncryptionNotConfigured, "ListEnabled: nil box must return ErrEncryptionNotConfigured")
}
