package telegram

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/cabinetbot"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// DomainReaders bundles the tenant-scoped domain read/act ports handed to
// shop-bot op dispatch (bothost.OpContext). Nil fields disable their ops.
// It is passed as one parameter so the Bot/BotManager constructors do not gain
// five individual parameters each time a new reader is added.
type DomainReaders struct {
	Tariffs  bothost.TariffReader
	Subs     bothost.SubscriptionReader
	Invoices bothost.InvoiceReader
	Balance  bothost.BalanceReader
	Checkout bothost.CheckoutStarter
}

// Bot wraps the Telegram bot instance and all domain services needed to handle
// commands and callback queries.
//
// Architecture: The bot uses domain services for all write operations (checkout,
// cancel subscription) and repositories directly for read-only queries (plans,
// subscriptions, bindings). This follows the CQRS read path pattern -- reads
// bypass the write-side services for performance and simplicity.
type Bot struct {
	bot           *tgbot.Bot
	token         string
	webhookURL    string
	cabinetURL    string
	tenantID      string
	webhookSecret string
	isShop        bool

	identity *identity.Service
	billing  *billingservice.BillingService
	checkout *billingservice.CheckoutService
	bindings multisub.BindingRepository
	plans    billing.PlanReader
	subs     billing.SubscriptionReader
	logger   *slog.Logger

	// shop-bot dispatch fields (non-nil only when isShop is true)
	botPluginSlug string
	botRegistry   *BuiltinBotRegistry
	botOps        *bothost.Registry
	wasmBots      WASMBotDispatcher
	txRunner      txmanager.Runner
	readers       DomainReaders
}

// NewBot creates the platform Bot. The underlying tgbot.Bot is initialized
// lazily in Init so that the constructor does not block on a network call.
func NewBot(
	cfg *config.Config,
	identitySvc *identity.Service,
	billingSvc *billingservice.BillingService,
	checkoutSvc *billingservice.CheckoutService,
	bindingRepo multisub.BindingRepository,
	planRepo billing.PlanReader,
	subRepo billing.SubscriptionReader,
	txRunner txmanager.Runner,
	logger *slog.Logger,
) *Bot {
	return &Bot{
		token:      cfg.Telegram.BotToken.Expose(),
		webhookURL: cfg.Telegram.WebhookURL,
		cabinetURL: cfg.Telegram.CabinetURL,
		tenantID:   tenantctx.PlatformScopeSentinel,
		txRunner:   txRunner,

		identity: identitySvc,
		billing:  billingSvc,
		checkout: checkoutSvc,
		bindings: bindingRepo,
		plans:    planRepo,
		subs:     subRepo,
		logger:   logger.With(slog.String("component", "telegram_bot")),
	}
}

// NewShopBot builds a per-shop bot that dispatches all updates through the
// BuiltinBotRegistry (or a WASM plugin when wasmBots resolves the slug) using
// the configured plugin slug (default: cabinet-bot). Platform-only services are
// nil — a shop bot never reaches the other command handlers.
func NewShopBot(
	token, webhookURL, webhookSecret, tenantID, cabinetURL string,
	identitySvc *identity.Service,
	botPluginSlug string,
	botRegistry *BuiltinBotRegistry,
	botOps *bothost.Registry,
	txRunner txmanager.Runner,
	wasmBots WASMBotDispatcher,
	readers DomainReaders,
	logger *slog.Logger,
) *Bot {
	return &Bot{
		token:         token,
		webhookURL:    webhookURL,
		webhookSecret: webhookSecret,
		cabinetURL:    cabinetURL,
		tenantID:      tenantID,
		isShop:        true,
		identity:      identitySvc,
		botPluginSlug: botPluginSlug,
		botRegistry:   botRegistry,
		botOps:        botOps,
		wasmBots:      wasmBots,
		txRunner:      txRunner,
		readers:       readers,
		logger:        logger.With(slog.String("component", "telegram_shop_bot"), slog.String("tenant", tenantID)),
	}
}

// Init constructs the underlying tgbot.Bot and registers handlers. It makes no
// network call (WithSkipGetMe), so the manager can build every bot synchronously
// before serving webhooks. An empty token leaves the bot disabled (b.bot nil).
func (b *Bot) Init(_ context.Context) error {
	if b.token == "" {
		b.logger.Warn("telegram bot token not configured, bot disabled")
		return nil
	}
	opts := []tgbot.Option{
		tgbot.WithSkipGetMe(),
		tgbot.WithErrorsHandler(func(err error) {
			b.logger.Error("telegram bot error", slog.Any("error", err))
		}),
	}
	if b.webhookSecret != "" {
		opts = append(opts, tgbot.WithWebhookSecretToken(b.webhookSecret))
	}
	if b.isShop {
		opts = append(opts, tgbot.WithDefaultHandler(b.dispatchUpdate))
	}
	tb, err := tgbot.New(b.token, opts...)
	if err != nil {
		return err
	}
	b.bot = tb
	b.registerHandlers()
	return nil
}

