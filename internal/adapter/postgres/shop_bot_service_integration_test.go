//go:build integration

package postgres_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secretbox"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// noopEventPublisher discards all domain events; shop-bot methods don't publish.
type noopEventPublisher struct{}

func (noopEventPublisher) Publish(_ context.Context, _ domainevent.Event) error        { return nil }
func (noopEventPublisher) PublishBatch(_ context.Context, _ []domainevent.Event) error { return nil }

// TestShopBotService_RunInTx_EndToEnd builds a real ResellerService (real
// ShopBotRepository + real TxManager + real secretbox.Box) and asserts that
// SetShopBot followed by GetShopBot succeeds end-to-end against the DB.
//
// This test guards the class of bug where shop-bot methods call repo directly
// (without RunInTx), so the app.tenant_id GUC is never set and RLS rejects the
// write with a WITH CHECK violation or returns 0 rows on read.
func TestShopBotService_RunInTx_EndToEnd(t *testing.T) {
	admin, connStr := setupTestDBWith(t, shopBotMigrations...)
	ctx := context.Background()

	rlsPool := connectAsRLSApp(t, admin, connStr)
	grantShopBotSchemasToRLSApp(t, ctx, admin)

	tenantID := uuid.Must(uuid.NewV7()).String()
	seedShopBotTenant(t, ctx, rlsPool, tenantID)

	testKey := make([]byte, secretbox.KeySize)
	box, err := secretbox.New(testKey)
	require.NoError(t, err)

	tm := postgres.NewTxManager(rlsPool)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	repo := postgres.NewShopBotRepository(rlsPool, box, logger)

	svc := service.NewResellerService(
		nil, nil, nil,
		noopEventPublisher{},
		logger,
		clock.NewReal(),
		tm,
		repo,
	)

	shopCtx := tenantctx.WithTenantID(ctx, tenantID)

	// SetShopBot must succeed — the service's own RunInTx sets the GUC so that
	// RLS WITH CHECK permits the insert under the tenant scope.
	err = svc.SetShopBot(shopCtx, tenantID, service.SetShopBotInput{
		BotToken:   "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZ_test_tok",
		CabinetURL: "https://cabinet.example.com",
		Enabled:    true,
	})
	require.NoError(t, err, "SetShopBot must succeed when service wraps the call in RunInTx")

	// GetShopBot must return the stored (decrypted) config — same RunInTx requirement.
	cfg, err := svc.GetShopBot(shopCtx, tenantID)
	require.NoError(t, err, "GetShopBot must succeed when service wraps the call in RunInTx")
	require.NotNil(t, cfg)
	assert.Equal(t, "123456:ABCDEFGHIJKLMNOPQRSTUVWXYZ_test_tok", cfg.Token.Expose(),
		"decrypted token must match the value passed to SetShopBot")
	assert.Equal(t, "https://cabinet.example.com", cfg.CabinetURL)
	assert.True(t, cfg.Enabled)
	assert.NotEmpty(t, cfg.WebhookSecret, "webhook secret must have been generated")
}
