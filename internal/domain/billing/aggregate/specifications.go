package aggregate

import "fmt"

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
		return fmt.Errorf("plan %s is not active", ce.Plan.ID)
	}
	if !ce.Plan.BasePrice.IsPositive() {
		return fmt.Errorf("plan %s has no price configured", ce.Plan.ID)
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

// BindingEligibility validates that the subscription has not exceeded the
// maximum number of Remnawave bindings allowed by the plan.
type BindingEligibility struct {
	Plan            *Plan
	CurrentBindings int
}

// Check returns nil if a new binding can be created, or an error if the limit
// has been reached.
func (be BindingEligibility) Check() error {
	if be.CurrentBindings >= be.Plan.MaxRemnawaveBindings {
		return ErrMaxBindingsExceeded
	}
	return nil
}
