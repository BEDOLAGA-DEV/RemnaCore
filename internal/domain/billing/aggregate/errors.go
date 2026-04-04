package aggregate

import "errors"

// Aggregate-level sentinel errors shared across billing aggregates and
// specifications. These are defined here so that specifications can reference
// them without creating circular imports with the parent billing package.

// ErrFamilyNotEnabled indicates the plan does not support family sharing.
var ErrFamilyNotEnabled = errors.New("family not enabled for this plan")

// ErrMaxBindingsExceeded indicates the subscription has reached its maximum
// number of Remnawave bindings.
var ErrMaxBindingsExceeded = errors.New("maximum remnawave bindings exceeded")
