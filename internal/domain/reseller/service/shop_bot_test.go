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
	lastUpsert vo.ShopBotConfig
}

func (s *stubShopBotRepo) Upsert(_ context.Context, _ string, cfg vo.ShopBotConfig) error {
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

// newServiceWithStubBots constructs a ResellerService wired with a stub
// ShopBotRepository so that shop-bot unit tests remain database-free.
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
	)
	return svc, repo
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
