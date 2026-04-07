package service

import (
	"log/slog"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

// lineItemQuantityOne is the standard quantity for plan and addon line items.
const lineItemQuantityOne = 1

// buildLineItems creates invoice line items from a plan and selected addon IDs.
// Addons referenced by the subscription but no longer available on the plan are
// skipped with a warning log. This can happen after plan migration or addon removal.
func buildLineItems(plan *aggregate.Plan, addonIDs []string, logger *slog.Logger) []vo.LineItem {
	items := []vo.LineItem{
		vo.NewLineItem(plan.Name, vo.LineItemPlan, plan.BasePrice, lineItemQuantityOne),
	}

	addonMap := make(map[string]aggregate.Addon, len(plan.AvailableAddons))
	for _, addon := range plan.AvailableAddons {
		addonMap[addon.ID] = addon
	}

	for _, addonID := range addonIDs {
		if addon, ok := addonMap[addonID]; ok {
			items = append(items, vo.NewLineItem(
				addon.Name,
				vo.LineItemAddon,
				addon.Price,
				lineItemQuantityOne,
			))
		} else {
			// Addon referenced by subscription but not available on plan.
			// This can happen after plan migration or addon removal.
			logger.Warn("addon not found on plan, skipping line item",
				slog.String("addon_id", addonID),
				slog.String("plan_id", plan.ID),
			)
		}
	}

	return items
}
