package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tokenhash"
)

// IdentityRepository implements identity.Repository backed by PostgreSQL via
// sqlc-generated queries. All methods resolve the underlying connection via
// DBFromContext so that queries participate in the active transaction (and
// therefore respect SET LOCAL app.tenant_id for RLS enforcement).
type IdentityRepository struct {
	pool *pgxpool.Pool
}

// NewIdentityRepository returns a new IdentityRepository using the given pool.
func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool}
}

// q returns the sqlc Queries bound to the transaction from ctx (if any),
// otherwise falls back to the pool. This ensures row-level security session
// variables set via SET LOCAL are visible to every query.
func (r *IdentityRepository) q(ctx context.Context) *gen.Queries {
	return gen.New(DBFromContext(ctx, r.pool))
}

// bootstrapAdvisoryLockKey is a fixed key for the first-admin advisory lock. It
// only needs to be unique among advisory locks used by the application.
const bootstrapAdvisoryLockKey int64 = 911002

// CountAdmins returns the number of users with role 'admin'.
func (r *IdentityRepository) CountAdmins(ctx context.Context) (int64, error) {
	var n int64
	err := DBFromContext(ctx, r.pool).
		QueryRow(ctx, `SELECT count(*) FROM identity.platform_users WHERE role = 'admin'`).
		Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting admins: %w", err)
	}
	return n, nil
}

