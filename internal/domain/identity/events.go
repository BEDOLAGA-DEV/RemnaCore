package identity

import (
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Identity-specific event types.
const (
	EventUserRegistered         domainevent.EventType = "user.registered"
	EventEmailVerified          domainevent.EventType = "user.email_verified"
	EventUserLoggedIn           domainevent.EventType = "user.logged_in"
	// EventProfileUpdated is reserved for future use.
	EventProfileUpdated domainevent.EventType = "user.profile_updated"
	EventPasswordResetRequested domainevent.EventType = "user.password_reset_requested"
	EventPasswordReset          domainevent.EventType = "user.password_reset"
)

// Event is an alias for the shared domainevent.Event so that existing callers
// referencing identity.Event continue to compile without changes.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType so that existing
// callers referencing identity.EventType continue to compile without changes.
type EventType = domainevent.EventType

// NewUserRegisteredEvent creates an event for a newly registered user.
func NewUserRegisteredEvent(userID, email string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventUserRegistered, UserRegisteredPayload{
		UserID: userID,
		Email:  email,
	}, now, userID)
}

// NewEmailVerifiedEvent creates an event for a successful email verification.
func NewEmailVerifiedEvent(userID, email string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventEmailVerified, EmailVerifiedPayload{
		UserID: userID,
		Email:  email,
	}, now, userID)
}

// NewUserLoggedInEvent creates an event for a successful login.
func NewUserLoggedInEvent(userID string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventUserLoggedIn, UserLoggedInPayload{
		UserID: userID,
	}, now, userID)
}

// NewPasswordResetRequestedEvent creates an event when a user requests a
// password reset. Notification plugins listen for this to send the reset email.
func NewPasswordResetRequestedEvent(userID, email, token string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventPasswordResetRequested, PasswordResetRequestedPayload{
		UserID: userID,
		Email:  email,
		Token:  token,
	}, now, userID)
}

// NewPasswordResetEvent creates an event when a password has been successfully
// reset.
func NewPasswordResetEvent(userID string, now time.Time) Event {
	return domainevent.NewAtWithEntity(EventPasswordReset, PasswordResetPayload{
		UserID: userID,
	}, now, userID)
}
