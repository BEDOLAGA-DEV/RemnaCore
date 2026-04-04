package aggregate

import (
	"errors"
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
	StatusTrial:   {StatusActive, StatusCancelled, StatusExpired},
	StatusActive:  {StatusPastDue, StatusCancelled, StatusPaused, StatusExpired},
	StatusPastDue: {StatusActive, StatusCancelled, StatusExpired},
	StatusPaused:  {StatusActive, StatusCancelled, StatusExpired},
}

// Subscription is the aggregate root for a user's subscription.
// It embeds EventRecorder to accumulate domain events during mutations.
// Services must call DomainEvents() after persisting the aggregate to
// retrieve and publish all pending events.
type Subscription struct {
	domainevent.EventRecorder

	ID          string
	UserID      string
	PlanID      string
	Status      SubscriptionStatus
	Period      vo.BillingPeriod
	AddonIDs    []string
	AssignedTo  string // self or familyMemberID
	CancelledAt *time.Time
	PausedAt    *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewSubscription creates a new subscription in the trial state.
func NewSubscription(userID, planID string, interval vo.BillingInterval, addonIDs []string, now time.Time) *Subscription {
	return &Subscription{
		ID:        uuid.New().String(),
		UserID:    userID,
		PlanID:    planID,
		Status:    StatusTrial,
		Period:    vo.NewBillingPeriod(now, interval),
		AddonIDs:  addonIDs,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// CanTransitionTo reports whether the subscription can move from its current
// status to the target status.
func (s *Subscription) CanTransitionTo(target SubscriptionStatus) bool {
	allowed, ok := validTransitions[s.Status]
	if !ok {
		return false
	}
	for _, status := range allowed {
		if status == target {
			return true
		}
	}
	return false
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
	return s.transitionTo(StatusActive, now)
}

// MarkPastDue moves the subscription from active to past_due.
func (s *Subscription) MarkPastDue(now time.Time) error {
	return s.transitionTo(StatusPastDue, now)
}

// Cancel moves the subscription to cancelled from any non-terminal state.
func (s *Subscription) Cancel(now time.Time) error {
	if err := s.transitionTo(StatusCancelled, now); err != nil {
		return err
	}
	s.CancelledAt = &now
	return nil
}

// Pause moves the subscription from active to paused.
func (s *Subscription) Pause(now time.Time) error {
	if err := s.transitionTo(StatusPaused, now); err != nil {
		return err
	}
	s.PausedAt = &now
	return nil
}

// Resume moves the subscription from paused to active.
func (s *Subscription) Resume(now time.Time) error {
	if err := s.transitionTo(StatusActive, now); err != nil {
		return err
	}
	s.PausedAt = nil
	return nil
}

// Expire moves the subscription to expired from any non-terminal state.
func (s *Subscription) Expire(now time.Time) error {
	return s.transitionTo(StatusExpired, now)
}

// Renew advances the subscription to its next billing period. The next period
// is calculated from the current period's end date and interval, so the caller
// does not need to construct the new period manually. Only allowed when active.
func (s *Subscription) Renew(now time.Time) error {
	if s.Status != StatusActive {
		return ErrSubscriptionNotActiveForRenewal
	}
	s.Period = s.Period.Next()
	s.UpdatedAt = now
	return nil
}
