package vo

import "time"

// PendingPlanChange records a deferred plan change (downgrade) that will
// be applied at the next renewal. Captures full context for auditability.
// This is an immutable value object — zero value means no pending change.
type PendingPlanChange struct {
	PlanID         string    // target plan to switch to
	OriginalPlanID string    // plan at the time of request
	RequestedAt    time.Time // when the downgrade was requested
}

// IsZero returns true if no pending change exists.
func (p PendingPlanChange) IsZero() bool {
	return p.PlanID == ""
}
