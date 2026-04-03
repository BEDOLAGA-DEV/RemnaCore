package nats

import (
	"context"
	"fmt"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
)

// BillingSubscriptionLookup implements SubscriptionLookup by delegating to the
// billing domain repositories. It bridges the NATS consumer's enrichment needs
// with the billing bounded context.
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

// GetPlanByID fetches a plan by ID.
func (l *BillingSubscriptionLookup) GetPlanByID(ctx context.Context, id string) (*billingaggregate.Plan, error) {
	return l.plans.GetByID(ctx, id)
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

// compile-time interface check
var _ SubscriptionLookup = (*BillingSubscriptionLookup)(nil)
