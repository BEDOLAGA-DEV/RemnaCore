package service

import (
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

// ProrateCalculator computes prorated credits and costs for plan changes.
type ProrateCalculator struct{}

// NewProrateCalculator creates a new ProrateCalculator.
func NewProrateCalculator() *ProrateCalculator {
	return &ProrateCalculator{}
}

// CalculateUpgradeCredit returns the unused-portion credit for the current plan.
// Uses (amount * daysRemaining) / totalDays — multiply before divide to
// minimise integer truncation on small amounts.
func (pc *ProrateCalculator) CalculateUpgradeCredit(
	currentPlan *aggregate.Plan,
	currentPeriod vo.BillingPeriod,
	now time.Time,
) vo.Money {
	totalDays := totalDaysInPeriod(currentPeriod)
	if totalDays == 0 {
		return vo.Zero(currentPlan.BasePrice.Currency)
	}

	daysRemaining := currentPeriod.DaysRemaining(now)
	// Multiply first, divide last — preserves precision for small amounts.
	creditAmount := (currentPlan.BasePrice.Amount * int64(daysRemaining)) / int64(totalDays)

	return vo.NewMoney(creditAmount, currentPlan.BasePrice.Currency)
}

// CalculateUpgradeCost returns the prorated cost of the new plan minus any credit.
// cost = (newPrice * daysRemaining) / totalDays - credit, floored at 0.
// Returns ErrCurrencyMismatch if currencies differ.
func (pc *ProrateCalculator) CalculateUpgradeCost(
	newPlan *aggregate.Plan,
	currentPeriod vo.BillingPeriod,
	now time.Time,
	credit vo.Money,
) (vo.Money, error) {
	if newPlan.BasePrice.Currency != credit.Currency {
		return vo.Money{}, billing.ErrCurrencyMismatch
	}

	totalDays := totalDaysInPeriod(currentPeriod)
	if totalDays == 0 {
		return vo.Zero(newPlan.BasePrice.Currency), nil
	}

	daysRemaining := currentPeriod.DaysRemaining(now)
	// Multiply first, divide last — preserves precision for small amounts.
	proratedAmount := (newPlan.BasePrice.Amount * int64(daysRemaining)) / int64(totalDays)

	cost := max(proratedAmount-credit.Amount, 0)

	return vo.NewMoney(cost, newPlan.BasePrice.Currency), nil
}

// totalDaysInPeriod returns the whole number of days in a billing period.
func totalDaysInPeriod(period vo.BillingPeriod) int {
	hours := period.End.Sub(period.Start).Hours()
	return int(hours / vo.HoursPerDay)
}
