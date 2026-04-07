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

// --- CheckoutEligibility: new fields ---

func TestCheckoutEligibility_AlreadySubscribed(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  true,
		BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{
		Plan:                  plan,
		HasActiveSubscription: true,
	}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAlreadySubscribed)
}

func TestCheckoutEligibility_CountryNotAllowed(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  true,
		BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{
		Plan:             plan,
		AllowedCountries: []string{"US", "DE"},
		UserCountry:      "RU",
	}).Check()

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCountryNotAllowed)
}

func TestCheckoutEligibility_CountryAllowed(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  true,
		BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{
		Plan:             plan,
		AllowedCountries: []string{"US", "DE"},
		UserCountry:      "US",
	}).Check()

	require.NoError(t, err)
}

func TestCheckoutEligibility_EmptyCountries_AllAllowed(t *testing.T) {
	plan := &Plan{
		ID:        "plan-1",
		IsActive:  true,
		BasePrice: vo.NewMoney(999, vo.CurrencyUSD),
	}

	err := (CheckoutEligibility{
		Plan:        plan,
		UserCountry: "JP",
	}).Check()

	require.NoError(t, err)
}

// --- AddonSpec ---

func TestAddonSpec_CanAddAddon(t *testing.T) {
	t.Run("success with valid addon", func(t *testing.T) {
		sub := &Subscription{
			ID:       "sub-1",
			UserID:   "user-1",
			Status:   StatusActive,
			AddonIDs: nil,
		}
		spec := AddonSpec{
			AvailableAddonIDs: []string{"addon-1", "addon-2"},
			MaxAddons:         3,
		}

		err := spec.CanAddAddon(sub, "addon-1")

		require.NoError(t, err)
	})

	t.Run("empty addon ID", func(t *testing.T) {
		sub := &Subscription{ID: "sub-1", Status: StatusActive}
		spec := AddonSpec{}

		err := spec.CanAddAddon(sub, "")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrEmptyAddonID)
	})

	t.Run("addon already on subscription", func(t *testing.T) {
		sub := &Subscription{
			ID:       "sub-1",
			Status:   StatusActive,
			AddonIDs: []string{"addon-1"},
		}
		spec := AddonSpec{
			AvailableAddonIDs: []string{"addon-1"},
		}

		err := spec.CanAddAddon(sub, "addon-1")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAddonAlreadyOnSubscription)
	})

	t.Run("addon not available on plan", func(t *testing.T) {
		sub := &Subscription{
			ID:       "sub-1",
			Status:   StatusActive,
			AddonIDs: nil,
		}
		spec := AddonSpec{
			AvailableAddonIDs: []string{"addon-1", "addon-2"},
		}

		err := spec.CanAddAddon(sub, "addon-3")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAddonNotAvailableOnPlan)
	})

	t.Run("max addons exceeded", func(t *testing.T) {
		sub := &Subscription{
			ID:       "sub-1",
			Status:   StatusActive,
			AddonIDs: []string{"addon-1", "addon-2"},
		}
		spec := AddonSpec{
			AvailableAddonIDs: []string{"addon-1", "addon-2", "addon-3"},
			MaxAddons:         2,
		}

		err := spec.CanAddAddon(sub, "addon-3")

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrMaxAddonsExceeded)
	})

	t.Run("unlimited addons when max is zero", func(t *testing.T) {
		sub := &Subscription{
			ID:       "sub-1",
			Status:   StatusActive,
			AddonIDs: []string{"addon-1", "addon-2", "addon-3"},
		}
		spec := AddonSpec{
			AvailableAddonIDs: []string{"addon-1", "addon-2", "addon-3", "addon-4"},
			MaxAddons:         0, // unlimited
		}

		err := spec.CanAddAddon(sub, "addon-4")

		require.NoError(t, err)
	})

	t.Run("empty available list allows any addon", func(t *testing.T) {
		sub := &Subscription{
			ID:       "sub-1",
			Status:   StatusActive,
			AddonIDs: nil,
		}
		spec := AddonSpec{
			AvailableAddonIDs: nil, // no restriction
		}

		err := spec.CanAddAddon(sub, "any-addon")

		require.NoError(t, err)
	})
}

func TestNewAddonSpec(t *testing.T) {
	plan := &Plan{
		AvailableAddons: []Addon{
			{ID: "addon-1"},
			{ID: "addon-2"},
		},
		MaxAddons: 5,
	}

	spec := NewAddonSpec(plan)

	assert.Equal(t, []string{"addon-1", "addon-2"}, spec.AvailableAddonIDs)
	assert.Equal(t, 5, spec.MaxAddons)
}

// --- UpgradeSpec ---

func TestUpgradeSpec_CanUpgrade(t *testing.T) {
	t.Run("target plan active", func(t *testing.T) {
		spec := UpgradeSpec{
			FromPlanTier: TierBasic,
			ToPlanTier:   TierPremium,
			ToPlanActive: true,
		}

		err := spec.CanUpgrade()

		require.NoError(t, err)
	})

	t.Run("target plan inactive", func(t *testing.T) {
		spec := UpgradeSpec{
			FromPlanTier: TierBasic,
			ToPlanTier:   TierPremium,
			ToPlanActive: false,
		}

		err := spec.CanUpgrade()

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrPendingPlanInactive)
	})
}

func TestFamilyGroup_EventRecorder_Embedded(t *testing.T) {
	fg, err := NewFamilyGroup("owner-1", 5, time.Now())
	require.NoError(t, err)

	// NewFamilyGroup records a creation event.
	assert.True(t, fg.HasEvents())

	// Flush the creation event before testing AddMember.
	creationEvents := fg.DomainEvents()
	require.Len(t, creationEvents, 1)
	assert.Equal(t, EventFamilyCreated, creationEvents[0].Type)
	assert.False(t, fg.HasEvents())

	// AddMember records its own event.
	require.NoError(t, fg.AddMember("user-2", "Alice", time.Now()))
	assert.True(t, fg.HasEvents())

	events := fg.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventFamilyMemberAdded, events[0].Type)
}
