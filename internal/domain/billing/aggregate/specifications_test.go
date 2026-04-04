package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// dummyEvent creates a minimal event for testing the EventRecorder embedding.
func dummyEvent(eventType string) domainevent.Event {
	return domainevent.NewAt(domainevent.EventType(eventType), nil, time.Now())
}

func TestCheckoutEligibility_ActivePlanWithPrice(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  true,
		BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{Plan: plan}).Check()

	require.NoError(t, err)
}

func TestCheckoutEligibility_InactivePlan(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  false,
		BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{Plan: plan}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPlanNotActive)
}

func TestCheckoutEligibility_ZeroPrice(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  true,
		BasePrice: vo.NewMoney(0, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{Plan: plan}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoPriceConfigured)
}

func TestFamilyEligibility_Enabled_UnderLimit(t *testing.T) {
	plan := &Plan{
		ID:               "plan-1",
		FamilyEnabled:    true,
		MaxFamilyMembers: 5,
	}

	err := (FamilyEligibility{Plan: plan, MemberCount: 3}).Check()

	require.NoError(t, err)
}

func TestFamilyEligibility_Disabled(t *testing.T) {
	plan := &Plan{
		ID:            "plan-1",
		FamilyEnabled: false,
	}

	err := (FamilyEligibility{Plan: plan, MemberCount: 0}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFamilyNotEnabled)
}

func TestFamilyEligibility_AtLimit(t *testing.T) {
	plan := &Plan{
		ID:               "plan-1",
		FamilyEnabled:    true,
		MaxFamilyMembers: 5,
	}

	err := (FamilyEligibility{Plan: plan, MemberCount: 5}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxFamilyExceeded)
}

func TestFamilyEligibility_OverLimit(t *testing.T) {
	plan := &Plan{
		ID:               "plan-1",
		FamilyEnabled:    true,
		MaxFamilyMembers: 5,
	}

	err := (FamilyEligibility{Plan: plan, MemberCount: 6}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxFamilyExceeded)
}

func TestSubscription_EventRecorder_Embedded(t *testing.T) {
	now := time.Now()
	sub := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, now)

	assert.False(t, sub.HasEvents())

	// Simulate what the service does: record an event on the aggregate.
	sub.RecordEvent(dummyEvent("subscription.created"))

	assert.True(t, sub.HasEvents())

	events := sub.DomainEvents()
	require.Len(t, events, 1)
	assert.False(t, sub.HasEvents())
}

func TestFamilyGroup_EventRecorder_Embedded(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	assert.False(t, fg.HasEvents())

	fg.RecordEvent(dummyEvent("family.member_added"))
	assert.True(t, fg.HasEvents())
}
