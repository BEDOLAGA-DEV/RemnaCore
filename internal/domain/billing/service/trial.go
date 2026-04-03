package service

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// DefaultTrialDays is the default number of days for a trial period.
const DefaultTrialDays = 7

// TrialManager handles trial-related logic for subscriptions.
type TrialManager struct {
	trialDays int
	clock     clock.Clock
}

// NewTrialManager creates a TrialManager with the specified trial duration.
// If trialDays is zero or negative, DefaultTrialDays is used.
func NewTrialManager(trialDays int) *TrialManager {
	return NewTrialManagerWithClock(trialDays, clock.NewReal())
}

// NewTrialManagerWithClock creates a TrialManager with an explicit clock for
// deterministic testing.
func NewTrialManagerWithClock(trialDays int, c clock.Clock) *TrialManager {
	if trialDays <= 0 {
		trialDays = DefaultTrialDays
	}
	return &TrialManager{trialDays: trialDays, clock: c}
}

// TrialDays returns the configured number of trial days.
func (tm *TrialManager) TrialDays() int {
	return tm.trialDays
}

// StartTrial sets the subscription status to trial and configures the period
// from now to now + trialDays. The subscription must be in trial status
// (as created by NewSubscription).
func (tm *TrialManager) StartTrial(sub *aggregate.Subscription) error {
	if sub.Status != aggregate.StatusTrial {
		return billing.ErrNotTrialStatus
	}

	now := tm.clock.Now()
	sub.Period = vo.BillingPeriod{
		Start:    now,
		End:      now.AddDate(0, 0, tm.trialDays),
		Interval: sub.Period.Interval,
	}
	sub.UpdatedAt = now

	return nil
}

// IsTrialExpiring reports whether the subscription trial ends within warningDays.
// Returns false if the subscription is not in trial status.
func (tm *TrialManager) IsTrialExpiring(sub *aggregate.Subscription, warningDays int) bool {
	if sub.Status != aggregate.StatusTrial {
		return false
	}

	deadline := tm.clock.Now().AddDate(0, 0, warningDays)
	return sub.Period.End.Before(deadline) || sub.Period.End.Equal(deadline)
}

// IsTrialExpired reports whether the trial period has ended.
// Returns false if the subscription is not in trial status.
func (tm *TrialManager) IsTrialExpired(sub *aggregate.Subscription) bool {
	if sub.Status != aggregate.StatusTrial {
		return false
	}

	return tm.clock.Now().After(sub.Period.End)
}
