package telegram

import (
	"context"
	"log/slog"
	"net/http"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
)

// Bot wraps the Telegram bot instance and all domain services needed to handle
// commands and callback queries.
type Bot struct {
	bot        *tgbot.Bot
	token      string
	webhookURL string
	cabinetURL string

	identity *identity.Service
	billing  *billingservice.BillingService
	checkout *billingservice.CheckoutService
	bindings multisub.BindingRepository
	plans    billing.PlanRepository
	subs     billing.SubscriptionRepository
	logger   *slog.Logger
}

// NewBot creates a new Bot. The underlying tgbot.Bot is initialized lazily in
// Start so that the constructor does not block on a network call.
func NewBot(
	cfg *config.Config,
	identitySvc *identity.Service,
	billingSvc *billingservice.BillingService,
	checkoutSvc *billingservice.CheckoutService,
	bindingRepo multisub.BindingRepository,
	planRepo billing.PlanRepository,
	subRepo billing.SubscriptionRepository,
	logger *slog.Logger,
) *Bot {
	return &Bot{
		token:      cfg.Telegram.BotToken,
		webhookURL: cfg.Telegram.WebhookURL,
		cabinetURL: cfg.Telegram.CabinetURL,

		identity: identitySvc,
		billing:  billingSvc,
		checkout: checkoutSvc,
		bindings: bindingRepo,
		plans:    planRepo,
		subs:     subRepo,
		logger:   logger.With(slog.String("component", "telegram_bot")),
	}
}

// Start initializes the Telegram bot and begins processing updates. If a
// webhook URL is configured, the bot runs in webhook mode; otherwise it falls
// back to long polling. Start blocks until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	if b.token == "" {
		b.logger.Warn("telegram bot token not configured, skipping bot startup")
		return nil
	}

	opts := []tgbot.Option{
		tgbot.WithSkipGetMe(),
		tgbot.WithErrorsHandler(func(err error) {
			b.logger.Error("telegram bot error", slog.Any("error", err))
		}),
	}

	tb, err := tgbot.New(b.token, opts...)
	if err != nil {
		return err
	}
	b.bot = tb

	b.registerHandlers()

	if b.webhookURL != "" {
		b.logger.Info("starting telegram bot in webhook mode", slog.String("url", b.webhookURL))
		_, err = tb.SetWebhook(ctx, &tgbot.SetWebhookParams{URL: b.webhookURL})
		if err != nil {
			return err
		}
		tb.StartWebhook(ctx)
	} else {
		b.logger.Info("starting telegram bot in long-polling mode")
		tb.Start(ctx)
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

// registerHandlers wires all command and callback handlers to the bot.
func (b *Bot) registerHandlers() {
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
func (b *Bot) sendText(ctx context.Context, chatID int64, text string) {
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
	_, err := b.bot.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
	})
	if err != nil {
		b.logger.Error("failed to answer callback query", slog.Any("error", err))
	}
}

// editMessageText replaces the text of an existing message (used for inline
// keyboard interactions).
func (b *Bot) editMessageText(ctx context.Context, chatID int64, messageID int, text string, kb *models.InlineKeyboardMarkup) {
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
