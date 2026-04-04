package identity

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

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
func NewUserRegisteredEvent(userID, email string) Event {
	return domainevent.NewWithEntity(EventUserRegistered, UserRegisteredPayload{
		UserID: userID,
		Email:  email,
	}, userID)
}

// NewEmailVerifiedEvent creates an event for a successful email verification.
func NewEmailVerifiedEvent(userID, email string) Event {
	return domainevent.NewWithEntity(EventEmailVerified, EmailVerifiedPayload{
		UserID: userID,
		Email:  email,
	}, userID)
}

// NewUserLoggedInEvent creates an event for a successful login.
func NewUserLoggedInEvent(userID string) Event {
	return domainevent.NewWithEntity(EventUserLoggedIn, UserLoggedInPayload{
		UserID: userID,
	}, userID)
}

// NewPasswordResetRequestedEvent creates an event when a user requests a
// password reset. Notification plugins listen for this to send the reset email.
func NewPasswordResetRequestedEvent(userID, email, token string) Event {
	return domainevent.NewWithEntity(EventPasswordResetRequested, PasswordResetRequestedPayload{
		UserID: userID,
		Email:  email,
		Token:  token,
	}, userID)
}

// NewPasswordResetEvent creates an event when a password has been successfully
// reset.
func NewPasswordResetEvent(userID string) Event {
	return domainevent.NewWithEntity(EventPasswordReset, PasswordResetPayload{
		UserID: userID,
	}, userID)
}
