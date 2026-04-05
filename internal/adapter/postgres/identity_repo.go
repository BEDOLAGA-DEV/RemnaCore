package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// IdentityRepository implements identity.Repository backed by PostgreSQL via
// sqlc-generated queries.
type IdentityRepository struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

// NewIdentityRepository returns a new IdentityRepository using the given pool.
func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{
		pool:    pool,
		queries: gen.New(pool),
	}
}

// ---------------------------------------------------------------------------
// Row converter
// ---------------------------------------------------------------------------

// rowToUser converts the sqlc IdentityPlatformUser model to a domain PlatformUser.
func rowToUser(row gen.IdentityPlatformUser) *identity.PlatformUser {
	return &identity.PlatformUser{
		ID:            pgutil.PgtypeToUUID(row.ID),
		Email:         row.Email,
		PasswordHash:  row.PasswordHash,
		DisplayName:   pgutil.DerefStr(row.DisplayName),
		EmailVerified: row.EmailVerified,
		TelegramID:    row.TelegramID,
		Role:          identity.Role(row.Role),
		TenantID:      pgutil.PgtypeUUIDToOptStr(row.TenantID),
		CreatedAt:     pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:     pgutil.PgtypeToTime(row.UpdatedAt),
	}
}

// rowToSession converts the sqlc IdentitySession model to a domain Session.
func rowToSession(row gen.IdentitySession) *identity.Session {
	return &identity.Session{
		ID:           pgutil.PgtypeToUUID(row.ID),
		UserID:       pgutil.PgtypeToUUID(row.UserID),
		RefreshToken: row.RefreshToken,
		ExpiresAt:    pgutil.PgtypeToTime(row.ExpiresAt),
		CreatedAt:    pgutil.PgtypeToTime(row.CreatedAt),
	}
}

// rowToEmailVerification converts the sqlc IdentityEmailVerification model to
// a domain EmailVerification.
func rowToEmailVerification(row gen.IdentityEmailVerification) *identity.EmailVerification {
	return &identity.EmailVerification{
		ID:        pgutil.PgtypeToUUID(row.ID),
		UserID:    pgutil.PgtypeToUUID(row.UserID),
		Email:     row.Email,
		Token:     row.Token,
		ExpiresAt: pgutil.PgtypeToTime(row.ExpiresAt),
		CreatedAt: pgutil.PgtypeToTime(row.CreatedAt),
	}
}


// ---------------------------------------------------------------------------
// Repository interface implementation
// ---------------------------------------------------------------------------

func (r *IdentityRepository) CreateUser(ctx context.Context, user *identity.PlatformUser) error {
	err := r.queries.CreateUser(ctx, gen.CreateUserParams{
		ID:            pgutil.UUIDToPgtype(user.ID),
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		DisplayName:   pgutil.StrPtrOrNil(user.DisplayName),
		EmailVerified: user.EmailVerified,
		TelegramID:    user.TelegramID,
		Role:          string(user.Role),
		TenantID:      pgutil.OptStrToPgtypeUUID(user.TenantID),
		CreatedAt:     pgutil.TimeToPgtype(user.CreatedAt),
		UpdatedAt:     pgutil.TimeToPgtype(user.UpdatedAt),
	})
	return pgutil.MapErr(err, "create user", identity.ErrNotFound)
}

func (r *IdentityRepository) GetUserByID(ctx context.Context, id string) (*identity.PlatformUser, error) {
	row, err := r.queries.GetUserByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by id", identity.ErrNotFound)
	}
	return rowToUser(row), nil
}

func (r *IdentityRepository) GetUserByEmail(ctx context.Context, email string) (*identity.PlatformUser, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by email", identity.ErrNotFound)
	}
	return rowToUser(row), nil
}

func (r *IdentityRepository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*identity.PlatformUser, error) {
	row, err := r.queries.GetUserByTelegramID(ctx, &telegramID)
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by telegram id", identity.ErrNotFound)
	}
	return rowToUser(row), nil
}

func (r *IdentityRepository) ListUsers(ctx context.Context, limit, offset int) ([]*identity.PlatformUser, error) {
	rows, err := r.queries.ListUsers(ctx, gen.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "list users", identity.ErrNotFound)
	}
	users := make([]*identity.PlatformUser, 0, len(rows))
	for _, row := range rows {
		users = append(users, rowToUser(row))
	}
	return users, nil
}

func (r *IdentityRepository) UpdateUser(ctx context.Context, user *identity.PlatformUser) error {
	err := r.queries.UpdateUser(ctx, gen.UpdateUserParams{
		ID:            pgutil.UUIDToPgtype(user.ID),
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		DisplayName:   pgutil.StrPtrOrNil(user.DisplayName),
		EmailVerified: user.EmailVerified,
		TelegramID:    user.TelegramID,
		Role:          string(user.Role),
		TenantID:      pgutil.OptStrToPgtypeUUID(user.TenantID),
	})
	return pgutil.MapErr(err, "update user", identity.ErrNotFound)
}

