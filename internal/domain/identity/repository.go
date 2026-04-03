package identity

import "context"

// Repository defines the persistence operations for the identity domain.
type Repository interface {
	CreateUser(ctx context.Context, user *PlatformUser) error
	GetUserByID(ctx context.Context, id string) (*PlatformUser, error)
	GetUserByEmail(ctx context.Context, email string) (*PlatformUser, error)
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*PlatformUser, error)
	UpdateUser(ctx context.Context, user *PlatformUser) error
	ListUsers(ctx context.Context, limit, offset int) ([]*PlatformUser, error)

	CreateSession(ctx context.Context, session *Session) error
	GetSessionByRefreshToken(ctx context.Context, token string) (*Session, error)
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID string) error

	CreateEmailVerification(ctx context.Context, v *EmailVerification) error
	GetEmailVerification(ctx context.Context, token string) (*EmailVerification, error)
	DeleteEmailVerification(ctx context.Context, id string) error

	CreatePasswordReset(ctx context.Context, pr *PasswordReset) error
	GetPasswordResetByToken(ctx context.Context, token string) (*PasswordReset, error)
	DeletePasswordReset(ctx context.Context, id string) error
	DeleteUserPasswordResets(ctx context.Context, userID string) error
}
