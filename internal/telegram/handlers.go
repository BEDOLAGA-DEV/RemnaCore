package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// handleStart handles the /start command.
// If the Telegram user is linked to a platform account, it shows a welcome
// message. Otherwise, it prompts the user to register or link their account.
func (b *Bot) handleStart(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	tgUser := update.Message.From
	if tgUser == nil {
		b.sendText(ctx, chatID, "Could not identify you. Please try again.")
		return
	}

	displayName := tgUser.FirstName
	if tgUser.LastName != "" {
		displayName += " " + tgUser.LastName
	}

	// Check if the user has a linked platform account.
	user, err := b.identity.GetByTelegramID(ctx, tgUser.ID)
	if err != nil || user == nil {
		msg := fmt.Sprintf(
			"Welcome, %s!\n\nTo get started, register at %s and link your Telegram account.\n\nUse /plans to see available VPN plans.",
			displayName, b.cabinetURL,
		)
		b.sendText(ctx, chatID, msg)
		return
	}

	msg := fmt.Sprintf(
		"Welcome back, %s!\n\nUse /my to see your subscriptions or /plans to browse available plans.",
		displayName,
	)
	b.sendText(ctx, chatID, msg)
}

// handlePlans handles the /plans command by listing all active plans.
func (b *Bot) handlePlans(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID

	plans, err := b.plans.GetActive(ctx)
	if err != nil {
		b.logger.Error("failed to get plans", slog.Any("error", err))
		b.sendText(ctx, chatID, "Failed to load plans. Please try again later.")
		return
	}

	if len(plans) == 0 {
		b.sendText(ctx, chatID, "No plans available at the moment.")
		return
	}

	b.sendTextWithKeyboard(ctx, chatID, "Available plans:", PlansKeyboard(plans))
}

// handleSubscribe handles the /subscribe command. It expects a plan ID as an
// argument, e.g. /subscribe <planID>.
func (b *Bot) handleSubscribe(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID

	parts := strings.Fields(update.Message.Text)
	if len(parts) < 2 {
		b.sendText(ctx, chatID, "Usage: /subscribe <plan_id>\n\nUse /plans to see available plans.")
		return
	}
	planID := parts[1]

	plan, err := b.plans.GetByID(ctx, planID)
	if err != nil {
		b.logger.Error("failed to get plan", slog.String("plan_id", planID), slog.Any("error", err))
		b.sendText(ctx, chatID, "Plan not found. Use /plans to see available plans.")
		return
	}

	text := FormatPlanDetail(plan)
	if len(plan.AvailableAddons) > 0 {
		text += "\nSelect addons or confirm purchase:"
		b.sendTextWithKeyboard(ctx, chatID, text, AddonsKeyboard(plan, nil))
	} else {
		text += "\nConfirm purchase:"
		b.sendTextWithKeyboard(ctx, chatID, text, ConfirmPurchaseKeyboard(plan.ID, nil))
	}
}

// handleMy handles the /my command showing the user's active subscriptions.
func (b *Bot) handleMy(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	tgUser := update.Message.From
	if tgUser == nil {
		b.sendText(ctx, chatID, "Could not identify you.")
		return
	}

	user, err := b.identity.GetByTelegramID(ctx, tgUser.ID)
	if err != nil || user == nil {
		b.sendText(ctx, chatID, "Your Telegram account is not linked. Please register at "+b.cabinetURL)
		return
	}

	subs, err := b.subs.GetByUserID(ctx, user.ID)
	if err != nil {
		b.logger.Error("failed to get subscriptions", slog.Any("error", err))
		b.sendText(ctx, chatID, "Failed to load subscriptions. Please try again later.")
		return
	}

	if len(subs) == 0 {
		b.sendText(ctx, chatID, "You have no subscriptions yet. Use /plans to get started.")
		return
	}

	for _, sub := range subs {
		planName := sub.PlanID
		plan, planErr := b.plans.GetByID(ctx, sub.PlanID)
		if planErr == nil {
			planName = plan.Name
		}

		bindings, bindErr := b.bindings.GetBySubscriptionID(ctx, sub.ID)
		if bindErr != nil {
			b.logger.Error("failed to get bindings", slog.String("sub_id", sub.ID), slog.Any("error", bindErr))
		}

		text := FormatSubscription(sub, planName, bindings)
		b.sendTextWithKeyboard(ctx, chatID, text, SubscriptionKeyboard(sub, bindings))
	}
}

// handleTraffic handles the /traffic command showing usage per binding.
func (b *Bot) handleTraffic(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	tgUser := update.Message.From
	if tgUser == nil {
		b.sendText(ctx, chatID, "Could not identify you.")
		return
	}

	user, err := b.identity.GetByTelegramID(ctx, tgUser.ID)
	if err != nil || user == nil {
		b.sendText(ctx, chatID, "Your Telegram account is not linked. Please register at "+b.cabinetURL)
		return
	}

	bindings, err := b.bindings.GetByPlatformUserID(ctx, user.ID)
	if err != nil {
		b.logger.Error("failed to get bindings", slog.Any("error", err))
		b.sendText(ctx, chatID, "Failed to load traffic data. Please try again later.")
		return
	}

	b.sendText(ctx, chatID, FormatTrafficUsage(bindings))
}

// handleSupport handles the /support command.
func (b *Bot) handleSupport(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	b.sendText(ctx, chatID, "For support, please contact us via the web cabinet or email support@remnacore.com.")
}

// handleReferral handles the /referral command (placeholder for Phase 8).
func (b *Bot) handleReferral(ctx context.Context, _ *tgbot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}
	chatID := update.Message.Chat.ID
	b.sendText(ctx, chatID, "Referral program coming soon! Stay tuned.")
}
