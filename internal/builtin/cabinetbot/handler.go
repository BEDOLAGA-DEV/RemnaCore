package cabinetbot

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// User-facing copy (RU).
const (
	// msgRegistrationFailed is sent to the user when registration fails.
	msgRegistrationFailed = "Не удалось завершить регистрацию. Попробуйте позже."
	msgCommandFailed      = "Не получилось выполнить команду. Попробуйте позже."
	msgStaleKeyboard      = "Кнопка устарела. Отправьте команду ещё раз."
	msgPayPrompt          = "Счёт создан — оплатите по кнопке ниже:"
	btnPay                = "Оплатить"
)

// RequiredPerms is the op-permission set the cabinet-bot handler is granted.
func RequiredPerms() bothost.PermSet {
	return bothost.NewPermSet(
		plugin.PermTelegramSend,
		plugin.PermUsersWrite,
		plugin.PermBillingRead,
		plugin.PermPaymentWrite,
	)
}

// Handler routes an inbound update: recognized commands and callbacks are
// served through the tenant-scoped domain ops; anything else (incl. /start)
// keeps the original register-then-cabinet-button behavior. Every path
// registers the user first (RegisterViaTelegram is idempotent get-or-create),
// so domain ops always resolve the caller. It acts only through the
// bothost.Host op interface (never the bot token or a DB handle).
func Handler(ctx context.Context, update bothost.Update, host bothost.Host) error {
	if err := registerUser(ctx, update, host); err != nil {
		return err
	}
	if update.IsCallback {
		return handleCallback(ctx, update, host)
	}
	switch command(update.Text) {
	case "/plans":
		return handlePlans(ctx, update, host)
	case "/my":
		return handleMy(ctx, update, host)
	case "/balance":
		return handleBalance(ctx, update, host)
	case "/invoices":
		return handleInvoices(ctx, update, host)
	default:
		return openCabinet(ctx, update, host)
	}
}

// command extracts the leading slash-command token, stripping an @botname
// suffix ("/plans@shop_bot arg" → "/plans"). Returns "" for non-commands.
func command(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return ""
	}
	cmd, _, _ := strings.Cut(fields[0], "@")
	return cmd
}

// registerUser registers the update's sender as a customer of the shop. On
// failure it tells the user (best-effort) and returns the error.
func registerUser(ctx context.Context, update bothost.Update, host bothost.Host) error {
	_, err := call(ctx, host, bothost.OpUserRegister, map[string]any{
		"telegram_id":  update.From.ID,
		"display_name": bothost.DisplayName(update.From),
	})
	if err != nil {
		_ = sendText(ctx, host, update.ChatID, msgRegistrationFailed)
		return err
	}
	return nil
}

// openCabinet replies with the button opening the shop's personal cabinet.
func openCabinet(ctx context.Context, update bothost.Update, host bothost.Host) error {
	_, err := call(ctx, host, bothost.OpCabinetOpen, map[string]any{"chat_id": update.ChatID})
	return err
}

// ─── Commands ────────────────────────────────────────────────────────────────

func handlePlans(ctx context.Context, update bothost.Update, host bothost.Host) error {
	out, err := call(ctx, host, bothost.OpPlansList, map[string]any{})
	if err != nil {
		return failCommand(ctx, update, host, err)
	}
	var offers []bothost.TariffOffer
	if err := json.Unmarshal(out, &offers); err != nil {
		return failCommand(ctx, update, host, err)
	}
	text, kb := formatOffers(offers)
	if len(kb.Rows) == 0 {
		return sendText(ctx, host, update.ChatID, text)
	}
	return sendKeyboard(ctx, host, update.ChatID, text, kb)
}

func handleMy(ctx context.Context, update bothost.Update, host bothost.Host) error {
	out, err := call(ctx, host, bothost.OpSubscriptionsMine, map[string]any{"telegram_id": update.From.ID})
	if err != nil {
		return failCommand(ctx, update, host, err)
	}
	var subs []bothost.Subscription
	if err := json.Unmarshal(out, &subs); err != nil {
		return failCommand(ctx, update, host, err)
	}
	// Best-effort plan names; failures fall back to the raw PlanID.
	planNames := make(map[string]string, len(subs))
	for _, s := range subs {
		if _, ok := planNames[s.PlanID]; ok {
			continue
		}
		if o := getOffer(ctx, host, s.PlanID); o != nil {
			planNames[s.PlanID] = o.Name
		}
	}
	return sendText(ctx, host, update.ChatID, formatSubscriptions(subs, planNames))
}

