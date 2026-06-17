package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// RefreshTokenLen is the number of random bytes used for refresh tokens.
// The resulting hex-encoded string is twice this length (64 chars).
const RefreshTokenLen = 32

// Repository defines the persistence operations for the identity domain.
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
}

// Service implements the core identity use-cases: registration, login, email
// verification, token refresh, and profile retrieval.
//
// # Architectural rationale
//
// Service is a "thick" domain service that combines domain logic (user
// construction, credential verification, token generation) with application
// orchestration (transaction management, event publishing, session lifecycle).
// In strict hexagonal architecture these would be separate layers, but
// RemnaCore intentionally merges them:
//
//   - Every public method follows the same pattern: load entities, enforce
//     invariants, persist changes and publish events within a single database
//     transaction (txmanager.Runner). A separate application service would
//     duplicate this ceremony without adding meaningful separation.
//
//   - Domain entities (PlatformUser, EmailVerification, PasswordReset) own their
//     own validation and state transitions. Service orchestrates them but does
//     not contain business rules -- those live in the entities.
//
//   - Security-sensitive operations (password hashing, JWT signing, refresh token
//     rotation) are delegated to pkg/authutil, keeping the service focused on
//     coordination rather than cryptographic implementation.
//
// Rationale: in a modular monolith, splitting identity into a thin domain
// service + application service would create two files with near-identical
// method signatures where the application service simply delegates to the
// domain service and wraps it in a transaction. The thick service pattern keeps
// the identity context's public API in one place.
//
// Auth operations (Register, Login, RefreshToken) live in this file.
// Profile operations (VerifyEmail, UpdateDisplayName, LinkTelegram,
// UnlinkTelegram, ResetPassword) live in identity_profile.go.
type Service struct {
	repo       Repository
	publisher  domainevent.Publisher
	txRunner   txmanager.Runner
	jwt        *authutil.JWTIssuer
	clock      clock.Clock
	accessTTL  time.Duration
	refreshTTL time.Duration
	sessions   *SessionIssuer
}

// NewService creates a Service with the given dependencies. accessTTL and
// refreshTTL control token lifetimes and must be supplied by the caller
// (typically from configuration). sessions is the shared SessionIssuer used
// by Login, CreateFirstAdmin, and (in Phase B) IdentityAdminService.
func NewService(repo Repository, publisher domainevent.Publisher, txRunner txmanager.Runner, jwt *authutil.JWTIssuer, clk clock.Clock, accessTTL, refreshTTL time.Duration, sessions *SessionIssuer) *Service {
	return &Service{
		repo:       repo,
		publisher:  publisher,
		txRunner:   txRunner,
		jwt:        jwt,
		clock:      clk,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		sessions:   sessions,
	}
}

// RegisterInput holds the data required to register a new user.
type RegisterInput struct {
	Email    string
	Password string
}

// RegisterResult is returned on successful registration.
type RegisterResult struct {
	User              *aggregate.PlatformUser
	VerificationToken string
}

// LoginInput holds the credentials for authentication plus optional metadata
// captured from the HTTP request (IP address and user-agent).
type LoginInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

// LoginResult is returned on successful login or token refresh.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *aggregate.PlatformUser
}

// CreateFirstAdminInput holds the data to bootstrap the first administrator.
type CreateFirstAdminInput struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

// Register creates a new user, generates an email verification token, and
// publishes a UserRegistered event.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*RegisterResult, error) {
	// Check for duplicate email.
	existing, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("checking email: %w", err)
	}
	if existing != nil {
		return nil, ErrEmailTaken
	}

	now := s.clock.Now()
	user, err := aggregate.NewPlatformUser(input.Email, input.Password, now)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	verification, err := aggregate.NewEmailVerification(user.ID, user.Email, now)
	if err != nil {
		return nil, fmt.Errorf("creating email verification: %w", err)
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateUser(txCtx, user); err != nil {
			return fmt.Errorf("persisting user: %w", err)
		}
		if err := s.repo.CreateEmailVerification(txCtx, verification); err != nil {
			return fmt.Errorf("persisting email verification: %w", err)
		}
		if err := domainevent.PublishAll(txCtx, s.publisher, user); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &RegisterResult{
		User:              user,
		VerificationToken: verification.Token,
	}, nil
}

