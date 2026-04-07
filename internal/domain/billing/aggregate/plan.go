package aggregate

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

var (
	ErrEmptyPlanName             = errors.New("plan name must not be empty")
	ErrBasePriceNotPositive      = errors.New("base price must be positive")
	ErrNoCountriesAllowed        = errors.New("at least one country must be allowed")
	ErrFamilyDisabledWithMembers = errors.New("family is disabled but maxFamilyMembers is set")
	ErrNegativeTrafficLimit      = errors.New("traffic limit must not be negative")
	ErrAddonAlreadyExists        = errors.New("addon already exists on this plan")
	ErrAddonNotFound             = errors.New("addon not found")
	ErrPlanAlreadyActive         = errors.New("plan is already active")
	ErrPlanAlreadyInactive       = errors.New("plan is already inactive")
	ErrAddonNotOnPlan            = errors.New("addon not found on plan")
)

// PlanTier categorises a plan into a pricing tier.
type PlanTier string

const (
	TierBasic   PlanTier = "basic"
	TierPremium PlanTier = "premium"
	TierUltra   PlanTier = "ultra"
)

// AddonType categorises an addon by what it provides.
type AddonType string

const (
	AddonTraffic AddonType = "traffic"
	AddonNodes   AddonType = "nodes"
	AddonFeature AddonType = "feature"
)

// Addon represents an optional extra that can be added to a plan.
type Addon struct {
	ID                string
	Name              string
	Price             vo.Money
	Type              AddonType
	ExtraTrafficBytes int64
	ExtraNodes        []string
	ExtraFeatureFlags []string
}

// Plan is the aggregate root for a VPN subscription plan.
// It embeds EventRecorder to accumulate domain events during mutations.
type Plan struct {
	domainevent.EventRecorder

	ID                   string
	Name                 string
	Description          string
	BasePrice            vo.Money
	Interval             vo.BillingInterval
	TrafficLimitBytes    int64 // 0 = unlimited
	DeviceLimit          int
	AllowedCountries     []string
	AllowedProtocols     []string
	Tier                 PlanTier
	MaxRemnawaveBindings int
	FamilyEnabled        bool
	MaxFamilyMembers     int
	MaxAddons            int // 0 = unlimited
	AvailableAddons      []Addon
	IsActive             bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// NewPlan creates and validates a new Plan aggregate.
// Invariants: name not empty, base price positive, at least one country,
// family disabled must not have maxFamilyMembers > 0.
func NewPlan(
	name, description string,
	basePrice vo.Money,
	interval vo.BillingInterval,
	trafficLimitBytes int64,
	deviceLimit int,
	allowedCountries []string,
	allowedProtocols []string,
	tier PlanTier,
	maxRemnawaveBindings int,
	familyEnabled bool,
	maxFamilyMembers int,
	now time.Time,
) (*Plan, error) {
	if name == "" {
		return nil, ErrEmptyPlanName
	}
	if !basePrice.IsPositive() {
		return nil, ErrBasePriceNotPositive
	}
	if len(allowedCountries) == 0 {
		return nil, ErrNoCountriesAllowed
	}
	if !familyEnabled && maxFamilyMembers > 0 {
		return nil, ErrFamilyDisabledWithMembers
	}
	if trafficLimitBytes < 0 {
		return nil, ErrNegativeTrafficLimit
	}

	plan := &Plan{
		ID:                   uuid.Must(uuid.NewV7()).String(),
		Name:                 name,
		Description:          description,
		BasePrice:            basePrice,
		Interval:             interval,
		TrafficLimitBytes:    trafficLimitBytes,
		DeviceLimit:          deviceLimit,
		AllowedCountries:     allowedCountries,
		AllowedProtocols:     allowedProtocols,
		Tier:                 tier,
		MaxRemnawaveBindings: maxRemnawaveBindings,
		FamilyEnabled:        familyEnabled,
		MaxFamilyMembers:     maxFamilyMembers,
		AvailableAddons:      nil,
		IsActive:             true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	plan.RecordEvent(domainevent.NewTyped(PlanCreatedPayload{
		PlanID: plan.ID,
		Name:   plan.Name,
		Tier:   string(plan.Tier),
	}, now, plan.ID))
	return plan, nil
}

// Deactivate marks the plan as inactive. Inactive plans cannot be used for
// new checkouts. Returns ErrPlanAlreadyInactive if the plan is already inactive.
func (p *Plan) Deactivate(now time.Time) error {
	if !p.IsActive {
		return ErrPlanAlreadyInactive
	}
	p.IsActive = false
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewTyped(PlanDeactivatedPayload{
		PlanID: p.ID,
	}, now, p.ID))
	return nil
}

// Reactivate marks a previously deactivated plan as active again.
// Returns ErrPlanAlreadyActive if the plan is already active.
func (p *Plan) Reactivate(now time.Time) error {
	if p.IsActive {
		return ErrPlanAlreadyActive
	}
	p.IsActive = true
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewTyped(PlanUpdatedPayload{
		PlanID: p.ID,
		Name:   p.Name,
	}, now, p.ID))
	return nil
}

