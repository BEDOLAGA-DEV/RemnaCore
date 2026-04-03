package telegram

import (
	"fmt"
	"strings"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	msaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

// FormatPlanDetail returns an HTML-formatted description of a plan.
func FormatPlanDetail(plan *aggregate.Plan) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>%s</b>\n", plan.Name))
	if plan.Description != "" {
		sb.WriteString(plan.Description + "\n")
	}
	sb.WriteString(fmt.Sprintf("Price: %s / %s\n", plan.BasePrice.String(), string(plan.Interval)))
	sb.WriteString(fmt.Sprintf("Tier: %s\n", string(plan.Tier)))
	if plan.TrafficLimitBytes > 0 {
		sb.WriteString(fmt.Sprintf("Traffic: %s\n", formatBytes(plan.TrafficLimitBytes)))
	} else {
		sb.WriteString("Traffic: unlimited\n")
	}
	sb.WriteString(fmt.Sprintf("Devices: %d\n", plan.DeviceLimit))
	sb.WriteString(fmt.Sprintf("Countries: %s\n", strings.Join(plan.AllowedCountries, ", ")))
	if plan.FamilyEnabled {
		sb.WriteString(fmt.Sprintf("Family: up to %d members\n", plan.MaxFamilyMembers))
	}
	if len(plan.AvailableAddons) > 0 {
		sb.WriteString("\n<b>Available addons:</b>\n")
		for _, a := range plan.AvailableAddons {
			sb.WriteString(fmt.Sprintf("  - %s (%s): +%s\n", a.Name, string(a.Type), a.Price.String()))
		}
	}
	return sb.String()
}

// FormatSubscription returns a text summary of a subscription with its bindings.
func FormatSubscription(sub *aggregate.Subscription, planName string, bindings []*msaggregate.RemnawaveBinding) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<b>%s</b> [%s]\n", planName, string(sub.Status)))
	sb.WriteString(fmt.Sprintf("Period: %s - %s\n",
		sub.Period.Start.Format("2006-01-02"),
		sub.Period.End.Format("2006-01-02"),
	))
	if len(sub.AddonIDs) > 0 {
		sb.WriteString(fmt.Sprintf("Addons: %s\n", strings.Join(sub.AddonIDs, ", ")))
	}
	if len(bindings) > 0 {
		sb.WriteString("\n<b>Links:</b>\n")
		for _, b := range bindings {
			status := string(b.Status)
			sb.WriteString(fmt.Sprintf("  - %s (%s) [%s]\n", b.RemnawaveUsername, b.Purpose, status))
			if b.RemnawaveShortUUID != "" {
				sb.WriteString(fmt.Sprintf("    ShortUUID: <code>%s</code>\n", b.RemnawaveShortUUID))
			}
		}
	}
	return sb.String()
}

// FormatTrafficUsage returns a text summary of traffic usage per binding.
func FormatTrafficUsage(bindings []*msaggregate.RemnawaveBinding) string {
	if len(bindings) == 0 {
		return "No active bindings found."
	}

	var sb strings.Builder
	sb.WriteString("<b>Traffic usage:</b>\n\n")
	for _, b := range bindings {
		limit := "unlimited"
		if b.TrafficLimitBytes > 0 {
			limit = formatBytes(b.TrafficLimitBytes)
		}
		sb.WriteString(fmt.Sprintf("%s (%s): limit %s [%s]\n",
			b.RemnawaveUsername, b.Purpose, limit, string(b.Status),
		))
	}
	return sb.String()
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