// Login authenticates a user and returns JWT + refresh token. Returns
// ErrInvalidCredentials for both unknown emails and wrong passwords to avoid
// leaking user existence.
func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("finding user: %w", err)
	}

	ok, err := authutil.VerifyPassword(input.Password, user.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("verifying password: %w", err)
	}
	if !ok {
		return nil, ErrInvalidCredentials
	}

	now := s.clock.Now()
	var result *LoginResult
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		r, err := s.issueSession(txCtx, user, input.IPAddress, input.UserAgent, now)
		if err != nil {
			return err
		}
		result = r
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// issueSession is a thin wrapper that delegates to the shared SessionIssuer.
// Login and CreateFirstAdmin call this; their call sites are unchanged.
func (s *Service) issueSession(txCtx context.Context, user *aggregate.PlatformUser, ip, userAgent string, now time.Time) (*LoginResult, error) {
	return s.sessions.Issue(txCtx, user, ip, userAgent, now)
}

// RefreshToken validates the existing session, rotates the refresh token, and
// issues a new JWT access token.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*LoginResult, error) {
	session, err := s.repo.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("finding session: %w", err)
	}

	if s.clock.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	newRefreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	now := s.clock.Now()
	newSession := &aggregate.Session{
		ID:           uuid.Must(uuid.NewV7()).String(),
		UserID:       user.ID,
		RefreshToken: newRefreshToken,
		ExpiresAt:    now.Add(s.refreshTTL),
		CreatedAt:    now,
	}

	// Rotate atomically: delete old session, create new one, publish event.
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.DeleteSession(txCtx, session.ID); err != nil {
			return fmt.Errorf("deleting old session: %w", err)
		}
		if err := s.repo.CreateSession(txCtx, newSession); err != nil {
			return fmt.Errorf("persisting new session: %w", err)
		}
		if err := s.publisher.Publish(txCtx, NewTokenRefreshedEvent(user.ID, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventTokenRefreshed, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	accessToken, err := s.jwt.Sign(authutil.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	}, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("signing access token: %w", err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	}, nil
}

// SetupNeeded reports whether the platform has no administrator yet, i.e. the
// first-run admin setup wizard should be shown.
func (s *Service) SetupNeeded(ctx context.Context) (bool, error) {
	n, err := s.repo.CountAdmins(ctx)
	if err != nil {
		return false, fmt.Errorf("counting admins: %w", err)
	}
	return n == 0, nil
}

// CreateFirstAdmin bootstraps the first administrator and logs them in. It is a
// no-op once any admin exists, returning ErrSetupAlreadyCompleted. Concurrent
// calls serialize via a transaction-scoped advisory lock, guaranteeing that
// exactly one admin is ever created through this path.
func (s *Service) CreateFirstAdmin(ctx context.Context, in CreateFirstAdminInput) (*LoginResult, error) {
	now := s.clock.Now()
	user, err := aggregate.NewAdminUser(in.Email, in.Password, now)
	if err != nil {
		return nil, fmt.Errorf("creating admin user: %w", err)
	}

	var result *LoginResult
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.AcquireBootstrapLock(txCtx); err != nil {
			return err
		}
		n, err := s.repo.CountAdmins(txCtx)
		if err != nil {
			return fmt.Errorf("counting admins: %w", err)
		}
		if n > 0 {
			return ErrSetupAlreadyCompleted
		}
		if err := s.repo.CreateUser(txCtx, user); err != nil {
			return fmt.Errorf("persisting admin: %w", err)
		}
		if err := domainevent.PublishAll(txCtx, s.publisher, user); err != nil {
			return err
		}
		r, err := s.issueSession(txCtx, user, in.IPAddress, in.UserAgent, now)
		if err != nil {
			return err
		}
		result = r
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// NewUserLoggedInEvent creates an event for a successful login.
func NewUserLoggedInEvent(userID string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.UserLoggedInPayload{
		UserID: userID,
	}, now, userID)
}

// NewTokenRefreshedEvent creates an event for a successful token rotation.
func NewTokenRefreshedEvent(userID string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.TokenRefreshedPayload{
		UserID: userID,
	}, now, userID)
}
