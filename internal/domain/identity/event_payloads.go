package identity

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// UserRegisteredPayload is the typed payload for EventUserRegistered.
type UserRegisteredPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// EmailVerifiedPayload is the typed payload for EventEmailVerified.
type EmailVerifiedPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// UserLoggedInPayload is the typed payload for EventUserLoggedIn.
type UserLoggedInPayload struct {
	UserID string `json:"user_id"`
}

// TokenRefreshedPayload is the typed payload for EventTokenRefreshed.
type TokenRefreshedPayload struct {
	UserID string `json:"user_id"`
}

// PasswordResetRequestedPayload is the typed payload for EventPasswordResetRequested.
type PasswordResetRequestedPayload struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Token  string `json:"token"`
}

// PasswordResetPayload is the typed payload for EventPasswordReset.
type PasswordResetPayload struct {
	UserID string `json:"user_id"`
}

// --- EventPayload interface implementations ---

func (UserRegisteredPayload) EventType() domainevent.EventType { return EventUserRegistered }
func (EmailVerifiedPayload) EventType() domainevent.EventType  { return EventEmailVerified }
func (UserLoggedInPayload) EventType() domainevent.EventType    { return EventUserLoggedIn }
func (TokenRefreshedPayload) EventType() domainevent.EventType  { return EventTokenRefreshed }
func (PasswordResetRequestedPayload) EventType() domainevent.EventType {
	return EventPasswordResetRequested
}
func (PasswordResetPayload) EventType() domainevent.EventType { return EventPasswordReset }

// Compile-time interface checks.
var (
	_ domainevent.EventPayload = UserRegisteredPayload{}
	_ domainevent.EventPayload = EmailVerifiedPayload{}
	_ domainevent.EventPayload = UserLoggedInPayload{}
	_ domainevent.EventPayload = TokenRefreshedPayload{}
	_ domainevent.EventPayload = PasswordResetRequestedPayload{}
	_ domainevent.EventPayload = PasswordResetPayload{}
)
