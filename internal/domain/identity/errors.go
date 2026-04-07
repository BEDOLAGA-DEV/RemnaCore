package identity

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
)

// Service-level error aliases re-exported for backward compatibility.
var (
	ErrNotFound              = service.ErrNotFound
	ErrEmailTaken            = service.ErrEmailTaken
	ErrInvalidCredentials    = service.ErrInvalidCredentials
	ErrEmailNotVerified      = service.ErrEmailNotVerified
	ErrTokenExpired          = service.ErrTokenExpired
	ErrSessionExpired        = service.ErrSessionExpired
	ErrPasswordResetExpired  = service.ErrPasswordResetExpired
	ErrPasswordResetNotFound = service.ErrPasswordResetNotFound
)

// Aggregate-level error aliases re-exported for backward compatibility.
var (
	ErrPasswordTooShort      = aggregate.ErrPasswordTooShort
	ErrPasswordTooWeak       = aggregate.ErrPasswordTooWeak
	ErrDisplayNameTooLong    = aggregate.ErrDisplayNameTooLong
	ErrTelegramAlreadyLinked = aggregate.ErrTelegramAlreadyLinked
	ErrTelegramNotLinked     = aggregate.ErrTelegramNotLinked
)
