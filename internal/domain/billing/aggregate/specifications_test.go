package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

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
	sub, err := NewSubscription("user-1", "plan-1", vo.IntervalMonth, nil, now)
	require.NoError(t, err)

	// NewSubscription now records its own creation event.
	assert.True(t, sub.HasEvents())

	events := sub.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventSubCreated, events[0].Type)
	assert.False(t, sub.HasEvents())

	// Subsequent mutations also record events.
	require.NoError(t, sub.Activate(time.Now()))
	assert.True(t, sub.HasEvents())

	events = sub.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventSubActivated, events[0].Type)
	assert.False(t, sub.HasEvents())
}

func TestFamilyGroup_EventRecorder_Embedded(t *testing.T) {
	fg := NewFamilyGroup("owner-1", 5, time.Now())

	assert.False(t, fg.HasEvents())

	// AddMember now records its own event.
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.True(t, fg.HasEvents())

	events := fg.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventFamilyMemberAdded, events[0].Type)
}
