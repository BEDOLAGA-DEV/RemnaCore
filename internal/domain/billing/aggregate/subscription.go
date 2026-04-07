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
	// ErrInvalidTransition indicates an invalid subscription state transition.
	ErrInvalidTransition = errors.New("invalid subscription state transition")

	// ErrSubscriptionNotActiveForRenewal indicates an attempt to renew a
	// subscription that is not in the active state.
	ErrSubscriptionNotActiveForRenewal = errors.New("subscription must be active to renew")

	// ErrEmptyUserID indicates that a required user ID was not provided.
	ErrEmptyUserID = errors.New("user ID is required")

	// ErrEmptyPlanID indicates that a required plan ID was not provided.
	ErrEmptyPlanID = errors.New("plan ID is required")
)

// SubscriptionStatus represents the current state of a subscription.
type SubscriptionStatus string

const (
	StatusTrial     SubscriptionStatus = "trial"
	StatusActive    SubscriptionStatus = "active"
	StatusPastDue   SubscriptionStatus = "past_due"
	StatusCancelled SubscriptionStatus = "cancelled"
	StatusExpired   SubscriptionStatus = "expired"
	StatusPaused    SubscriptionStatus = "paused"
)

// validTransitions defines the state machine for subscription status.
// Terminal states (cancelled, expired) have no valid outbound transitions.
var validTransitions = map[SubscriptionStatus][]SubscriptionStatus{
	StatusTrial:     {StatusActive, StatusCancelled, StatusExpired},
	StatusActive:    {StatusPastDue, StatusCancelled, StatusPaused, StatusExpired},
	StatusPastDue:   {StatusActive, StatusCancelled, StatusExpired},
	StatusPaused:    {StatusActive, StatusCancelled, StatusExpired},
	StatusCancelled: {},
	StatusExpired:   {},
}

// allSubscriptionStatuses enumerates every SubscriptionStatus constant.
// Keep in sync with the const block above.
var allSubscriptionStatuses = []SubscriptionStatus{
	StatusTrial, StatusActive, StatusPastDue,
	StatusCancelled, StatusExpired, StatusPaused,
}

// ValidateSubscriptionTransitions checks that every subscription status
// has an entry in the transition map. Called from tests, not at runtime.
func ValidateSubscriptionTransitions() error {
	for _, s := range allSubscriptionStatuses {
		if _, ok := validTransitions[s]; !ok {
			return fmt.Errorf("missing transition entry for subscription status: %s", s)
		}
	}
	return nil
}

