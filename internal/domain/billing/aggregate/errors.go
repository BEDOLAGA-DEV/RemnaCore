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

// ErrPeriodNotElapsed indicates a renewal attempt before the current billing period has ended.
var ErrPeriodNotElapsed = errors.New("current billing period has not elapsed")

// ErrInvoiceNotDraft indicates the invoice must be in draft status to perform this operation.
var ErrInvoiceNotDraft = errors.New("invoice must be in draft status")

// ErrNegativeTotal indicates the invoice total must not be negative.
var ErrNegativeTotal = errors.New("invoice total must not be negative")

// ErrSamePlan indicates an attempt to upgrade or downgrade to the current plan.
var ErrSamePlan = errors.New("cannot change to the same plan")

// ErrPendingPlanInactive indicates the pending plan (from a deferred downgrade)
// is no longer active and cannot be applied during renewal.
var ErrPendingPlanInactive = errors.New("pending plan is no longer active")

// ErrSubscriptionNotActiveForInvoicing indicates the subscription is in a
// terminal state (cancelled or expired) and cannot be invoiced.
var ErrSubscriptionNotActiveForInvoicing = errors.New("subscription is not active for invoicing")

// ErrEmptyAddonID indicates that an addon ID was not provided.
var ErrEmptyAddonID = errors.New("addon ID must not be empty")

// ErrAddonNotAvailableOnPlan indicates the addon is not available on the subscription's plan.
var ErrAddonNotAvailableOnPlan = errors.New("addon is not available on this plan")

// ErrMaxAddonsExceeded indicates the subscription has reached the plan's addon limit.
var ErrMaxAddonsExceeded = errors.New("maximum number of addons exceeded")

// ErrSubscriptionNotActiveForAddon indicates the subscription must be active or trial
// to add an addon.
var ErrSubscriptionNotActiveForAddon = errors.New("subscription must be active or trial to modify addons")

// ErrAlreadySubscribed indicates the user already has an active subscription.
var ErrAlreadySubscribed = errors.New("user already has an active subscription")

// ErrCountryNotAllowed indicates the plan is not available in the user's country.
var ErrCountryNotAllowed = errors.New("plan not available in user's country")
