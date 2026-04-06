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
	EventTokenRefreshed         domainevent.EventType = "user.token_refreshed"
	EventPasswordResetRequested domainevent.EventType = "user.password_reset_requested"
	EventPasswordReset          domainevent.EventType = "user.password_reset"
	EventPasswordChanged        domainevent.EventType = "user.password_changed"
	EventTelegramLinked         domainevent.EventType = "user.telegram_linked"
	EventTelegramUnlinked       domainevent.EventType = "user.telegram_unlinked"
)

// Event is an alias for the shared domainevent.Event so that existing callers
// referencing identity.Event continue to compile without changes.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType so that existing
// callers referencing identity.EventType continue to compile without changes.
type EventType = domainevent.EventType

// NewUserLoggedInEvent creates an event for a successful login.
func NewUserLoggedInEvent(userID string, now time.Time) Event {
	return domainevent.NewTyped(UserLoggedInPayload{
		UserID: userID,
	}, now, userID)
}

// NewTokenRefreshedEvent creates an event for a successful token rotation.
func NewTokenRefreshedEvent(userID string, now time.Time) Event {
	return domainevent.NewTyped(TokenRefreshedPayload{
		UserID: userID,
	}, now, userID)
}

// NewPasswordResetRequestedEvent creates an event when a user requests a
// password reset. Notification plugins listen for this to send the reset email.
func NewPasswordResetRequestedEvent(userID, email, token string, now time.Time) Event {
	return domainevent.NewTyped(PasswordResetRequestedPayload{
		UserID: userID,
		Email:  email,
		Token:  token,
	}, now, userID)
}

// NewPasswordResetEvent creates an event when a password has been successfully
// reset.
func NewPasswordResetEvent(userID string, now time.Time) Event {
	return domainevent.NewTyped(PasswordResetPayload{
		UserID: userID,
	}, now, userID)
}
