package identity

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Identity-specific event types re-exported from aggregate for backward
// compatibility. New code should prefer the aggregate constants directly.
const (
	EventUserRegistered         = aggregate.EventUserRegistered
	EventEmailVerified          = aggregate.EventEmailVerified
	EventUserLoggedIn           = aggregate.EventUserLoggedIn
	EventProfileUpdated         = aggregate.EventProfileUpdated
	EventTokenRefreshed         = aggregate.EventTokenRefreshed
	EventPasswordResetRequested = aggregate.EventPasswordResetRequested
	EventPasswordReset          = aggregate.EventPasswordReset
	EventPasswordChanged        = aggregate.EventPasswordChanged
	EventTelegramLinked         = aggregate.EventTelegramLinked
	EventTelegramUnlinked       = aggregate.EventTelegramUnlinked
)

// Event is an alias for the shared domainevent.Event so that existing callers
// referencing identity.Event continue to compile without changes.
type Event = domainevent.Event

// EventType is an alias for the shared domainevent.EventType so that existing
// callers referencing identity.EventType continue to compile without changes.
type EventType = domainevent.EventType

// Payload type aliases re-exported from aggregate for backward compatibility.
type (
	UserRegisteredPayload         = aggregate.UserRegisteredPayload
	EmailVerifiedPayload          = aggregate.EmailVerifiedPayload
	UserLoggedInPayload           = aggregate.UserLoggedInPayload
	TokenRefreshedPayload         = aggregate.TokenRefreshedPayload
	PasswordResetRequestedPayload = aggregate.PasswordResetRequestedPayload
	PasswordResetPayload          = aggregate.PasswordResetPayload
	PasswordChangedPayload        = aggregate.PasswordChangedPayload
	TelegramLinkedPayload         = aggregate.TelegramLinkedPayload
	TelegramUnlinkedPayload       = aggregate.TelegramUnlinkedPayload
)