// Subscription is the aggregate root for a user's subscription.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
//
// Concurrency safety: this aggregate relies on PostgreSQL row-level locking
// (SELECT FOR UPDATE via txmanager.RunInTx) rather than application-level
// optimistic concurrency control (version field + WHERE version = N). This
// is a deliberate architectural choice for the following reasons:
//
//   - All mutations go through BillingService which wraps them in RunInTx
//   - The transactional outbox pattern ensures events are atomic with state
//   - Single-writer per entity (no concurrent writes to the same subscription)
//
// If the system evolves to require CQRS read models, event sourcing, or
// distributed deployment across multiple databases, add a Version int field
// and enforce it in the repository's Update method:
//
//	UPDATE billing.subscriptions SET ... WHERE id = $1 AND version = $2
//
// This would also require adding version to the sqlc query and incrementing
// it in every aggregate mutation method.
type Subscription struct {
	domainevent.EventRecorder

	ID          string
	UserID      string
	PlanID      string
	Status      SubscriptionStatus
	Period      vo.BillingPeriod
	AddonIDs      []string
	AssignedTo    string              // self or familyMemberID
	PendingChange vo.PendingPlanChange // deferred downgrade; applied on next Renew
	CancelledAt   *time.Time
	PausedAt      *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewSubscription creates a new subscription in the trial state.
// Returns an error if required fields are missing.
func NewSubscription(userID, planID string, interval vo.BillingInterval, addonIDs []string, now time.Time) (*Subscription, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	if planID == "" {
		return nil, ErrEmptyPlanID
	}

	period := vo.NewBillingPeriod(now, interval)
	sub := &Subscription{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    userID,
		PlanID:    planID,
		Status:    StatusTrial,
		Period:    period,
		AddonIDs:  addonIDs,
		CreatedAt: now,
		UpdatedAt: now,
	}
	sub.RecordEvent(domainevent.NewTyped(SubCreatedPayload{
		SubscriptionID: sub.ID,
		UserID:         sub.UserID,
		PlanID:         sub.PlanID,
	}, now, sub.ID))
	return sub, nil
}

// CanTransitionTo reports whether the subscription can move from its current
// status to the target status.
func (s *Subscription) CanTransitionTo(target SubscriptionStatus) bool {
	allowed, ok := validTransitions[s.Status]
	if !ok {
		return false
	}
	return slices.Contains(allowed, target)
}

// transitionTo attempts to move the subscription to the target status.
func (s *Subscription) transitionTo(target SubscriptionStatus, now time.Time) error {
	if !s.CanTransitionTo(target) {
		return ErrInvalidTransition
	}
	s.Status = target
	s.UpdatedAt = now
	return nil
}

// Activate moves the subscription from trial or past_due to active.
func (s *Subscription) Activate(now time.Time) error {
	if err := s.transitionTo(StatusActive, now); err != nil {
		return err
	}
	s.RecordEvent(domainevent.NewTyped(SubActivatedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// MarkPastDue moves the subscription from active to past_due.
func (s *Subscription) MarkPastDue(now time.Time) error {
	if err := s.transitionTo(StatusPastDue, now); err != nil {
		return err
	}
	s.RecordEvent(domainevent.NewTyped(SubPastDuePayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// Cancel moves the subscription to cancelled from any non-terminal state.
func (s *Subscription) Cancel(now time.Time) error {
	if err := s.transitionTo(StatusCancelled, now); err != nil {
		return err
	}
	s.CancelledAt = &now
	s.RecordEvent(domainevent.NewTyped(SubCancelledPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// Pause moves the subscription from active to paused.
func (s *Subscription) Pause(now time.Time) error {
	if err := s.transitionTo(StatusPaused, now); err != nil {
		return err
	}
	s.PausedAt = &now
	s.RecordEvent(domainevent.NewTyped(SubPausedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// Resume moves the subscription from paused to active.
func (s *Subscription) Resume(now time.Time) error {
	if err := s.transitionTo(StatusActive, now); err != nil {
		return err
	}
	s.PausedAt = nil
	s.RecordEvent(domainevent.NewTyped(SubResumedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// Expire moves the subscription to expired from any non-terminal state.
func (s *Subscription) Expire(now time.Time) error {
	if err := s.transitionTo(StatusExpired, now); err != nil {
		return err
	}
	s.RecordEvent(domainevent.NewTyped(SubExpiredPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// AddAddon appends an addon ID to the subscription and records an update event.
// The AddonSpec enforces plan-level constraints: addon must be available on the
// plan, not already present, and within the max addons limit.
func (s *Subscription) AddAddon(addonID string, spec AddonSpec, now time.Time) error {
	if s.Status != StatusActive && s.Status != StatusTrial {
		return ErrSubscriptionNotActiveForAddon
	}
	if err := spec.CanAddAddon(s, addonID); err != nil {
		return err
	}
	s.AddonIDs = append(s.AddonIDs, addonID)
	s.UpdatedAt = now
	s.RecordEvent(domainevent.NewTyped(SubUpdatedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// RemoveAddon removes an addon ID from the subscription and records an update
// event. Returns an error if the addon is not present.
func (s *Subscription) RemoveAddon(addonID string, now time.Time) error {
	idx := slices.Index(s.AddonIDs, addonID)
	if idx == -1 {
		return ErrAddonNotOnSubscription
	}
	s.AddonIDs = slices.Delete(s.AddonIDs, idx, idx+1)
	s.UpdatedAt = now
	s.RecordEvent(domainevent.NewTyped(SubUpdatedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// Upgrade changes the subscription to a new plan immediately.
// Only active subscriptions can be upgraded. The UpgradeSpec validates that
// the target plan is active before the transition proceeds.
func (s *Subscription) Upgrade(newPlanID string, newPeriod vo.BillingPeriod, spec UpgradeSpec, now time.Time) error {
	if err := spec.CanUpgrade(); err != nil {
		return err
	}
	if s.Status != StatusActive {
		return ErrInvalidTransition
	}
	if newPlanID == s.PlanID {
		return ErrSamePlan
	}
	oldPlanID := s.PlanID
	s.PlanID = newPlanID
	s.Period = newPeriod
	s.PendingChange = vo.PendingPlanChange{} // clear deferred downgrade
	s.UpdatedAt = now
	s.RecordEvent(domainevent.NewTyped(SubUpgradedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
		FromPlanID:     oldPlanID,
		ToPlanID:       newPlanID,
	}, now, s.ID))
	return nil
}

// Downgrade schedules a plan change for the next billing period.
// The actual switch happens during Renew.
func (s *Subscription) Downgrade(newPlanID string, now time.Time) error {
	if s.Status != StatusActive {
		return ErrInvalidTransition
	}
	if newPlanID == s.PlanID {
		return ErrSamePlan
	}
	s.PendingChange = vo.PendingPlanChange{
		PlanID:         newPlanID,
		OriginalPlanID: s.PlanID,
		RequestedAt:    now,
	}
	s.UpdatedAt = now
	s.RecordEvent(domainevent.NewTyped(SubDowngradedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
		FromPlanID:     s.PlanID,
		ToPlanID:       newPlanID,
	}, now, s.ID))
	return nil
}

// Renew advances the subscription to its next billing period. The next period
// is calculated from the current period's end date and interval, so the caller
// does not need to construct the new period manually. Only allowed when active
// and the current billing period has elapsed.
//
// If a PendingChange was set by a prior Downgrade, it is applied during renewal
// and the pending field is cleared.
func (s *Subscription) Renew(now time.Time, pendingPlanActive bool) error {
	if s.Status != StatusActive {
		return ErrSubscriptionNotActiveForRenewal
	}
	if now.Before(s.Period.End) {
		return ErrPeriodNotElapsed
	}
	if !s.PendingChange.IsZero() && !pendingPlanActive {
		return ErrPendingPlanInactive
	}
	s.Period = s.Period.Next()
	if !s.PendingChange.IsZero() {
		s.PlanID = s.PendingChange.PlanID
		s.PendingChange = vo.PendingPlanChange{}
	}
	s.UpdatedAt = now
	s.RecordEvent(domainevent.NewTyped(SubRenewedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}

// ValidateForInvoicing checks whether the subscription is in a state that
// allows invoice creation. Cancelled and expired subscriptions cannot be
// invoiced. Returns ErrSubscriptionNotActiveForInvoicing for terminal states.
func (s *Subscription) ValidateForInvoicing() error {
	if s.Status == StatusCancelled || s.Status == StatusExpired {
		return ErrSubscriptionNotActiveForInvoicing
	}
	return nil
}