func (r *IdentityRepository) CreateSession(ctx context.Context, session *identity.Session) error {
	err := r.queries.CreateSession(ctx, gen.CreateSessionParams{
		ID:           pgutil.UUIDToPgtype(session.ID),
		UserID:       pgutil.UUIDToPgtype(session.UserID),
		RefreshToken: session.RefreshToken,
		ExpiresAt:    pgutil.TimeToPgtype(session.ExpiresAt),
		CreatedAt:    pgutil.TimeToPgtype(session.CreatedAt),
	})
	return pgutil.MapErr(err, "create session", identity.ErrNotFound)
}

func (r *IdentityRepository) GetSessionByRefreshToken(ctx context.Context, token string) (*identity.Session, error) {
	row, err := r.queries.GetSessionByRefreshToken(ctx, token)
	if err != nil {
		return nil, pgutil.MapErr(err, "get session by refresh token", identity.ErrNotFound)
	}
	return rowToSession(row), nil
}

func (r *IdentityRepository) DeleteSession(ctx context.Context, id string) error {
	err := r.queries.DeleteSession(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete session", identity.ErrNotFound)
}

func (r *IdentityRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	err := r.queries.DeleteUserSessions(ctx, pgutil.UUIDToPgtype(userID))
	return pgutil.MapErr(err, "delete user sessions", identity.ErrNotFound)
}

func (r *IdentityRepository) CreateEmailVerification(ctx context.Context, v *identity.EmailVerification) error {
	err := r.queries.CreateEmailVerification(ctx, gen.CreateEmailVerificationParams{
		ID:        pgutil.UUIDToPgtype(v.ID),
		UserID:    pgutil.UUIDToPgtype(v.UserID),
		Email:     v.Email,
		Token:     v.Token,
		ExpiresAt: pgutil.TimeToPgtype(v.ExpiresAt),
		CreatedAt: pgutil.TimeToPgtype(v.CreatedAt),
	})
	return pgutil.MapErr(err, "create email verification", identity.ErrNotFound)
}

func (r *IdentityRepository) GetEmailVerification(ctx context.Context, token string) (*identity.EmailVerification, error) {
	row, err := r.queries.GetEmailVerification(ctx, token)
	if err != nil {
		return nil, pgutil.MapErr(err, "get email verification", identity.ErrNotFound)
	}
	return rowToEmailVerification(row), nil
}

func (r *IdentityRepository) DeleteEmailVerification(ctx context.Context, id string) error {
	err := r.queries.DeleteEmailVerification(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete email verification", identity.ErrNotFound)
}

// ---------------------------------------------------------------------------
// Password reset operations
// ---------------------------------------------------------------------------

// rowToPasswordReset converts the sqlc IdentityPasswordReset model to a domain
// PasswordReset.
func rowToPasswordReset(row gen.IdentityPasswordReset) *identity.PasswordReset {
	return &identity.PasswordReset{
		ID:        pgutil.PgtypeToUUID(row.ID),
		UserID:    pgutil.PgtypeToUUID(row.UserID),
		Email:     row.Email,
		Token:     row.Token,
		ExpiresAt: pgutil.PgtypeToTime(row.ExpiresAt),
		CreatedAt: pgutil.PgtypeToTime(row.CreatedAt),
	}
}

func (r *IdentityRepository) CreatePasswordReset(ctx context.Context, pr *identity.PasswordReset) error {
	err := r.queries.CreatePasswordReset(ctx, gen.CreatePasswordResetParams{
		ID:        pgutil.UUIDToPgtype(pr.ID),
		UserID:    pgutil.UUIDToPgtype(pr.UserID),
		Email:     pr.Email,
		Token:     pr.Token,
		ExpiresAt: pgutil.TimeToPgtype(pr.ExpiresAt),
		CreatedAt: pgutil.TimeToPgtype(pr.CreatedAt),
	})
	return pgutil.MapErr(err, "create password reset", identity.ErrNotFound)
}

func (r *IdentityRepository) GetPasswordResetByToken(ctx context.Context, token string) (*identity.PasswordReset, error) {
	row, err := r.queries.GetPasswordResetByToken(ctx, token)
	if err != nil {
		return nil, pgutil.MapErr(err, "get password reset by token", identity.ErrNotFound)
	}
	return rowToPasswordReset(row), nil
}

func (r *IdentityRepository) DeletePasswordReset(ctx context.Context, id string) error {
	err := r.queries.DeletePasswordReset(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete password reset", identity.ErrNotFound)
}

func (r *IdentityRepository) DeleteUserPasswordResets(ctx context.Context, userID string) error {
	err := r.queries.DeleteUserPasswordResets(ctx, pgutil.UUIDToPgtype(userID))
	return pgutil.MapErr(err, "delete user password resets", identity.ErrNotFound)
}

// compile-time interface check
var _ identity.Repository = (*IdentityRepository)(nil)