// RemoveAddon removes an addon from the plan by ID.
// Returns ErrAddonNotOnPlan if the addon is not found on this plan.
func (p *Plan) RemoveAddon(addonID string, now time.Time) error {
	idx := slices.IndexFunc(p.AvailableAddons, func(a Addon) bool {
		return a.ID == addonID
	})
	if idx == -1 {
		return ErrAddonNotOnPlan
	}
	p.AvailableAddons = slices.Delete(p.AvailableAddons, idx, idx+1)
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewTyped(PlanUpdatedPayload{
		PlanID: p.ID,
		Name:   p.Name,
	}, now, p.ID))
	return nil
}

// UpdateBasePrice changes the plan's base price. The new price must be positive
// and in the same currency as the current base price.
func (p *Plan) UpdateBasePrice(newPrice vo.Money, now time.Time) error {
	if !newPrice.IsPositive() {
		return ErrBasePriceNotPositive
	}
	if newPrice.Currency != p.BasePrice.Currency {
		return vo.ErrCurrencyMismatch
	}
	p.BasePrice = newPrice
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewTyped(PlanUpdatedPayload{
		PlanID: p.ID,
		Name:   p.Name,
	}, now, p.ID))
	return nil
}

// AddAddon adds an addon to the plan. Returns an error if an addon with the
// same ID already exists.
func (p *Plan) AddAddon(addon Addon, now time.Time) error {
	if p.HasAddon(addon.ID) {
		return ErrAddonAlreadyExists
	}
	p.AvailableAddons = append(p.AvailableAddons, addon)
	p.UpdatedAt = now
	p.RecordEvent(domainevent.NewTyped(PlanUpdatedPayload{
		PlanID: p.ID,
		Name:   p.Name,
	}, now, p.ID))
	return nil
}

// HasAddon reports whether the plan contains an addon with the given ID.
func (p *Plan) HasAddon(addonID string) bool {
	for _, a := range p.AvailableAddons {
		if a.ID == addonID {
			return true
		}
	}
	return false
}

// AvailableAddonIDs returns the IDs of all addons available on this plan.
func (p *Plan) AvailableAddonIDs() []string {
	ids := make([]string, len(p.AvailableAddons))
	for i, a := range p.AvailableAddons {
		ids[i] = a.ID
	}
	return ids
}

// CalculateTotal returns the base price plus the prices of all selected addons.
// Returns an error if any requested addon ID is not found on this plan.
func (p *Plan) CalculateTotal(addonIDs []string) (vo.Money, error) {
	total := p.BasePrice

	addonMap := p.addonMap()
	for _, id := range addonIDs {
		addon, ok := addonMap[id]
		if !ok {
			return vo.Money{}, fmt.Errorf("%w: %s", ErrAddonNotFound, id)
		}
		sum, err := total.Add(addon.Price)
		if err != nil {
			return vo.Money{}, err
		}
		total = sum
	}

	return total, nil
}

// addonMap builds a lookup map from addon ID to Addon for O(1) access.
func (p *Plan) addonMap() map[string]Addon {
	m := make(map[string]Addon, len(p.AvailableAddons))
	for _, a := range p.AvailableAddons {
		m[a.ID] = a
	}
	return m
}
