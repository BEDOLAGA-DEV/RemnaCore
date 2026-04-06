package service

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

// lineItemQuantityOne is the standard quantity for plan and addon line items.
const lineItemQuantityOne = 1

// buildLineItems creates invoice line items from a plan and selected addon IDs.
func buildLineItems(plan *aggregate.Plan, addonIDs []string) []vo.LineItem {
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
		}
	}

	return items
}
