package aggregate

import "slices"

// CheckoutEligibility validates all preconditions for starting a checkout
// against the given plan. Service methods call Check before proceeding with
// subscription creation to ensure business rules are satisfied up front.
type CheckoutEligibility struct {
	Plan                  *Plan
	HasActiveSubscription bool
	AllowedCountries      []string // empty = all allowed (from plan)
	UserCountry           string
}

// Check returns nil if all checkout preconditions are met, or the first
// violated constraint as an error.
func (ce CheckoutEligibility) Check() error {
	if !ce.Plan.IsActive {
		return ErrPlanNotActive
	}
	if !ce.Plan.BasePrice.IsPositive() {
		return ErrNoPriceConfigured
	}
	if ce.HasActiveSubscription {
		return ErrAlreadySubscribed
	}
	if len(ce.AllowedCountries) > 0 && !slices.Contains(ce.AllowedCountries, ce.UserCountry) {
		return ErrCountryNotAllowed
	}
	return nil
}

// FamilyEligibility validates that a plan supports family operations and
// that the current member count has not exceeded the maximum.
type FamilyEligibility struct {
	Plan        *Plan
	MemberCount int
}

// Check returns nil if family operations are permitted, or the first violated
// constraint as an error.
func (fe FamilyEligibility) Check() error {
	if !fe.Plan.FamilyEnabled {
		return ErrFamilyNotEnabled
	}
	if fe.MemberCount >= fe.Plan.MaxFamilyMembers {
		return ErrMaxFamilyExceeded
	}
	return nil
}

// AddonSpec validates addon operations against plan constraints.
// Built by service layer from the plan's available addons and max addons limit.
type AddonSpec struct {
	AvailableAddonIDs []string // addons available on the plan
	MaxAddons         int      // 0 = unlimited
}

// NewAddonSpec creates an AddonSpec from a Plan.
func NewAddonSpec(plan *Plan) AddonSpec {
	return AddonSpec{
		AvailableAddonIDs: plan.AvailableAddonIDs(),
		MaxAddons:         plan.MaxAddons,
	}
}

// CanAddAddon checks whether the addon can be added to the subscription.
// It validates the addon ID, uniqueness on the subscription, plan availability,
// and max addons limit.
func (s AddonSpec) CanAddAddon(sub *Subscription, addonID string) error {
	if addonID == "" {
		return ErrEmptyAddonID
	}
	if slices.Contains(sub.AddonIDs, addonID) {
		return ErrAddonAlreadyOnSubscription
	}
	if len(s.AvailableAddonIDs) > 0 && !slices.Contains(s.AvailableAddonIDs, addonID) {
		return ErrAddonNotAvailableOnPlan
	}
	if s.MaxAddons > 0 && len(sub.AddonIDs) >= s.MaxAddons {
		return ErrMaxAddonsExceeded
	}
	return nil
}

// UpgradeSpec validates plan change compatibility.
type UpgradeSpec struct {
	FromPlanTier PlanTier
	ToPlanTier   PlanTier
	ToPlanActive bool
}

// CanUpgrade checks whether the subscription can be upgraded to the target plan.
func (s UpgradeSpec) CanUpgrade() error {
	if !s.ToPlanActive {
		return ErrPendingPlanInactive
	}
	return nil
}