// Run sets the webhook (when configured) and processes updates until ctx is
// cancelled. A disabled bot (b.bot nil) returns immediately.
func (b *Bot) Run(ctx context.Context) error {
	if b.bot == nil {
		return nil
	}
	if b.webhookURL != "" {
		b.logger.Info("starting telegram bot in webhook mode", slog.String("url", b.webhookURL))
		if _, err := b.bot.SetWebhook(ctx, &tgbot.SetWebhookParams{URL: b.webhookURL, SecretToken: b.webhookSecret}); err != nil {
			return err
		}
		b.bot.StartWebhook(ctx)
	} else {
		b.logger.Info("starting telegram bot in long-polling mode")
		b.bot.Start(ctx)
	}
	return nil
}

// WebhookHandler returns an http.HandlerFunc that processes Telegram webhook
// updates. Only meaningful when running in webhook mode.
func (b *Bot) WebhookHandler() http.HandlerFunc {
	if b.bot == nil {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
	return b.bot.WebhookHandler()
}

// registerHandlers wires handlers. Shop bots dispatch all updates through the
// catch-all default handler (dispatchUpdate, registered in Init); the platform
// bot keeps the full command + callback set.
func (b *Bot) registerHandlers() {
	if b.isShop {
		return
	}

	// Command handlers
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdStart, tgbot.MatchTypeCommand, b.handleStart)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdPlans, tgbot.MatchTypeCommand, b.handlePlans)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdSubscribe, tgbot.MatchTypeCommand, b.handleSubscribe)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdMy, tgbot.MatchTypeCommand, b.handleMy)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdTraffic, tgbot.MatchTypeCommand, b.handleTraffic)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdSupport, tgbot.MatchTypeCommand, b.handleSupport)
	b.bot.RegisterHandler(tgbot.HandlerTypeMessageText, CmdReferral, tgbot.MatchTypeCommand, b.handleReferral)

	// Callback query handlers
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, CallbackPrefixPlan, tgbot.MatchTypePrefix, b.handlePlanCallback)
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, CallbackPrefixAddon, tgbot.MatchTypePrefix, b.handleAddonCallback)
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, CallbackPrefixConfirm, tgbot.MatchTypePrefix, b.handleConfirmCallback)
	b.bot.RegisterHandler(tgbot.HandlerTypeCallbackQueryData, CallbackPrefixCancel, tgbot.MatchTypePrefix, b.handleCancelCallback)
}

// sendText is a convenience wrapper for sending a plain-text message.
// withScope runs fn inside a transaction carrying this bot's tenant scope
// (the platform sentinel for the platform bot), so RLS-protected reads see
// the rows they are entitled to. Repos never set the GUC themselves.
func (b *Bot) withScope(ctx context.Context, fn func(ctx context.Context) error) error {
	return b.txRunner.RunInTx(tenantctx.WithTenantID(ctx, b.tenantID), fn)
}

func (b *Bot) sendText(ctx context.Context, chatID int64, text string) {
	if b.bot == nil {
		b.logger.Warn("sendText before Init", slog.Int64("chat_id", chatID))
		return
	}
	_, err := b.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	})
	if err != nil {
		b.logger.Error("failed to send message",
			slog.Int64("chat_id", chatID),
			slog.Any("error", err),
		)
	}
}

// sendTextWithKeyboard sends an HTML message with an inline keyboard.
func (b *Bot) sendTextWithKeyboard(ctx context.Context, chatID int64, text string, kb *models.InlineKeyboardMarkup) {
	if b.bot == nil {
		b.logger.Warn("sendTextWithKeyboard before Init", slog.Int64("chat_id", chatID))
		return
	}
	_, err := b.bot.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: kb,
	})
	if err != nil {
		b.logger.Error("failed to send message with keyboard",
			slog.Int64("chat_id", chatID),
			slog.Any("error", err),
		)
	}
}

// answerCallback acknowledges a callback query with an optional toast.
func (b *Bot) answerCallback(ctx context.Context, callbackID, text string) {
	if b.bot == nil {
		b.logger.Warn("answerCallback before Init")
		return
	}
	_, err := b.bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	})
	if err != nil {
		b.logger.Error("failed to answer callback query", slog.Any("error", err))
	}
}

