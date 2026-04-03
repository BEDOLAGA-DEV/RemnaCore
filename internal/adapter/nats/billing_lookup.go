package nats

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
)

// BillingSubscriptionLookup implements SubscriptionLookup by delegating to the
// billing domain repositories. It bridges the NATS consumer's enrichment needs
// with the billing bounded context and serves as the Anti-Corruption Layer that
// translates billing types into multisub PlanSnapshot values.
type BillingSubscriptionLookup struct {
	subs     billing.SubscriptionRepository
	plans    billing.PlanRepository
	families billing.FamilyRepository
}

// NewBillingSubscriptionLookup creates a BillingSubscriptionLookup with the
// given billing repositories.
func NewBillingSubscriptionLookup(
	subs billing.SubscriptionRepository,
	plans billing.PlanRepository,
	families billing.FamilyRepository,
) *BillingSubscriptionLookup {
	return &BillingSubscriptionLookup{
		subs:     subs,
		plans:    plans,
		families: families,
	}
}

// GetSubscriptionByID fetches minimal subscription data for event enrichment.
func (l *BillingSubscriptionLookup) GetSubscriptionByID(ctx context.Context, id string) (SubscriptionInfo, error) {
	sub, err := l.subs.GetByID(ctx, id)
	if err != nil {
		return SubscriptionInfo{}, fmt.Errorf("get subscription: %w", err)
	}

	return SubscriptionInfo{
		ID:       sub.ID,
		UserID:   sub.UserID,
		PlanID:   sub.PlanID,
		AddonIDs: sub.AddonIDs,
	}, nil
}

// GetPlanSnapshot fetches a billing plan by ID and translates it into the
// multisub Anti-Corruption Layer type. This is the boundary where billing
// domain types are converted into multisub-local types.
func (l *BillingSubscriptionLookup) GetPlanSnapshot(ctx context.Context, id string) (multisub.PlanSnapshot, error) {
	plan, err := l.plans.GetByID(ctx, id)
	if err != nil {
		return multisub.PlanSnapshot{}, fmt.Errorf("get plan: %w", err)
	}

	return planToSnapshot(plan), nil
}

// GetFamilyMemberIDs fetches the user IDs of family members for the given
// owner. Returns nil (not an error) if no family group exists for the owner.
func (l *BillingSubscriptionLookup) GetFamilyMemberIDs(ctx context.Context, ownerID string) ([]string, error) {
	fg, err := l.families.GetByOwnerID(ctx, ownerID)
	if err != nil {
		// No family group is a normal case (not all plans have family sharing).
		return nil, fmt.Errorf("get family group: %w", err)
	}

	memberIDs := make([]string, 0, len(fg.Members))
	for _, m := range fg.Members {
		// Exclude the owner from the family member list since the owner
		// already has a primary binding.
		if m.UserID != ownerID {
			memberIDs = append(memberIDs, m.UserID)
		}
	}

	return memberIDs, nil
}

// planToSnapshot translates a billing Plan aggregate into a multisub
// PlanSnapshot. This is the Anti-Corruption Layer translation point: all
// billing-specific types (AddonType, Money, etc.) are mapped to multisub-local
// equivalents so the multisub domain never imports billing types.
func planToSnapshot(plan *billingaggregate.Plan) multisub.PlanSnapshot {
	addons := make([]multisub.AddonSnapshot, len(plan.AvailableAddons))
	for i, a := range plan.AvailableAddons {
		addons[i] = multisub.AddonSnapshot{
			ID:                a.ID,
			Name:              a.Name,
			Type:              addonTypeToSnapshot(a.Type),
			ExtraTrafficBytes: a.ExtraTrafficBytes,
			ExtraNodes:        a.ExtraNodes,
		}
	}
	return multisub.PlanSnapshot{
		ID:                   plan.ID,
		TrafficLimitBytes:    plan.TrafficLimitBytes,
		MaxRemnawaveBindings: plan.MaxRemnawaveBindings,
		Addons:               addons,
	}
}

// addonTypeToSnapshot maps billing AddonType to multisub AddonSnapshotType.
var addonTypeMap = map[billingaggregate.AddonType]multisub.AddonSnapshotType{
	billingaggregate.AddonTraffic: multisub.AddonSnapshotTraffic,
	billingaggregate.AddonNodes:   multisub.AddonSnapshotNodes,
	billingaggregate.AddonFeature: multisub.AddonSnapshotFeature,
}

// addonTypeToSnapshot translates a billing AddonType to the multisub
// AddonSnapshotType. Unknown types default to the string value of the billing
// type to avoid data loss.
func addonTypeToSnapshot(t billingaggregate.AddonType) multisub.AddonSnapshotType {
	if mapped, ok := addonTypeMap[t]; ok {
		return mapped
	}
	return multisub.AddonSnapshotType(t)
}

// compile-time interface check
var _ SubscriptionLookup = (*BillingSubscriptionLookup)(nil)
