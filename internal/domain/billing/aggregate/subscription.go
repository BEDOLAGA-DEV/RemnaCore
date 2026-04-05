package aggregate

import (
	"errors"
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
	sub.RecordEvent(domainevent.NewAtWithEntity(EventSubCreated, SubCreatedPayload{
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubActivated, SubActivatedPayload{
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubPastDue, SubPastDuePayload{
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubCancelled, SubCancelledPayload{
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubPaused, SubPausedPayload{
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubResumed, SubResumedPayload{
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubExpired, SubExpiredPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
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
	s.RecordEvent(domainevent.NewAtWithEntity(EventSubRenewed, SubRenewedPayload{
		SubscriptionID: s.ID,
		UserID:         s.UserID,
	}, now, s.ID))
	return nil
}
