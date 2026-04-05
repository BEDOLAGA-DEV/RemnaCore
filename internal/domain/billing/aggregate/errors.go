package aggregate

import "errors"

// Aggregate-level sentinel errors shared across billing aggregates and
// specifications. These are defined here so that specifications can reference
// them without creating circular imports with the parent billing package.

// ErrFamilyNotEnabled indicates the plan does not support family sharing.
var ErrFamilyNotEnabled = errors.New("family not enabled for this plan")

// ErrPlanNotActive indicates the plan is inactive and cannot be used for checkout.
var ErrPlanNotActive = errors.New("plan is not active")

// ErrNoPriceConfigured indicates the plan has no positive price configured.
var ErrNoPriceConfigured = errors.New("plan has no price configured")

// ErrAddonAlreadyOnSubscription indicates the addon is already present on the subscription.
var ErrAddonAlreadyOnSubscription = errors.New("addon already added to subscription")

// ErrAddonNotOnSubscription indicates the addon was not found on the subscription.
var ErrAddonNotOnSubscription = errors.New("addon not found on subscription")
