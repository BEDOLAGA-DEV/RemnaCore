package aggregate

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"

// Identity-specific event types.
const (
	EventUserRegistered         domainevent.EventType = "user.registered"
	EventEmailVerified          domainevent.EventType = "user.email_verified"
	EventUserLoggedIn           domainevent.EventType = "user.logged_in"
	EventProfileUpdated         domainevent.EventType = "user.profile_updated"
	EventTokenRefreshed         domainevent.EventType = "user.token_refreshed"
	EventPasswordResetRequested domainevent.EventType = "user.password_reset_requested"
	EventPasswordReset          domainevent.EventType = "user.password_reset"
	EventPasswordChanged        domainevent.EventType = "user.password_changed"
	EventTelegramLinked         domainevent.EventType = "user.telegram_linked"
	EventTelegramUnlinked       domainevent.EventType = "user.telegram_unlinked"
)

// --- Event payload types ---

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

// PasswordChangedPayload is the typed payload for EventPasswordChanged.
type PasswordChangedPayload struct {
	UserID string `json:"user_id"`
}

// TelegramLinkedPayload is the typed payload for EventTelegramLinked.
type TelegramLinkedPayload struct {
	UserID     string `json:"user_id"`
	TelegramID int64  `json:"telegram_id"`
}

// TelegramUnlinkedPayload is the typed payload for EventTelegramUnlinked.
type TelegramUnlinkedPayload struct {
	UserID     string `json:"user_id"`
	TelegramID int64  `json:"telegram_id"`
}

// --- EventPayload interface implementations ---

func (UserRegisteredPayload) EventType() domainevent.EventType { return EventUserRegistered }
func (EmailVerifiedPayload) EventType() domainevent.EventType  { return EventEmailVerified }
func (UserLoggedInPayload) EventType() domainevent.EventType   { return EventUserLoggedIn }
func (TokenRefreshedPayload) EventType() domainevent.EventType { return EventTokenRefreshed }
func (PasswordResetRequestedPayload) EventType() domainevent.EventType {
	return EventPasswordResetRequested
}
func (PasswordResetPayload) EventType() domainevent.EventType   { return EventPasswordReset }
func (PasswordChangedPayload) EventType() domainevent.EventType { return EventPasswordChanged }
func (TelegramLinkedPayload) EventType() domainevent.EventType  { return EventTelegramLinked }
func (TelegramUnlinkedPayload) EventType() domainevent.EventType {
	return EventTelegramUnlinked
}

// Compile-time interface checks.
var (
	_ domainevent.EventPayload = UserRegisteredPayload{}
	_ domainevent.EventPayload = EmailVerifiedPayload{}
	_ domainevent.EventPayload = UserLoggedInPayload{}
	_ domainevent.EventPayload = TokenRefreshedPayload{}
	_ domainevent.EventPayload = PasswordResetRequestedPayload{}
	_ domainevent.EventPayload = PasswordResetPayload{}
	_ domainevent.EventPayload = PasswordChangedPayload{}
	_ domainevent.EventPayload = TelegramLinkedPayload{}
	_ domainevent.EventPayload = TelegramUnlinkedPayload{}
)
