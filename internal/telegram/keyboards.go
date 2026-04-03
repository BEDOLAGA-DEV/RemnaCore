package telegram

import (
	"fmt"
	"strings"

	"github.com/go-telegram/bot/models"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	msaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

// PlansKeyboard builds an inline keyboard with one button per plan.
func PlansKeyboard(plans []*aggregate.Plan) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	for _, p := range plans {
		label := fmt.Sprintf("%s - %s/%s", p.Name, p.BasePrice.String(), string(p.Interval))
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: label, CallbackData: CallbackPrefixPlan + p.ID},
		})
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// AddonsKeyboard builds an inline keyboard for addon selection during subscribe
// flow. Selected addons are marked with a checkmark.
func AddonsKeyboard(plan *aggregate.Plan, selectedAddons []string) *models.InlineKeyboardMarkup {
	selected := make(map[string]bool, len(selectedAddons))
	for _, id := range selectedAddons {
		selected[id] = true
	}

	var rows [][]models.InlineKeyboardButton
	for _, addon := range plan.AvailableAddons {
		prefix := ""
		if selected[addon.ID] {
			prefix = "[x] "
		}
		label := fmt.Sprintf("%s%s (+%s)", prefix, addon.Name, addon.Price.String())
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: label, CallbackData: fmt.Sprintf("%s%s:%s", CallbackPrefixAddon, plan.ID, addon.ID)},
		})
	}

	// Confirm button
	addonList := strings.Join(selectedAddons, ",")
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "Confirm purchase", CallbackData: fmt.Sprintf("%s%s:%s", CallbackPrefixConfirm, plan.ID, addonList)},
	})

	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// ConfirmPurchaseKeyboard builds a confirmation keyboard for checkout.
func ConfirmPurchaseKeyboard(planID string, addonIDs []string) *models.InlineKeyboardMarkup {
	addonList := strings.Join(addonIDs, ",")
	return &models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "Confirm", CallbackData: fmt.Sprintf("%s%s:%s", CallbackPrefixConfirm, planID, addonList)},
				{Text: "Cancel", CallbackData: CallbackPrefixCancel + "checkout"},
			},
		},
	}
}

// SubscriptionKeyboard builds an inline keyboard for a single subscription
// showing its bindings and a cancel option.
func SubscriptionKeyboard(sub *aggregate.Subscription, bindings []*msaggregate.RemnawaveBinding) *models.InlineKeyboardMarkup {
	var rows [][]models.InlineKeyboardButton
	for _, b := range bindings {
		label := fmt.Sprintf("%s [%s]", b.RemnawaveUsername, b.Purpose)
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: label, CallbackData: "binding:" + b.ID},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "Cancel subscription", CallbackData: CallbackPrefixCancel + sub.ID},
	})
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}
