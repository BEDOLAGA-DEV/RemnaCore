package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func testPlan(price int64, currency vo.Currency) *aggregate.Plan {
	return &aggregate.Plan{
		ID:        "plan-1",
		Name:      "Test Plan",
		BasePrice: vo.NewMoney(price, currency),
		Interval:  vo.IntervalMonth,
		IsActive:  true,
	}
}

func TestProrateCalculator_CalculateUpgradeCredit_FullPeriod(t *testing.T) {
	pc := NewProrateCalculator()
	plan := testPlan(3000, vo.CurrencyUSD) // $30.00

	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	period := vo.BillingPeriod{
		Start:    now,
		End:      now.AddDate(0, 1, 0), // 30 days
		Interval: vo.IntervalMonth,
	}

	// At the very start of the period, all days remaining = full credit
	credit := pc.CalculateUpgradeCredit(plan, period, now)

	assert.Equal(t, vo.CurrencyUSD, credit.Currency)
	assert.Equal(t, plan.BasePrice.Amount, credit.Amount)
}

func TestProrateCalculator_CalculateUpgradeCredit_HalfPeriod(t *testing.T) {
	pc := NewProrateCalculator()
	plan := testPlan(3000, vo.CurrencyUSD) // $30.00

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) // 30 days
	period := vo.BillingPeriod{
		Start:    start,
		End:      end,
		Interval: vo.IntervalMonth,
	}

	// 15 days into a 30-day period = half credit
	now := start.AddDate(0, 0, 15)
	credit := pc.CalculateUpgradeCredit(plan, period, now)

	assert.Equal(t, vo.CurrencyUSD, credit.Currency)
	// 3000 * 15 / 30 = 1500
	assert.Equal(t, int64(1500), credit.Amount)
}

func TestProrateCalculator_CalculateUpgradeCost_MinusCredit(t *testing.T) {
	pc := NewProrateCalculator()
	newPlan := testPlan(6000, vo.CurrencyUSD) // $60.00

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) // 30 days
	period := vo.BillingPeriod{
		Start:    start,
		End:      end,
		Interval: vo.IntervalMonth,
	}

	now := start.AddDate(0, 0, 15) // 15 days remaining
	credit := vo.NewMoney(1500, vo.CurrencyUSD)

	cost, err := pc.CalculateUpgradeCost(newPlan, period, now, credit)

	require.NoError(t, err)
	// newPlan prorated = 6000 * 15/30 = 3000
	// cost = 3000 - 1500 = 1500
	assert.Equal(t, int64(1500), cost.Amount)
	assert.Equal(t, vo.CurrencyUSD, cost.Currency)
}

func TestProrateCalculator_CalculateUpgradeCost_FloorAtZero(t *testing.T) {
	pc := NewProrateCalculator()
	newPlan := testPlan(1000, vo.CurrencyUSD) // $10.00

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) // 30 days
	period := vo.BillingPeriod{
		Start:    start,
		End:      end,
		Interval: vo.IntervalMonth,
	}

	now := start.AddDate(0, 0, 15) // 15 days remaining
	credit := vo.NewMoney(5000, vo.CurrencyUSD)

	cost, err := pc.CalculateUpgradeCost(newPlan, period, now, credit)

	require.NoError(t, err)
	// newPlan prorated = 1000 * 15/30 = 500
	// cost = 500 - 5000 = -4500 -> floored at 0
	assert.Equal(t, int64(0), cost.Amount)
}

func TestProrateCalculator_CalculateUpgradeCost_CurrencyMismatch(t *testing.T) {
	pc := NewProrateCalculator()
	newPlan := testPlan(6000, vo.CurrencyUSD)

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	period := vo.BillingPeriod{
		Start:    start,
		End:      start.AddDate(0, 1, 0),
		Interval: vo.IntervalMonth,
	}

	credit := vo.NewMoney(1500, vo.CurrencyEUR)

	_, err := pc.CalculateUpgradeCost(newPlan, period, start, credit)

	require.Error(t, err)
	assert.ErrorIs(t, err, billing.ErrCurrencyMismatch)
}

func TestProrateCalculator_CalculateUpgradeCredit_ExpiredPeriod(t *testing.T) {
	pc := NewProrateCalculator()
	plan := testPlan(3000, vo.CurrencyUSD)

	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	period := vo.BillingPeriod{
		Start:    start,
		End:      end,
		Interval: vo.IntervalMonth,
	}

	// now is after the period end
	now := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	credit := pc.CalculateUpgradeCredit(plan, period, now)

	assert.Equal(t, int64(0), credit.Amount)
}
