package vo

import "time"

// BillingInterval represents the recurrence of a billing cycle.
type BillingInterval string

const (
	IntervalMonth   BillingInterval = "month"
	IntervalQuarter BillingInterval = "quarter"
	IntervalYear    BillingInterval = "year"
)

// monthsPerInterval maps each interval to its duration in months.
const (
	monthsMonth   = 1
	monthsQuarter = 3
	monthsYear    = 12
)

// BillingPeriod represents a single billing cycle.
type BillingPeriod struct {
	Start    time.Time
	End      time.Time
	Interval BillingInterval
}

// NewBillingPeriod creates a BillingPeriod starting at start with the given
// interval. End is calculated by advancing start by the interval duration.
func NewBillingPeriod(start time.Time, interval BillingInterval) BillingPeriod {
	return BillingPeriod{
		Start:    start,
		End:      addInterval(start, interval),
		Interval: interval,
	}
}

// Contains reports whether t falls within the period [Start, End).
func (bp BillingPeriod) Contains(t time.Time) bool {
	return !t.Before(bp.Start) && t.Before(bp.End)
}

// DaysRemaining returns the number of whole days from now until the period end.
// Returns 0 if the period has already ended.
func (bp BillingPeriod) DaysRemaining() int {
	remaining := time.Until(bp.End)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

// Next returns the billing period that immediately follows this one, using the
// same interval.
func (bp BillingPeriod) Next() BillingPeriod {
	return NewBillingPeriod(bp.End, bp.Interval)
}

// addInterval advances t by the given billing interval.
func addInterval(t time.Time, interval BillingInterval) time.Time {
	switch interval {
	case IntervalMonth:
		return t.AddDate(0, monthsMonth, 0)
	case IntervalQuarter:
		return t.AddDate(0, monthsQuarter, 0)
	case IntervalYear:
		return t.AddDate(0, monthsYear, 0)
	default:
		// Fallback to monthly for unknown intervals.
		return t.AddDate(0, monthsMonth, 0)
	}
}
