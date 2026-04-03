package service

import (
	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

// BindingSpec describes a single Remnawave binding that should exist for a subscription.
type BindingSpec struct {
	Purpose      aggregate.BindingPurpose
	TrafficLimit int64
	AllowedNodes []string
}

// BindingCalculator is a pure-logic service that determines which Remnawave
// bindings are needed for a given plan, addons, and family member set.
type BindingCalculator struct{}

// NewBindingCalculator creates a BindingCalculator.
func NewBindingCalculator() *BindingCalculator {
	return &BindingCalculator{}
}

// Calculate returns the list of binding specifications needed for a subscription
// based on the plan snapshot, active addon IDs, and family members.
func (bc *BindingCalculator) Calculate(
	plan multisubdomain.PlanSnapshot,
	addonIDs []string,
	familyMembers []string,
) []BindingSpec {
	specs := []BindingSpec{}

	// 1. Base binding (always present)
	base := BindingSpec{
		Purpose:      aggregate.PurposeBase,
		TrafficLimit: plan.TrafficLimitBytes,
	}

	// 2. Process addons
	for _, addonID := range addonIDs {
		addon := findAddon(plan, addonID)
		if addon == nil {
			continue
		}
		switch addon.Type {
		case multisubdomain.AddonSnapshotNodes:
			specs = append(specs, BindingSpec{
				Purpose:      purposeFromAddonName(addon.Name),
				TrafficLimit: addon.ExtraTrafficBytes,
				AllowedNodes: addon.ExtraNodes,
			})
		case multisubdomain.AddonSnapshotTraffic:
			// Traffic addons increase the base binding's traffic limit.
			base.TrafficLimit += addon.ExtraTrafficBytes
		}
	}

	// Prepend base as the first spec.
	specs = append([]BindingSpec{base}, specs...)

	// 3. Family member bindings
	for range familyMembers {
		specs = append(specs, BindingSpec{
			Purpose:      aggregate.PurposeFamilyMember,
			TrafficLimit: plan.TrafficLimitBytes,
		})
	}

	return specs
}

// findAddon locates an addon on a plan snapshot by ID, returning nil if not found.
func findAddon(plan multisubdomain.PlanSnapshot, addonID string) *multisubdomain.AddonSnapshot {
	for i := range plan.Addons {
		if plan.Addons[i].ID == addonID {
			return &plan.Addons[i]
		}
	}
	return nil
}

// addonNameToPurpose maps well-known addon names to their typed binding purposes.
var addonNameToPurpose = map[string]aggregate.BindingPurpose{
	string(aggregate.PurposeGaming):   aggregate.PurposeGaming,
	string(aggregate.PurposeStreaming): aggregate.PurposeStreaming,
}

// purposeFromAddonName maps well-known addon names to binding purposes.
// Unknown names default to the addon name itself as a BindingPurpose.
func purposeFromAddonName(name string) aggregate.BindingPurpose {
	if purpose, ok := addonNameToPurpose[name]; ok {
		return purpose
	}
	return aggregate.BindingPurpose(name)
}
