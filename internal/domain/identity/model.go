// model.go re-exports types from aggregate/, service/, and vo/ subpackages
// for backward compatibility. New code should import subpackages directly:
//   - identity/aggregate for PlatformUser, Session, EmailVerification
//   - identity/service for Service, Repository
//   - identity/vo for Role
//
// These aliases will be removed in a future major version.
package identity

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// --- Aggregate type aliases (backward compatibility) ---

type (
	PlatformUser      = aggregate.PlatformUser
	Session           = aggregate.Session
	EmailVerification = aggregate.EmailVerification
	PasswordReset     = aggregate.PasswordReset
	Invitation        = aggregate.Invitation
)

// --- Value object aliases (backward compatibility) ---

// Role re-exported from vo for backward compatibility.
type Role = vo.Role

const (
	RoleCustomer = vo.RoleCustomer
	RoleReseller = vo.RoleReseller
	RoleAdmin    = vo.RoleAdmin
)

// --- Aggregate constant aliases (backward compatibility) ---

const (
	MinPasswordLength     = aggregate.MinPasswordLength
	MaxDisplayNameLen     = aggregate.MaxDisplayNameLen
	EmailVerificationTTL  = aggregate.EmailVerificationTTL
	PasswordResetTTL      = aggregate.PasswordResetTTL
	VerificationTokenLen  = aggregate.VerificationTokenLen
	PasswordResetTokenLen = aggregate.PasswordResetTokenLen
	RefreshTokenLen       = service.RefreshTokenLen
)

// --- Constructor aliases (backward compatibility) ---

var (
	NewPlatformUser      = aggregate.NewPlatformUser
	NewAdminUser         = aggregate.NewAdminUser
	NewEmailVerification = aggregate.NewEmailVerification
	NewPasswordReset     = aggregate.NewPasswordReset
)

// --- Service type aliases (backward compatibility) ---

type (
	Service               = service.Service
	RegisterInput         = service.RegisterInput
	RegisterResult        = service.RegisterResult
	LoginInput            = service.LoginInput
	LoginResult           = service.LoginResult
	CreateFirstAdminInput = service.CreateFirstAdminInput
)

// NewService is a convenience re-export from the service subpackage.
var NewService = service.NewService

// --- Event factory aliases (backward compatibility) ---

var (
	NewUserLoggedInEvent           = service.NewUserLoggedInEvent
	NewTokenRefreshedEvent         = service.NewTokenRefreshedEvent
	NewPasswordResetRequestedEvent = service.NewPasswordResetRequestedEvent
	NewPasswordResetEvent          = service.NewPasswordResetEvent
)

// EventPublisher is an alias for the shared domainevent.Publisher so that
// existing callers referencing identity.EventPublisher continue to compile.
type EventPublisher = domainevent.Publisher
