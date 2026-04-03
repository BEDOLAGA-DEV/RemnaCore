package identity

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
