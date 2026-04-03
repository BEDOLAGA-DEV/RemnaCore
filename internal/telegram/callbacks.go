package telegram

import (
	"context"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
)

// handlePlanCallback handles callback queries with the "plan:" prefix.
// It shows detailed plan information and addon selection.
func (b *Bot) handlePlanCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	cb := update.CallbackQuery
	planID := strings.TrimPrefix(cb.Data, CallbackPrefixPlan)

	plan, err := b.plans.GetByID(ctx, planID)
	if err != nil {
		b.logger.Error("plan not found for callback", slog.String("plan_id", planID), slog.Any("error", err))
		b.answerCallback(ctx, cb.ID, "Plan not found.")
		return
	}

	text := FormatPlanDetail(plan)
	chatID, messageID := callbackChatAndMessage(cb)

	if len(plan.AvailableAddons) > 0 {
		text += "\nSelect addons or confirm:"
		b.editMessageText(ctx, chatID, messageID, text, AddonsKeyboard(plan, nil))
	} else {
		text += "\nConfirm purchase:"
		b.editMessageText(ctx, chatID, messageID, text, ConfirmPurchaseKeyboard(plan.ID, nil))
	}

	b.answerCallback(ctx, cb.ID, "")
}

// handleAddonCallback handles "addon:{planID}:{addonID}" callbacks. It toggles
// addon selection and re-renders the keyboard.
func (b *Bot) handleAddonCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	cb := update.CallbackQuery
	data := strings.TrimPrefix(cb.Data, CallbackPrefixAddon)

	// data format: {planID}:{addonID}
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 2 {
		b.answerCallback(ctx, cb.ID, "Invalid addon selection.")
		return
	}
	planID, addonID := parts[0], parts[1]

	plan, err := b.plans.GetByID(ctx, planID)
	if err != nil {
		b.answerCallback(ctx, cb.ID, "Plan not found.")
		return
	}

	// Parse currently selected addons from the existing keyboard (stored in the
	// confirm button callback data). For simplicity, we toggle based on the
	// current message keyboard state. Since we cannot reliably read the current
	// keyboard state from the update, we use a simple toggle approach: parse
	// the confirm button from the message reply markup.
	selectedAddons := parseSelectedAddons(cb)
	if containsAddon(selectedAddons, addonID) {
		selectedAddons = removeAddon(selectedAddons, addonID)
	} else {
		selectedAddons = append(selectedAddons, addonID)
	}

	text := FormatPlanDetail(plan) + "\nSelect addons or confirm:"
	chatID, messageID := callbackChatAndMessage(cb)
	b.editMessageText(ctx, chatID, messageID, text, AddonsKeyboard(plan, selectedAddons))
	b.answerCallback(ctx, cb.ID, "")
}

// handleConfirmCallback handles "confirm:{planID}:{addonIDs}" callbacks.
// It creates a checkout for the authenticated user.
func (b *Bot) handleConfirmCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	cb := update.CallbackQuery
	data := strings.TrimPrefix(cb.Data, CallbackPrefixConfirm)

	// data format: {planID}:{comma-separated addonIDs}
	parts := strings.SplitN(data, ":", 2)
	if len(parts) < 1 || parts[0] == "" {
		b.answerCallback(ctx, cb.ID, "Invalid confirmation data.")
		return
	}
	planID := parts[0]
	var addonIDs []string
	if len(parts) > 1 && parts[1] != "" {
		addonIDs = strings.Split(parts[1], ",")
	}

	chatID, messageID := callbackChatAndMessage(cb)
	tgUserID := cb.From.ID

	user, err := b.identity.GetByTelegramID(ctx, tgUserID)
	if err != nil || user == nil {
		b.editMessageText(ctx, chatID, messageID, "Please link your Telegram account at "+b.cabinetURL+" first.", nil)
		b.answerCallback(ctx, cb.ID, "Account not linked.")
		return
	}

	result, err := b.checkout.StartCheckout(ctx, billingservice.CheckoutRequest{
		UserID:    user.ID,
		UserEmail: user.Email,
		PlanID:    planID,
		AddonIDs:  addonIDs,
	})
	if err != nil {
		b.logger.Error("checkout failed", slog.Any("error", err))
		b.editMessageText(ctx, chatID, messageID, "Failed to start checkout. Please try again or use the web cabinet.", nil)
		b.answerCallback(ctx, cb.ID, "Checkout failed.")
		return
	}

	text := "Checkout started!\n\n"
	if result.CheckoutURL != "" {
		text += "Complete payment: " + result.CheckoutURL
	} else {
		text += "Subscription ID: " + result.SubscriptionID
	}
	b.editMessageText(ctx, chatID, messageID, text, nil)
	b.answerCallback(ctx, cb.ID, "Checkout started!")
}

// handleCancelCallback handles "cancel:{subID}" callbacks.
func (b *Bot) handleCancelCallback(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.CallbackQuery == nil {
		return
	}
	cb := update.CallbackQuery
	subID := strings.TrimPrefix(cb.Data, CallbackPrefixCancel)
	chatID, messageID := callbackChatAndMessage(cb)

	if subID == "checkout" {
		b.editMessageText(ctx, chatID, messageID, "Purchase cancelled.", nil)
		b.answerCallback(ctx, cb.ID, "Cancelled.")
		return
	}

	err := b.billing.CancelSubscription(ctx, subID)
	if err != nil {
		b.logger.Error("cancel subscription failed", slog.String("sub_id", subID), slog.Any("error", err))
		b.answerCallback(ctx, cb.ID, "Failed to cancel subscription.")
		return
	}

	b.editMessageText(ctx, chatID, messageID, "Subscription cancelled.", nil)
	b.answerCallback(ctx, cb.ID, "Subscription cancelled.")
}

// callbackChatAndMessage extracts the chat ID and message ID from a callback
// query. Returns 0 values if the message is inaccessible.
func callbackChatAndMessage(cb *models.CallbackQuery) (int64, int) {
	if cb.Message.Message != nil {
		return cb.Message.Message.Chat.ID, cb.Message.Message.ID
	}
	return cb.From.ID, 0
}

// parseSelectedAddons attempts to extract the currently selected addon IDs from
// the confirm button in the callback query's message reply markup.
func parseSelectedAddons(cb *models.CallbackQuery) []string {
	if cb.Message.Message == nil || cb.Message.Message.ReplyMarkup == nil {
		return nil
	}
	for _, row := range cb.Message.Message.ReplyMarkup.InlineKeyboard {
		for _, btn := range row {
			if strings.HasPrefix(btn.CallbackData, CallbackPrefixConfirm) {
				data := strings.TrimPrefix(btn.CallbackData, CallbackPrefixConfirm)
				parts := strings.SplitN(data, ":", 2)
				if len(parts) > 1 && parts[1] != "" {
					return strings.Split(parts[1], ",")
				}
			}
		}
	}
	return nil
}

// containsAddon checks if addonID exists in the list.
func containsAddon(addons []string, addonID string) bool {
	for _, a := range addons {
		if a == addonID {
			return true
		}
	}
	return false
}

// removeAddon removes addonID from the list.
func removeAddon(addons []string, addonID string) []string {
	result := make([]string, 0, len(addons))
	for _, a := range addons {
		if a != addonID {
			result = append(result, a)
		}
	}
	return result
}
