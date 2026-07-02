package botsdk

// Op name constants — the public op-catalog ABI. These MUST match the host's
// bothost.Op* consts (internal/telegram/bothost/ops.go and ops_domain.go).
const (
	OpTelegramSendText       = "telegram.send_text"
	OpTelegramSendKeyboard   = "telegram.send_keyboard"
	OpTelegramAnswerCallback = "telegram.answer_callback"
	OpTelegramEditMessage    = "telegram.edit_message"
	OpCabinetOpen            = "cabinet.open"
	OpUserRegister           = "user.register"

	OpPlansList            = "plans.list"
	OpPlansGet             = "plans.get"
	OpSubscriptionsMine    = "subscriptions.mine"
	OpSubscriptionsGet     = "subscriptions.get"
	OpSubscriptionsCancel  = "subscriptions.cancel"
	OpSubscriptionsUpgrade = "subscriptions.upgrade"
	OpInvoicesPending      = "invoices.pending"
	OpBalanceGet           = "balance.get"
	OpCheckoutCreate       = "checkout.create"
)

// ChannelTelegram is the default sales channel for PlansList.
const ChannelTelegram = "telegram"

// --- Telegram send ops (require the telegram:send permission) ---

// SendText sends a plain text message to chatID.
func SendText(chatID int64, text string) error {
	_, err := Call(OpTelegramSendText, map[string]any{"chat_id": chatID, "text": text})
	return err
}

// SendKeyboard sends text with an inline keyboard to chatID.
func SendKeyboard(chatID int64, text string, kb Keyboard) error {
	_, err := Call(OpTelegramSendKeyboard, map[string]any{"chat_id": chatID, "text": text, "keyboard": kb})
	return err
}

// AnswerCallback acknowledges a callback query (clears the client spinner).
func AnswerCallback(callbackID, text string) error {
	_, err := Call(OpTelegramAnswerCallback, map[string]any{"callback_id": callbackID, "text": text})
	return err
}

// EditMessage replaces the text (and optional keyboard) of an existing message.
func EditMessage(chatID int64, messageID int, text string, kb *Keyboard) error {
	args := map[string]any{"chat_id": chatID, "message_id": messageID, "text": text}
	if kb != nil {
		args["keyboard"] = kb
	}
	_, err := Call(OpTelegramEditMessage, args)
	return err
}

// OpenCabinet replies with a button opening the shop cabinet. An empty text uses
// the host's default copy.
func OpenCabinet(chatID int64, text string) error {
	args := map[string]any{"chat_id": chatID}
	if text != "" {
		args["text"] = text
	}
	_, err := Call(OpCabinetOpen, args)
	return err
}

// RegisterUser registers (get-or-creates) the Telegram user as a shop customer.
// It is idempotent and must be called before any user-scoped domain op.
func RegisterUser(telegramID int64, displayName string) error {
	_, err := Call(OpUserRegister, map[string]any{"telegram_id": telegramID, "display_name": displayName})
	return err
}

// --- Domain read ops (require billing:read) ---

// PlansList returns the tariff offers visible in the given channel. Pass
// ChannelTelegram (or "") for the default.
func PlansList(channel string) ([]TariffOffer, error) {
	if channel == "" {
		channel = ChannelTelegram
	}
	var offers []TariffOffer
	if err := callInto(OpPlansList, map[string]any{"channel": channel}, &offers); err != nil {
		return nil, err
	}
	return offers, nil
}

// PlanGet resolves a plan ID to its tariff offer.
func PlanGet(planID string) (*TariffOffer, error) {
	var offer TariffOffer
	if err := callInto(OpPlansGet, map[string]any{"plan_id": planID}, &offer); err != nil {
		return nil, err
	}
	return &offer, nil
}

// SubscriptionsMine returns the caller's active subscriptions.
func SubscriptionsMine(telegramID int64) ([]Subscription, error) {
	var subs []Subscription
	if err := callInto(OpSubscriptionsMine, map[string]any{"telegram_id": telegramID}, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// SubscriptionGet returns a single subscription the caller owns (host enforces
// ownership; a foreign ID yields an error).
func SubscriptionGet(telegramID int64, id string) (*Subscription, error) {
	var sub Subscription
	if err := callInto(OpSubscriptionsGet, map[string]any{"telegram_id": telegramID, "id": id}, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// InvoicesPending returns the caller's unpaid invoices.
func InvoicesPending(telegramID int64) ([]Invoice, error) {
	var invoices []Invoice
	if err := callInto(OpInvoicesPending, map[string]any{"telegram_id": telegramID}, &invoices); err != nil {
		return nil, err
	}
	return invoices, nil
}

// BalanceGet returns the caller's wallets.
func BalanceGet(telegramID int64) ([]Wallet, error) {
	var wallets []Wallet
	if err := callInto(OpBalanceGet, map[string]any{"telegram_id": telegramID}, &wallets); err != nil {
		return nil, err
	}
	return wallets, nil
}

// --- Domain write ops ---

// CheckoutCreate starts a checkout for planID (requires payment:write).
func CheckoutCreate(telegramID int64, planID string) (*CheckoutResult, error) {
	var res CheckoutResult
	if err := callInto(OpCheckoutCreate, map[string]any{"telegram_id": telegramID, "plan_id": planID}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// CancelSubscription cancels a subscription the caller owns (requires
// billing:write). An empty reason is sent as none.
func CancelSubscription(telegramID int64, id, reason string) error {
	args := map[string]any{"telegram_id": telegramID, "id": id}
	if reason != "" {
		args["reason"] = reason
	}
	_, err := Call(OpSubscriptionsCancel, args)
	return err
}

// UpgradeSubscription moves a subscription the caller owns to newPlanID
// (requires billing:write; host validates the plan is visible to the shop).
func UpgradeSubscription(telegramID int64, id, newPlanID string) error {
	_, err := Call(OpSubscriptionsUpgrade, map[string]any{"telegram_id": telegramID, "id": id, "new_plan_id": newPlanID})
	return err
}
