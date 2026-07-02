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
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/secret"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// stubShopBotRepo is a minimal in-memory ShopBotRepository for unit tests.
// Set storedCfg to simulate an existing persisted config; leave it nil to
// simulate the "no config" case (GetByTenant returns ErrShopBotNotFound).
type stubShopBotRepo struct {
	storedCfg   *vo.ShopBotConfig
	lastUpsert  vo.ShopBotConfig
	upsertCalls int
}

func (s *stubShopBotRepo) Upsert(_ context.Context, _ string, cfg vo.ShopBotConfig) error {
	s.upsertCalls++
	s.lastUpsert = cfg
	return nil
}

func (s *stubShopBotRepo) GetByTenant(_ context.Context, _ string) (*vo.ShopBotConfig, error) {
	if s.storedCfg == nil {
		return nil, service.ErrShopBotNotFound
	}
	return s.storedCfg, nil
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

// storedWebhookSecret is a fixed 64-character hex string used in tests that
// prime the stub with a pre-existing webhook secret.
const storedWebhookSecret = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

// storedBotToken is the pre-existing token used in stub configs for
// empty-token-path tests. It satisfies the Telegram token regex.
const storedBotToken = "789:DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"

// TestSetShopBot_EmptyToken_ExistingConfig verifies that an empty BotToken
// preserves the stored token and webhook secret while applying the new
// CabinetURL, Enabled, and BotPluginSlug from the input.
func TestSetShopBot_EmptyToken_ExistingConfig(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceWithStubBots(t)

	repo.storedCfg = &vo.ShopBotConfig{
		Token:         secret.NewString(storedBotToken),
		WebhookSecret: storedWebhookSecret,
		CabinetURL:    "https://old.example.com",
		BotPluginSlug: "old-plugin",
	}

	err := svc.SetShopBot(ctx, "t1", service.SetShopBotInput{
		BotToken:   "",
		CabinetURL: "https://new.example.com",
		Enabled:    true,
		BotPlugin:  "cabinet-bot",
	})
	require.NoError(t, err)
	require.Equal(t, 1, repo.upsertCalls)
	require.Equal(t, storedBotToken, repo.lastUpsert.Token.Expose(), "stored token must be preserved")
	require.Equal(t, storedWebhookSecret, repo.lastUpsert.WebhookSecret, "stored webhook secret must be preserved")
	require.Equal(t, "cabinet-bot", repo.lastUpsert.BotPluginSlug, "new plugin must be applied")
	require.Equal(t, "https://new.example.com", repo.lastUpsert.CabinetURL, "new cabinet URL must be applied")
	require.True(t, repo.lastUpsert.Enabled, "new enabled flag must be applied")
}

// TestSetShopBot_EmptyToken_NoConfig verifies that an empty BotToken when no
// config exists returns ErrShopBotInvalidToken and never calls Upsert.
func TestSetShopBot_EmptyToken_NoConfig(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceWithStubBots(t)
	// storedCfg is nil → GetByTenant returns ErrShopBotNotFound

	err := svc.SetShopBot(ctx, "t1", service.SetShopBotInput{
		BotToken:   "",
		CabinetURL: "https://example.com",
		Enabled:    true,
	})
	require.ErrorIs(t, err, service.ErrShopBotInvalidToken)
	require.Equal(t, 0, repo.upsertCalls)
}

// TestSetShopBot_NonEmptyToken_FreshSecret is a regression test verifying that
// a non-empty BotToken always generates a fresh webhook secret, even when a
// stored config with a known secret already exists.
func TestSetShopBot_NonEmptyToken_FreshSecret(t *testing.T) {
	ctx := context.Background()
	svc, repo := newServiceWithStubBots(t)

	repo.storedCfg = &vo.ShopBotConfig{
		Token:         secret.NewString(storedBotToken),
		WebhookSecret: storedWebhookSecret,
		CabinetURL:    "https://old.example.com",
	}

	const newToken = "456:CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"
	err := svc.SetShopBot(ctx, "t1", service.SetShopBotInput{
		BotToken:   newToken,
		CabinetURL: "https://new.example.com",
		Enabled:    true,
	})
	require.NoError(t, err)
	require.Equal(t, newToken, repo.lastUpsert.Token.Expose(), "new token must be applied")
	require.NotEmpty(t, repo.lastUpsert.WebhookSecret, "webhook secret must not be empty")
	require.NotEqual(t, storedWebhookSecret, repo.lastUpsert.WebhookSecret, "non-empty token must regenerate the webhook secret")
}

// TestSetShopBot_EmptyToken_InvalidPlugin verifies that plugin validation runs
// on the empty-token path and rejects an invalid plugin without calling Upsert,
// even when an existing config is present.
func TestSetShopBot_EmptyToken_InvalidPlugin(t *testing.T) {
	ctx := context.Background()
	svc, repo, validator := newResellerServiceWithStubs(t)

	repo.storedCfg = &vo.ShopBotConfig{
		Token:         secret.NewString(storedBotToken),
		WebhookSecret: storedWebhookSecret,
		CabinetURL:    "https://example.com",
	}
	validator.valid = false

	err := svc.SetShopBot(ctx, "t1", service.SetShopBotInput{
		BotToken:   "",
		CabinetURL: "https://example.com",
		BotPlugin:  "ghost-bot",
		Enabled:    true,
	})
	require.ErrorIs(t, err, service.ErrShopBotInvalidPlugin)
	require.Equal(t, 0, repo.upsertCalls, "no upsert must occur when plugin is invalid")
}