func handleBalance(ctx context.Context, update bothost.Update, host bothost.Host) error {
	out, err := call(ctx, host, bothost.OpBalanceGet, map[string]any{"telegram_id": update.From.ID})
	if err != nil {
		return failCommand(ctx, update, host, err)
	}
	var wallets []bothost.Wallet
	if err := json.Unmarshal(out, &wallets); err != nil {
		return failCommand(ctx, update, host, err)
	}
	return sendText(ctx, host, update.ChatID, formatWallets(wallets))
}

func handleInvoices(ctx context.Context, update bothost.Update, host bothost.Host) error {
	out, err := call(ctx, host, bothost.OpInvoicesPending, map[string]any{"telegram_id": update.From.ID})
	if err != nil {
		return failCommand(ctx, update, host, err)
	}
	var invoices []bothost.Invoice
	if err := json.Unmarshal(out, &invoices); err != nil {
		return failCommand(ctx, update, host, err)
	}
	return sendText(ctx, host, update.ChatID, formatInvoices(invoices))
}

// ─── Callbacks ───────────────────────────────────────────────────────────────

func handleCallback(ctx context.Context, update bothost.Update, host bothost.Host) error {
	switch {
	case strings.HasPrefix(update.CallbackData, callbackPrefixPlan):
		return handlePlanCallback(ctx, update, host)
	case strings.HasPrefix(update.CallbackData, callbackPrefixBuy):
		return handleBuyCallback(ctx, update, host)
	default:
		answerCallback(ctx, host, update.CallbackID, msgStaleKeyboard)
		return nil
	}
}

func handlePlanCallback(ctx context.Context, update bothost.Update, host bothost.Host) error {
	planID := strings.TrimPrefix(update.CallbackData, callbackPrefixPlan)
	answerCallback(ctx, host, update.CallbackID, "")
	o := getOffer(ctx, host, planID)
	if o == nil {
		return sendText(ctx, host, update.ChatID, msgCommandFailed)
	}
	text, kb := formatOfferDetail(*o)
	if len(kb.Rows) == 0 {
		return sendText(ctx, host, update.ChatID, text)
	}
	return sendKeyboard(ctx, host, update.ChatID, text, kb)
}

func handleBuyCallback(ctx context.Context, update bothost.Update, host bothost.Host) error {
	planID := strings.TrimPrefix(update.CallbackData, callbackPrefixBuy)
	answerCallback(ctx, host, update.CallbackID, "")
	out, err := call(ctx, host, bothost.OpCheckoutCreate, map[string]any{
		"telegram_id": update.From.ID,
		"plan_id":     planID,
	})
	if err != nil {
		return failCommand(ctx, update, host, err)
	}
	var res bothost.CheckoutResult
	if err := json.Unmarshal(out, &res); err != nil {
		return failCommand(ctx, update, host, err)
	}
	if res.CheckoutURL == "" {
		return sendText(ctx, host, update.ChatID, msgCommandFailed)
	}
	return sendKeyboard(ctx, host, update.ChatID, msgPayPrompt, bothost.Keyboard{
		Rows: [][]bothost.Button{{{Text: btnPay, URL: res.CheckoutURL}}},
	})
}

// ─── Host helpers ────────────────────────────────────────────────────────────

// call marshals args and invokes one host op.
func call(ctx context.Context, host bothost.Host, op string, args any) (json.RawMessage, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return host.Call(ctx, op, raw)
}

func sendText(ctx context.Context, host bothost.Host, chatID int64, text string) error {
	_, err := call(ctx, host, bothost.OpTelegramSendText, map[string]any{"chat_id": chatID, "text": text})
	return err
}

func sendKeyboard(ctx context.Context, host bothost.Host, chatID int64, text string, kb bothost.Keyboard) error {
	_, err := call(ctx, host, bothost.OpTelegramSendKeyboard, map[string]any{"chat_id": chatID, "text": text, "keyboard": kb})
	return err
}

// answerCallback clears the Telegram callback spinner (best-effort).
func answerCallback(ctx context.Context, host bothost.Host, callbackID, text string) {
	_, _ = call(ctx, host, bothost.OpTelegramAnswerCallback, map[string]any{"callback_id": callbackID, "text": text})
}

// failCommand notifies the user (best-effort) and returns the original error
// so the dispatch layer logs it.
func failCommand(ctx context.Context, update bothost.Update, host bothost.Host, err error) error {
	_ = sendText(ctx, host, update.ChatID, msgCommandFailed)
	return err
}

// getOffer resolves a planID best-effort; nil on any error.
func getOffer(ctx context.Context, host bothost.Host, planID string) *bothost.TariffOffer {
	out, err := call(ctx, host, bothost.OpPlansGet, map[string]any{"plan_id": planID})
	if err != nil {
		return nil
	}
	var o bothost.TariffOffer
	if json.Unmarshal(out, &o) != nil {
		return nil
	}
	return &o
}
