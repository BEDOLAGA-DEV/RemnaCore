package identity

import (
	"context"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
)

// Repository defines the persistence operations for the identity domain.
// This is the canonical port interface; it references aggregate types directly.
// The service subpackage defines its own identical Repository interface to
// avoid circular imports.
type Repository interface {
	CreateUser(ctx context.Context, user *aggregate.PlatformUser) error
	GetUserByID(ctx context.Context, id string) (*aggregate.PlatformUser, error)
	// GetUserByIDForUpdate retrieves a user by ID with a SELECT FOR UPDATE row
	// lock. Must be called within a RunInTx transaction to prevent TOCTOU races
	// during read-modify-write cycles.
	GetUserByIDForUpdate(ctx context.Context, id string) (*aggregate.PlatformUser, error)
	GetUserByEmail(ctx context.Context, email string) (*aggregate.PlatformUser, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*aggregate.PlatformUser, error)
	UpdateUser(ctx context.Context, user *aggregate.PlatformUser) error
	ListUsers(ctx context.Context, limit, offset int) ([]*aggregate.PlatformUser, error)

	// CountAdmins returns the number of users with role 'admin'.
	CountAdmins(ctx context.Context) (int64, error)
	// AcquireBootstrapLock takes a transaction-scoped advisory lock that
	// serializes concurrent first-admin creation. Must be called inside RunInTx.
	AcquireBootstrapLock(ctx context.Context) error

	CreateSession(ctx context.Context, session *aggregate.Session) error
	GetSessionByRefreshToken(ctx context.Context, token string) (*aggregate.Session, error)
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID string) error

	CreateEmailVerification(ctx context.Context, v *aggregate.EmailVerification) error
	GetEmailVerification(ctx context.Context, token string) (*aggregate.EmailVerification, error)
	DeleteEmailVerification(ctx context.Context, id string) error

	CreatePasswordReset(ctx context.Context, pr *aggregate.PasswordReset) error
	GetPasswordResetByToken(ctx context.Context, token string) (*aggregate.PasswordReset, error)
	DeletePasswordReset(ctx context.Context, id string) error
	DeleteUserPasswordResets(ctx context.Context, userID string) error

	// Cleanup operations for expired data. Return the number of rows deleted.
	DeleteExpiredSessions(ctx context.Context) (int64, error)
	DeleteExpiredVerifications(ctx context.Context) (int64, error)
	DeleteExpiredPasswordResets(ctx context.Context) (int64, error)

	// Invitation operations.
	CreateInvitation(ctx context.Context, inv *aggregate.Invitation) error
	GetInvitationByToken(ctx context.Context, token string) (*aggregate.Invitation, error)
	GetInvitationByID(ctx context.Context, id string) (*aggregate.Invitation, error)
	DeleteInvitation(ctx context.Context, id string) error
	// ListInvitations returns all invitations. If all is true, all invitations are
	// returned; otherwise only those scoped to the given tenantIDs.
	ListInvitations(ctx context.Context, tenantIDs []string, all bool) ([]*aggregate.Invitation, error)
	DeleteExpiredInvitations(ctx context.Context) (int64, error)
}