// AcquireBootstrapLock takes a transaction-scoped advisory lock so concurrent
// first-admin creations serialize. The lock releases automatically when the
// surrounding transaction ends. Must be called inside RunInTx.
func (r *IdentityRepository) AcquireBootstrapLock(ctx context.Context) error {
	if _, err := DBFromContext(ctx, r.pool).
		Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, bootstrapAdvisoryLockKey); err != nil {
		return fmt.Errorf("acquiring bootstrap lock: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Row converter
// ---------------------------------------------------------------------------

// rowToUser converts the sqlc IdentityPlatformUser model to a domain PlatformUser.
func rowToUser(row gen.IdentityPlatformUser) *identity.PlatformUser {
	return &identity.PlatformUser{
		ID:                 pgutil.PgtypeToUUID(row.ID),
		Email:              row.Email,
		PasswordHash:       row.PasswordHash,
		DisplayName:        pgutil.DerefStr(row.DisplayName),
		EmailVerified:      row.EmailVerified,
		TelegramID:         row.TelegramID,
		Role:               identity.Role(row.Role),
		TenantID:           pgutil.PgtypeUUIDToOptStr(row.TenantID),
		MustChangePassword: row.MustChangePassword,
		CreatedAt:          pgutil.PgtypeToTime(row.CreatedAt),
		UpdatedAt:          pgutil.PgtypeToTime(row.UpdatedAt),
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
	err := r.q(ctx).CreateUser(ctx, gen.CreateUserParams{
		ID:                 pgutil.UUIDToPgtype(user.ID),
		Email:              user.Email,
		PasswordHash:       user.PasswordHash,
		DisplayName:        pgutil.StrPtrOrNil(user.DisplayName),
		EmailVerified:      user.EmailVerified,
		TelegramID:         user.TelegramID,
		Role:               string(user.Role),
		TenantID:           pgutil.OptStrToPgtypeUUID(user.TenantID),
		CreatedAt:          pgutil.TimeToPgtype(user.CreatedAt),
		UpdatedAt:          pgutil.TimeToPgtype(user.UpdatedAt),
		MustChangePassword: user.MustChangePassword,
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create user: %w", identity.ErrAlreadyExists)
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (r *IdentityRepository) GetUserByID(ctx context.Context, id string) (*identity.PlatformUser, error) {
	row, err := r.q(ctx).GetUserByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by id", identity.ErrNotFound)
	}
	return rowToUser(row), nil
}

// getUserByIDForUpdateSQL is identical to GetUserByID but acquires a FOR UPDATE
// row lock. Must be called within a transaction.
const getUserByIDForUpdateSQL = `
SELECT id, email, password_hash, display_name, email_verified, telegram_id, role, tenant_id, created_at, updated_at, must_change_password
FROM identity.platform_users WHERE id = $1 FOR UPDATE
`

func (r *IdentityRepository) GetUserByIDForUpdate(ctx context.Context, id string) (*identity.PlatformUser, error) {
	db := DBFromContext(ctx, r.pool)
	row := db.QueryRow(ctx, getUserByIDForUpdateSQL, pgutil.UUIDToPgtype(id))

	var rawRow gen.IdentityPlatformUser
	err := row.Scan(
		&rawRow.ID, &rawRow.Email, &rawRow.PasswordHash, &rawRow.DisplayName,
		&rawRow.EmailVerified, &rawRow.TelegramID, &rawRow.Role, &rawRow.TenantID,
		&rawRow.CreatedAt, &rawRow.UpdatedAt, &rawRow.MustChangePassword,
	)
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by id for update", identity.ErrNotFound)
	}
	return rowToUser(rawRow), nil
}

func (r *IdentityRepository) GetUserByEmail(ctx context.Context, email string) (*identity.PlatformUser, error) {
	row, err := r.q(ctx).GetUserByEmail(ctx, email)
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by email", identity.ErrNotFound)
	}
	return rowToUser(row), nil
}

func (r *IdentityRepository) GetUserByTelegramID(ctx context.Context, telegramID int64) (*identity.PlatformUser, error) {
	row, err := r.q(ctx).GetUserByTelegramID(ctx, &telegramID)
	if err != nil {
		return nil, pgutil.MapErr(err, "get user by telegram id", identity.ErrNotFound)
	}
	return rowToUser(row), nil
}

func (r *IdentityRepository) ListUsers(ctx context.Context, limit, offset int) ([]*identity.PlatformUser, error) {
	rows, err := r.q(ctx).ListUsers(ctx, gen.ListUsersParams{
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
	err := r.q(ctx).UpdateUser(ctx, gen.UpdateUserParams{
		ID:                 pgutil.UUIDToPgtype(user.ID),
		Email:              user.Email,
		PasswordHash:       user.PasswordHash,
		DisplayName:        pgutil.StrPtrOrNil(user.DisplayName),
		EmailVerified:      user.EmailVerified,
		TelegramID:         user.TelegramID,
		Role:               string(user.Role),
		TenantID:           pgutil.OptStrToPgtypeUUID(user.TenantID),
		MustChangePassword: user.MustChangePassword,
	})
	return pgutil.MapErr(err, "update user", identity.ErrNotFound)
}

func (r *IdentityRepository) CreateSession(ctx context.Context, session *identity.Session) error {
	err := r.q(ctx).CreateSession(ctx, gen.CreateSessionParams{
		ID:           pgutil.UUIDToPgtype(session.ID),
		UserID:       pgutil.UUIDToPgtype(session.UserID),
		RefreshToken: tokenhash.Hash(session.RefreshToken),
		IpAddress:    session.IPAddress,
		UserAgent:    session.UserAgent,
		ExpiresAt:    pgutil.TimeToPgtype(session.ExpiresAt),
		CreatedAt:    pgutil.TimeToPgtype(session.CreatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create session: %w", identity.ErrAlreadyExists)
		}
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *IdentityRepository) GetSessionByRefreshToken(ctx context.Context, token string) (*identity.Session, error) {
	row, err := r.q(ctx).GetSessionByRefreshToken(ctx, tokenhash.Hash(token))
	if err != nil {
		return nil, pgutil.MapErr(err, "get session by refresh token", identity.ErrNotFound)
	}
	return &identity.Session{
		ID:           pgutil.PgtypeToUUID(row.ID),
		UserID:       pgutil.PgtypeToUUID(row.UserID),
		RefreshToken: row.RefreshToken,
		IPAddress:    row.IpAddress,
		UserAgent:    row.UserAgent,
		ExpiresAt:    pgutil.PgtypeToTime(row.ExpiresAt),
		CreatedAt:    pgutil.PgtypeToTime(row.CreatedAt),
	}, nil
}

func (r *IdentityRepository) DeleteSession(ctx context.Context, id string) error {
	err := r.q(ctx).DeleteSession(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete session", identity.ErrNotFound)
}

func (r *IdentityRepository) DeleteUserSessions(ctx context.Context, userID string) error {
	err := r.q(ctx).DeleteUserSessions(ctx, pgutil.UUIDToPgtype(userID))
	return pgutil.MapErr(err, "delete user sessions", identity.ErrNotFound)
}

// ActiveSession is a read-only DTO for listing active sessions with user email.
// It is not a domain entity -- it exists to serve admin list queries without
// inflating domain aggregates with cross-context data.
type ActiveSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	UserEmail string    `json:"user_email"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ListActiveSessions returns a paginated list of active (non-expired) sessions
// with the owning user's email. Intended for admin dashboards.
func (r *IdentityRepository) ListActiveSessions(ctx context.Context, limit, offset int) ([]ActiveSession, int64, error) {
	total, err := r.q(ctx).CountActiveSessions(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count active sessions: %w", err)
	}

	rows, err := r.q(ctx).ListActiveSessions(ctx, gen.ListActiveSessionsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list active sessions: %w", err)
	}

	sessions := make([]ActiveSession, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, ActiveSession{
			ID:        pgutil.PgtypeToUUID(row.ID),
			UserID:    pgutil.PgtypeToUUID(row.UserID),
			UserEmail: row.UserEmail,
			IPAddress: row.IpAddress,
			UserAgent: row.UserAgent,
			ExpiresAt: pgutil.PgtypeToTime(row.ExpiresAt),
			CreatedAt: pgutil.PgtypeToTime(row.CreatedAt),
		})
	}
	return sessions, total, nil
}

func (r *IdentityRepository) CreateEmailVerification(ctx context.Context, v *identity.EmailVerification) error {
	err := r.q(ctx).CreateEmailVerification(ctx, gen.CreateEmailVerificationParams{
		ID:        pgutil.UUIDToPgtype(v.ID),
		UserID:    pgutil.UUIDToPgtype(v.UserID),
		Email:     v.Email,
		Token:     tokenhash.Hash(v.Token),
		ExpiresAt: pgutil.TimeToPgtype(v.ExpiresAt),
		CreatedAt: pgutil.TimeToPgtype(v.CreatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create email verification: %w", identity.ErrAlreadyExists)
		}
		return fmt.Errorf("create email verification: %w", err)
	}
	return nil
}

func (r *IdentityRepository) GetEmailVerification(ctx context.Context, token string) (*identity.EmailVerification, error) {
	row, err := r.q(ctx).GetEmailVerification(ctx, tokenhash.Hash(token))
	if err != nil {
		return nil, pgutil.MapErr(err, "get email verification", identity.ErrNotFound)
	}
	return rowToEmailVerification(row), nil
}

func (r *IdentityRepository) DeleteEmailVerification(ctx context.Context, id string) error {
	err := r.q(ctx).DeleteEmailVerification(ctx, pgutil.UUIDToPgtype(id))
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
	err := r.q(ctx).CreatePasswordReset(ctx, gen.CreatePasswordResetParams{
		ID:        pgutil.UUIDToPgtype(pr.ID),
		UserID:    pgutil.UUIDToPgtype(pr.UserID),
		Email:     pr.Email,
		Token:     tokenhash.Hash(pr.Token),
		ExpiresAt: pgutil.TimeToPgtype(pr.ExpiresAt),
		CreatedAt: pgutil.TimeToPgtype(pr.CreatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create password reset: %w", identity.ErrAlreadyExists)
		}
		return fmt.Errorf("create password reset: %w", err)
	}
	return nil
}

func (r *IdentityRepository) GetPasswordResetByToken(ctx context.Context, token string) (*identity.PasswordReset, error) {
	row, err := r.q(ctx).GetPasswordResetByToken(ctx, tokenhash.Hash(token))
	if err != nil {
		return nil, pgutil.MapErr(err, "get password reset by token", identity.ErrNotFound)
	}
	return rowToPasswordReset(row), nil
}

func (r *IdentityRepository) DeletePasswordReset(ctx context.Context, id string) error {
	err := r.q(ctx).DeletePasswordReset(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete password reset", identity.ErrNotFound)
}

func (r *IdentityRepository) DeleteUserPasswordResets(ctx context.Context, userID string) error {
	err := r.q(ctx).DeleteUserPasswordResets(ctx, pgutil.UUIDToPgtype(userID))
	return pgutil.MapErr(err, "delete user password resets", identity.ErrNotFound)
}

// ---------------------------------------------------------------------------
// Cleanup operations for expired data
// ---------------------------------------------------------------------------

func (r *IdentityRepository) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	n, err := r.q(ctx).DeleteExpiredSessions(ctx)
	if err != nil {
		return 0, pgutil.MapErr(err, "delete expired sessions", identity.ErrNotFound)
	}
	return n, nil
}

func (r *IdentityRepository) DeleteExpiredVerifications(ctx context.Context) (int64, error) {
	n, err := r.q(ctx).DeleteExpiredVerifications(ctx)
	if err != nil {
		return 0, pgutil.MapErr(err, "delete expired verifications", identity.ErrNotFound)
	}
	return n, nil
}

func (r *IdentityRepository) DeleteExpiredPasswordResets(ctx context.Context) (int64, error) {
	n, err := r.q(ctx).DeleteExpiredPasswordResets(ctx)
	if err != nil {
		return 0, pgutil.MapErr(err, "delete expired password resets", identity.ErrNotFound)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Invitation operations
// ---------------------------------------------------------------------------

// rowToInvitation converts the sqlc IdentityInvitation model to a domain Invitation.
func rowToInvitation(row gen.IdentityInvitation) *identity.Invitation {
	var commissionRate *int
	if row.CommissionRate != nil {
		v := int(*row.CommissionRate)
		commissionRate = &v
	}
	return &identity.Invitation{
		ID:             pgutil.PgtypeToUUID(row.ID),
		Email:          row.Email,
		Token:          row.Token,
		RoleKey:        row.RoleKey,
		TenantID:       pgutil.PgtypeUUIDToOptStr(row.TenantID),
		CommissionRate: commissionRate,
		InvitedBy:      pgutil.PgtypeToUUID(row.InvitedBy),
		ExpiresAt:      pgutil.PgtypeToTime(row.ExpiresAt),
		CreatedAt:      pgutil.PgtypeToTime(row.CreatedAt),
	}
}

func (r *IdentityRepository) CreateInvitation(ctx context.Context, inv *identity.Invitation) error {
	var commissionRate *int32
	if inv.CommissionRate != nil {
		v := int32(*inv.CommissionRate)
		commissionRate = &v
	}
	err := r.q(ctx).CreateInvitation(ctx, gen.CreateInvitationParams{
		ID:             pgutil.UUIDToPgtype(inv.ID),
		Email:          inv.Email,
		Token:          tokenhash.Hash(inv.Token),
		RoleKey:        inv.RoleKey,
		TenantID:       pgutil.OptStrToPgtypeUUID(inv.TenantID),
		CommissionRate: commissionRate,
		InvitedBy:      pgutil.UUIDToPgtype(inv.InvitedBy),
		ExpiresAt:      pgutil.TimeToPgtype(inv.ExpiresAt),
		CreatedAt:      pgutil.TimeToPgtype(inv.CreatedAt),
	})
	if err != nil {
		if pgutil.IsUniqueViolation(err) {
			return fmt.Errorf("create invitation: %w", identity.ErrAlreadyExists)
		}
		return fmt.Errorf("create invitation: %w", err)
	}
	return nil
}

func (r *IdentityRepository) GetInvitationByToken(ctx context.Context, token string) (*identity.Invitation, error) {
	row, err := r.q(ctx).GetInvitationByToken(ctx, tokenhash.Hash(token))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invitation by token", identity.ErrNotFound)
	}
	return rowToInvitation(row), nil
}

func (r *IdentityRepository) GetInvitationByID(ctx context.Context, id string) (*identity.Invitation, error) {
	row, err := r.q(ctx).GetInvitationByID(ctx, pgutil.UUIDToPgtype(id))
	if err != nil {
		return nil, pgutil.MapErr(err, "get invitation by id", identity.ErrNotFound)
	}
	return rowToInvitation(row), nil
}

func (r *IdentityRepository) DeleteInvitation(ctx context.Context, id string) error {
	err := r.q(ctx).DeleteInvitation(ctx, pgutil.UUIDToPgtype(id))
	return pgutil.MapErr(err, "delete invitation", identity.ErrNotFound)
}

func (r *IdentityRepository) ListInvitations(ctx context.Context, tenantIDs []string, all bool) ([]*identity.Invitation, error) {
	if all {
		rows, err := r.q(ctx).ListAllInvitations(ctx)
		if err != nil {
			return nil, fmt.Errorf("list all invitations: %w", err)
		}
		invitations := make([]*identity.Invitation, 0, len(rows))
		for _, row := range rows {
			invitations = append(invitations, rowToInvitation(row))
		}
		return invitations, nil
	}
	pgIDs := pgutil.StringsToPgtypeUUIDs(tenantIDs)
	rows, err := r.q(ctx).ListInvitationsByTenants(ctx, pgIDs)
	if err != nil {
		return nil, fmt.Errorf("list invitations by tenants: %w", err)
	}
	invitations := make([]*identity.Invitation, 0, len(rows))
	for _, row := range rows {
		invitations = append(invitations, rowToInvitation(row))
	}
	return invitations, nil
}

func (r *IdentityRepository) DeleteExpiredInvitations(ctx context.Context) (int64, error) {
	n, err := r.q(ctx).DeleteExpiredInvitations(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired invitations: %w", err)
	}
	return n, nil
}

// compile-time interface check
var _ identity.Repository = (*IdentityRepository)(nil)
