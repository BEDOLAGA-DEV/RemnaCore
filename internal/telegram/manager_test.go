package telegram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
)

type stubLister struct {
	bots []vo.ShopBotWithTenant
	err  error
}

func (s stubLister) ListEnabledShopBots(_ context.Context) ([]vo.ShopBotWithTenant, error) {
	return s.bots, s.err
}

func newManagerForTest(lister ShopBotLister, webhookURL string) *BotManager {
	cfg := &config.Config{}
	cfg.Telegram.WebhookURL = webhookURL
	return NewBotManager(nil, nil, lister, cfg, testLogger())
}

func TestBotManager_Load_BuildsEnabledShops(t *testing.T) {
	lister := stubLister{bots: []vo.ShopBotWithTenant{
		{TenantID: "tenant-a", Config: vo.ShopBotConfig{Token: config.NewSecretString("1:a"), CabinetURL: "https://a", Enabled: true}},
		{TenantID: "tenant-b", Config: vo.ShopBotConfig{Token: config.NewSecretString("2:b"), CabinetURL: "https://b", Enabled: true}},
	}}
	m := newManagerForTest(lister, "https://host.example.com/webhooks/telegram")
	require.NoError(t, m.Load(context.Background()))
	assert.NotNil(t, m.shops["tenant-a"])
	assert.NotNil(t, m.shops["tenant-b"])
	// Webhook URL derived from origin + prefix + tenant.
	assert.Equal(t, "https://host.example.com/webhooks/telegram/tenant-a", m.shops["tenant-a"].webhookURL)
}

func TestBotManager_Load_SkipsBadBotKeepsOthers(t *testing.T) {
	lister := stubLister{bots: []vo.ShopBotWithTenant{
		{TenantID: "good", Config: vo.ShopBotConfig{Token: config.NewSecretString("1:a"), CabinetURL: "https://a", Enabled: true}},
		{TenantID: "bad", Config: vo.ShopBotConfig{Token: config.NewSecretString(""), CabinetURL: "https://b", Enabled: true}},
	}}
	m := newManagerForTest(lister, "")
	require.NoError(t, m.Load(context.Background()))
	assert.NotNil(t, m.shops["good"])
	// Empty token → bot.Init leaves b.bot nil → not routable; excluded from shops.
	assert.Nil(t, m.shops["bad"])
}

func TestBotManager_ShopWebhookHandler_Routing(t *testing.T) {
	lister := stubLister{bots: []vo.ShopBotWithTenant{
		{TenantID: "tenant-a", Config: vo.ShopBotConfig{Token: config.NewSecretString("1:a"), CabinetURL: "https://a", Enabled: true}},
	}}
	m := newManagerForTest(lister, "")
	require.NoError(t, m.Load(context.Background()))

	r := chi.NewRouter()
	r.Post("/webhooks/telegram/{tenantID}", m.ShopWebhookHandler())

	// Unknown tenant → 404.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/webhooks/telegram/unknown", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Known tenant → delegated to the bot handler (not 404).
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/webhooks/telegram/tenant-a", nil))
	assert.NotEqual(t, http.StatusNotFound, rec2.Code)
}