// newOpContext builds the per-dispatch OpContext from the bot's bound fields.
// Extracted to avoid duplicating the literal in both the built-in and WASM
// dispatch paths.
func (b *Bot) newOpContext() *bothost.OpContext {
	return &bothost.OpContext{
		TenantID:   b.tenantID,
		CabinetURL: b.cabinetURL,
		Sender:     b,
		Identity:   b.identity,
		TxRunner:   b.txRunner,
		Tariffs:    b.readers.Tariffs,
		Subs:       b.readers.Subs,
		Invoices:   b.readers.Invoices,
		Balance:    b.readers.Balance,
		Checkout:   b.readers.Checkout,
	}
}

// dispatchUpdate is the default handler for shop bots. It maps the raw
// go-telegram Update to a bothost.Update, then tries (in order):
//  1. The built-in BuiltinBotRegistry for the configured slug.
//  2. The WASMBotDispatcher when the slug resolves to an enabled WASM bot plugin.
//  3. The cabinet-bot fallback (always registered).
func (b *Bot) dispatchUpdate(ctx context.Context, _ *tgbot.Bot, u *models.Update) {
	upd, ok := toBotHostUpdate(u)
	if !ok {
		return
	}
	slug := b.botPluginSlug
	if slug == "" {
		slug = cabinetbot.Slug
	}
	handler, perms, found := b.botRegistry.Lookup(slug)
	if !found {
		// Try the WASM dispatcher before falling back to the cabinet-bot.
		if b.wasmBots != nil {
			if wperms, entry, wasmOK := b.wasmBots.Resolve(ctx, slug); wasmOK {
				host := b.botOps.Bind(b.newOpContext(), wperms)
				env, err := json.Marshal(upd)
				if err != nil {
					b.logger.Error("wasm bot marshal update failed",
						slog.String("tenant", b.tenantID), slog.String("slug", slug), slog.Any("error", err))
					return
				}
				if err := b.wasmBots.Invoke(plugin.WithBotHost(ctx, host), slug, entry, env); err != nil {
					b.logger.Error("wasm bot dispatch failed",
						slog.String("tenant", b.tenantID), slog.String("slug", slug), slog.Any("error", err))
				}
				return
			}
		}
		b.logger.Warn("bot plugin not found; falling back to cabinet-bot",
			slog.String("tenant", b.tenantID), slog.String("slug", slug))
		handler, perms, found = b.botRegistry.Lookup(cabinetbot.Slug)
		if !found {
			b.logger.Error("cabinet-bot fallback not registered", slog.String("tenant", b.tenantID))
			return
		}
	}
	host := b.botOps.Bind(b.newOpContext(), perms)
	if err := handler(ctx, upd, host); err != nil {
		b.logger.Error("bot plugin handler failed",
			slog.String("tenant", b.tenantID), slog.String("slug", slug), slog.Any("error", err))
	}
}

// toBotHostUpdate maps a go-telegram Update to the serializable bothost.Update.
// Returns false for updates carrying neither a message nor a callback query.
func toBotHostUpdate(u *models.Update) (bothost.Update, bool) {
	switch {
	case u.Message != nil && u.Message.From != nil:
		m := u.Message
		return bothost.Update{
			ChatID:    m.Chat.ID,
			From:      bothost.User{ID: m.From.ID, FirstName: m.From.FirstName, LastName: m.From.LastName, Username: m.From.Username},
			Text:      m.Text,
			MessageID: m.ID,
		}, true
	case u.CallbackQuery != nil:
		cq := u.CallbackQuery
		upd := bothost.Update{
			From:         bothost.User{ID: cq.From.ID, FirstName: cq.From.FirstName, LastName: cq.From.LastName, Username: cq.From.Username},
			IsCallback:   true,
			CallbackID:   cq.ID,
			CallbackData: cq.Data,
		}
		if cq.Message.Message != nil {
			upd.ChatID = cq.Message.Message.Chat.ID
			upd.MessageID = cq.Message.Message.ID
		}
		return upd, true
	default:
		return bothost.Update{}, false
	}
}

// editMessageText replaces the text of an existing message (used for inline
// keyboard interactions).
func (b *Bot) editMessageText(ctx context.Context, chatID int64, messageID int, text string, kb *models.InlineKeyboardMarkup) {
	if b.bot == nil {
		b.logger.Warn("editMessageText before Init", slog.Int64("chat_id", chatID))
		return
	}
	params := &tgbot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if kb != nil {
		params.ReplyMarkup = kb
	}
	_, err := b.bot.EditMessageText(ctx, params)
	if err != nil {
		b.logger.Error("failed to edit message",
			slog.Int64("chat_id", chatID),
			slog.Any("error", err),
		)
	}
}
