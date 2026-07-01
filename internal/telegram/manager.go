package telegram

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// WebhookPathPrefix is the per-shop webhook path; the tenant id is appended.
const WebhookPathPrefix = "/webhooks/telegram/"

// ShopBotLister loads enabled per-shop bot configs (satisfied by
// *reseller.ResellerService). It self-scopes to the platform sentinel.
type ShopBotLister interface {
	ListEnabledShopBots(ctx context.Context) ([]vo.ShopBotWithTenant, error)
}

// BotManager runs one bot per shop plus an optional platform bot, and routes
// inbound webhooks to the right bot by tenant id.
type BotManager struct {
	platform      *Bot
	identity      *identity.Service
	lister        ShopBotLister
	webhookOrigin string
	logger        *slog.Logger

	botRegistry *BuiltinBotRegistry
	botOps      *bothost.Registry
	txRunner    txmanager.Runner

	mu      sync.RWMutex
	shops   map[string]*Bot
	cancels map[string]context.CancelFunc
}

// NewBotManager builds the manager. The per-shop webhook origin (scheme://host)
// is derived from cfg.Telegram.WebhookURL; empty/unparseable → long-polling.
func NewBotManager(
	platform *Bot,
	identitySvc *identity.Service,
	lister ShopBotLister,
	cfg *config.Config,
	botRegistry *BuiltinBotRegistry,
	botOps *bothost.Registry,
	txRunner txmanager.Runner,
	logger *slog.Logger,
) *BotManager {
	origin := ""
	if raw := cfg.Telegram.WebhookURL; raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
			origin = u.Scheme + "://" + u.Host
		}
	}
	return &BotManager{
		platform:      platform,
		identity:      identitySvc,
		lister:        lister,
		webhookOrigin: origin,
		botRegistry:   botRegistry,
		botOps:        botOps,
		txRunner:      txRunner,
		logger:        logger.With(slog.String("component", "telegram_bot_manager")),
		shops:         make(map[string]*Bot),
		cancels:       make(map[string]context.CancelFunc),
	}
}

// shopWebhookURL returns the per-tenant webhook URL, or "" when no origin is
// configured (the bot then long-polls).
func (m *BotManager) shopWebhookURL(tenantID string) string {
	if m.webhookOrigin == "" {
		return ""
	}
	return m.webhookOrigin + WebhookPathPrefix + tenantID
}

// Load initialises the platform bot and every enabled shop bot. A single bot's
// init failure is logged and skipped — it never aborts the others or boot.
func (m *BotManager) Load(ctx context.Context) error {
	if m.platform != nil {
		if err := m.platform.Init(ctx); err != nil {
			m.logger.Error("platform bot init failed", slog.Any("error", err))
		}
	}

	list, err := m.lister.ListEnabledShopBots(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entry := range list {
		webhookURL := m.shopWebhookURL(entry.TenantID)
		if webhookURL != "" && entry.Config.WebhookSecret == "" {
			// Fail closed: an empty secret would leave the webhook endpoint
			// unauthenticated (the library skips the secret-token check when the
			// configured secret is ""). Fall back to long-polling instead of
			// serving inbound updates without a gate.
			m.logger.Warn("shop bot has no webhook secret; forcing long-poll", slog.String("tenant", entry.TenantID))
			webhookURL = ""
		}
		bot := NewShopBot(
			entry.Config.Token.Expose(),
			webhookURL,
			entry.Config.WebhookSecret,
			entry.TenantID,
			entry.Config.CabinetURL,
			m.identity,
			entry.Config.BotPluginSlug,
			m.botRegistry,
			m.botOps,
			m.txRunner,
			m.logger,
		)
		if err := bot.Init(ctx); err != nil {
			m.logger.Error("shop bot init failed", slog.String("tenant", entry.TenantID), slog.Any("error", err))
			continue
		}
		if bot.bot == nil { // empty/invalid token left it disabled
			m.logger.Warn("shop bot disabled (no usable token)", slog.String("tenant", entry.TenantID))
			continue
		}
		m.shops[entry.TenantID] = bot
	}
	m.logger.Info("telegram bots loaded", slog.Int("shops", len(m.shops)))
	return nil
}

// Run launches each initialised bot's processing loop in its own goroutine. The
// lock is held while mutating cancels and reading shops (startLocked records the
// cancel func); the launched goroutines touch only their own bot, never m.
func (m *BotManager) Run(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.platform != nil && m.platform.bot != nil {
		m.startLocked(ctx, tenantctxPlatformKey, m.platform)
	}
	for tenantID, bot := range m.shops {
		m.startLocked(ctx, tenantID, bot)
	}
}

// startLocked records a cancel func and launches the bot loop. Caller holds m.mu.
func (m *BotManager) startLocked(ctx context.Context, key string, bot *Bot) {
	if prev, ok := m.cancels[key]; ok {
		prev() // cancel any prior loop for this key before replacing it (no goroutine leak)
	}
	botCtx, cancel := context.WithCancel(ctx)
	m.cancels[key] = cancel
	go func() {
		if err := bot.Run(botCtx); err != nil {
			m.logger.Error("telegram bot run error", slog.String("key", key), slog.Any("error", err))
		}
	}()
}

// Stop cancels every running bot.
func (m *BotManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.cancels = make(map[string]context.CancelFunc)
}

// Reload rebuilds the full bot set (stop → load → run).
func (m *BotManager) Reload(ctx context.Context) error {
	m.Stop()
	m.mu.Lock()
	m.shops = make(map[string]*Bot)
	m.mu.Unlock()
	if err := m.Load(ctx); err != nil {
		return err
	}
	m.Run(ctx)
	return nil
}

// ShopWebhookHandler routes POST /webhooks/telegram/{tenantID} to that shop's
// bot, which verifies the per-bot secret token before processing.
func (m *BotManager) ShopWebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenantID")
		m.mu.RLock()
		bot := m.shops[tenantID]
		m.mu.RUnlock()
		if bot == nil {
			http.NotFound(w, r)
			return
		}
		bot.WebhookHandler()(w, r)
	}
}

// PlatformWebhookHandler preserves the legacy /webhooks/telegram path for the
// optional platform bot.
func (m *BotManager) PlatformWebhookHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if m.platform == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		m.platform.WebhookHandler()(w, r)
	}
}

// tenantctxPlatformKey keys the platform bot in the cancels map (never a valid
// tenant id, so it cannot collide with a shop).
const tenantctxPlatformKey = "__platform__"
