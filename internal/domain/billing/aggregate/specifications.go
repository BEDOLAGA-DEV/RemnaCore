package aggregate

// CheckoutEligibility validates all preconditions for starting a checkout
// against the given plan. Service methods call Check before proceeding with
// subscription creation to ensure business rules are satisfied up front.
type CheckoutEligibility struct {
	Plan *Plan
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

