package service_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// stubShopBotRepo is a minimal in-memory ShopBotRepository for unit tests.
type stubShopBotRepo struct {
	lastUpsert  vo.ShopBotConfig
	upsertCalls int
}

func (s *stubShopBotRepo) Upsert(_ context.Context, _ string, cfg vo.ShopBotConfig) error {
	s.upsertCalls++
	s.lastUpsert = cfg
	return nil
}

func (s *stubShopBotRepo) GetByTenant(_ context.Context, _ string) (*vo.ShopBotConfig, error) {
	return nil, service.ErrShopBotNotFound
}

func (s *stubShopBotRepo) ListEnabled(_ context.Context) ([]vo.ShopBotWithTenant, error) {
	return nil, nil
}

// noopPublisher discards all domain events (shop-bot methods do not publish).
type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ domainevent.Event) error      { return nil }
func (noopPublisher) PublishBatch(_ context.Context, _ []domainevent.Event) error { return nil }

// stubBotPluginValidator is a controllable BotPluginValidator for unit tests.
// Set valid to control IsValidBotPlugin's return; LastSlug records the last
// slug passed in.
type stubBotPluginValidator struct {
	valid    bool
	lastSlug string
}

func (s *stubBotPluginValidator) IsValidBotPlugin(_ context.Context, slug string) (bool, error) {
	s.lastSlug = slug
	return s.valid, nil
}

// newServiceWithStubBots constructs a ResellerService wired with a stub
// ShopBotRepository so that shop-bot unit tests remain database-free.
// It uses AllowAllBotPlugins as the BotPluginValidator, so existing tests are
// unaffected. Tests that need to control plugin validation should use
// newResellerServiceWithStubs instead.
func newServiceWithStubBots(t *testing.T) (*service.ResellerService, *stubShopBotRepo) {
	t.Helper()
	repo := &stubShopBotRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewResellerService(
		nil, nil, nil,
		noopPublisher{},
		logger,
		clock.NewReal(),
		txmanagertest.NoopTxRunner{},
		repo,
		service.AllowAllBotPlugins{},
	)
	return svc, repo
}

// newResellerServiceWithStubs constructs a ResellerService wired with both a
// stub ShopBotRepository and a stub BotPluginValidator. Use this helper in
// tests that need to control plugin slug validation.
func newResellerServiceWithStubs(t *testing.T) (*service.ResellerService, *stubShopBotRepo, *stubBotPluginValidator) {
	t.Helper()
	repo := &stubShopBotRepo{}
	validator := &stubBotPluginValidator{}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewResellerService(
		nil, nil, nil,
		noopPublisher{},
		logger,
		clock.NewReal(),
		txmanagertest.NoopTxRunner{},
		repo,
		validator,
	)
	return svc, repo, validator
}

// withPlugin returns a copy of in with BotPlugin set to slug.
func withPlugin(in service.SetShopBotInput, slug string) service.SetShopBotInput {
	in.BotPlugin = slug
	return in
}

func TestSetShopBot_Validation(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceWithStubBots(t)

	// bad token — must be rejected
	require.Error(t, svc.SetShopBot(ctx, "t1", service.SetShopBotInput{BotToken: "nope", CabinetURL: "https://c", Enabled: true}))

	// non-https cabinet URL — must be rejected
	require.Error(t, svc.SetShopBot(ctx, "t1", service.SetShopBotInput{BotToken: "123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", CabinetURL: "http://c", Enabled: true}))

	// valid input → upsert called, webhook_secret generated (non-empty)
	require.NoError(t, svc.SetShopBot(ctx, "t1", service.SetShopBotInput{BotToken: "123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", CabinetURL: "https://c", Enabled: true}))
	require.NotEmpty(t, repo.lastUpsert.WebhookSecret)
	require.Equal(t, "123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", repo.lastUpsert.Token.Expose())
}

func TestSetShopBot_BotPluginValidation(t *testing.T) {
	ctx := context.Background()
	svc, repo, validator := newResellerServiceWithStubs(t)
	base := service.SetShopBotInput{
		BotToken:   "123:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		CabinetURL: "https://c.example.com",
		Enabled:    true,
	}

	// unknown/disabled plugin slug → ErrShopBotInvalidPlugin, no upsert
	validator.valid = false
	err := svc.SetShopBot(ctx, "t-1", withPlugin(base, "ghost-bot"))
	require.ErrorIs(t, err, service.ErrShopBotInvalidPlugin)
	require.Equal(t, 0, repo.upsertCalls)

	// valid plugin slug → upsert with BotPluginSlug set
	validator.valid = true
	require.NoError(t, svc.SetShopBot(ctx, "t-1", withPlugin(base, "cabinet-bot")))
	require.Equal(t, "cabinet-bot", repo.lastUpsert.BotPluginSlug)

	// empty plugin slug → default behaviour, upsert with empty BotPluginSlug
	require.NoError(t, svc.SetShopBot(ctx, "t-1", base))
	require.Equal(t, "", repo.lastUpsert.BotPluginSlug)
}
