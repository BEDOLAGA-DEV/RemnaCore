package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
// When to split: if Service exceeds ~500 lines or gains responsibilities beyond
// authentication and profile management (e.g., RBAC policy engine, OAuth2
// provider integration), extract focused services (e.g., AuthService,
// ProfileService) that each own a subset of the use-cases.
type Service struct {
	repo       Repository
	publisher  domainevent.Publisher
	txRunner   txmanager.Runner
	jwt        *authutil.JWTIssuer
	clock      clock.Clock
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewService creates a Service with the given dependencies. accessTTL and
// refreshTTL control token lifetimes and must be supplied by the caller
// (typically from configuration).
func NewService(repo Repository, publisher domainevent.Publisher, txRunner txmanager.Runner, jwt *authutil.JWTIssuer, clk clock.Clock, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       repo,
		publisher:  publisher,
		txRunner:   txRunner,
		jwt:        jwt,
		clock:      clk,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
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

// LoginInput holds the credentials for authentication.
type LoginInput struct {
	Email    string
	Password string
}

// LoginResult is returned on successful login or token refresh.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *aggregate.PlatformUser
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

	accessToken, err := s.jwt.Sign(authutil.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   string(user.Role),
	}, s.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("signing access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	now := s.clock.Now()
	session := &aggregate.Session{
		ID:           uuid.Must(uuid.NewV7()).String(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(s.refreshTTL),
		CreatedAt:    now,
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateSession(txCtx, session); err != nil {
			return fmt.Errorf("persisting session: %w", err)
		}
		if err := s.publisher.Publish(txCtx, NewUserLoggedInEvent(user.ID, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventUserLoggedIn, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// VerifyEmail validates the token, marks the user's email as verified, removes
// the verification record, and publishes an EmailVerified event. The user read
// (with FOR UPDATE lock), mutation, update, and event publish are all inside a
// single transaction to prevent TOCTOU races.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	// Verification token lookup is read-only and not subject to concurrent mutation.
	verification, err := s.repo.GetEmailVerification(ctx, token)
	if err != nil {
		return fmt.Errorf("finding verification: %w", err)
	}

	if verification.IsExpiredAt(s.clock.Now()) {
		return ErrTokenExpired
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, verification.UserID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}

		now := s.clock.Now()
		user.VerifyEmail(now)

		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		if err := s.repo.DeleteEmailVerification(txCtx, verification.ID); err != nil {
			return fmt.Errorf("deleting verification: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
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

	newRefreshToken, err := s.generateRefreshToken()
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
		Role:   string(user.Role),
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

// GetMe retrieves the authenticated user's profile by ID.
func (s *Service) GetMe(ctx context.Context, userID string) (*aggregate.PlatformUser, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}
	return user, nil
}

// GetByTelegramID retrieves a user by their linked Telegram ID.
func (s *Service) GetByTelegramID(ctx context.Context, telegramID int64) (*aggregate.PlatformUser, error) {
	user, err := s.repo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("finding user by telegram id: %w", err)
	}
	return user, nil
}

// UpdateDisplayName updates the user's display name. The read (with FOR UPDATE
// lock), mutation, and update are all inside a single transaction to prevent
// TOCTOU races.
func (s *Service) UpdateDisplayName(ctx context.Context, userID, displayName string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if err := user.ChangeDisplayName(displayName, s.clock.Now()); err != nil {
			return err
		}
		return s.repo.UpdateUser(txCtx, user)
	})
}

// LinkTelegram links a Telegram ID to the user's account. The read (with FOR
// UPDATE lock), mutation, update, and event publish are all inside a single
// transaction to prevent TOCTOU races.
func (s *Service) LinkTelegram(ctx context.Context, userID string, telegramID int64) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if err := user.LinkTelegram(telegramID, s.clock.Now()); err != nil {
			return err
		}
		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// UnlinkTelegram removes the Telegram ID from the user's account. The read
// (with FOR UPDATE lock), mutation, update, and event publish are all inside a
// single transaction to prevent TOCTOU races.
func (s *Service) UnlinkTelegram(ctx context.Context, userID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}
		if err := user.UnlinkTelegram(s.clock.Now()); err != nil {
			return err
		}
		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// ListUsers returns a paginated list of all users. Intended for admin endpoints.
func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]*aggregate.PlatformUser, error) {
	users, err := s.repo.ListUsers(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	return users, nil
}

// RequestPasswordReset generates a password reset token for the given email and
// publishes a PasswordResetRequested event. If the email is not found, no error
// is returned to prevent user enumeration.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Silently succeed to prevent email enumeration.
			return nil
		}
		return fmt.Errorf("finding user by email: %w", err)
	}

	now := s.clock.Now()
	reset, err := aggregate.NewPasswordReset(user.ID, user.Email, now)
	if err != nil {
		return fmt.Errorf("creating password reset: %w", err)
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		// Remove any existing reset tokens for this user.
		if err := s.repo.DeleteUserPasswordResets(txCtx, user.ID); err != nil {
			return fmt.Errorf("clearing existing resets: %w", err)
		}
		if err := s.repo.CreatePasswordReset(txCtx, reset); err != nil {
			return fmt.Errorf("persisting password reset: %w", err)
		}
		// Notification plugins listen for this event to send the actual email.
		if err := s.publisher.Publish(txCtx, NewPasswordResetRequestedEvent(user.ID, user.Email, reset.Token, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventPasswordResetRequested, err)
		}
		return nil
	})
}

// ResetPassword validates the reset token, sets the new password, invalidates
// all existing sessions, and publishes a PasswordReset event. The user read
// (with FOR UPDATE lock), mutation, update, session invalidation, and event
// publish are all inside a single transaction to prevent TOCTOU races.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Token lookup and validation are read-only and not subject to concurrent mutation.
	reset, err := s.repo.GetPasswordResetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPasswordResetNotFound
		}
		return fmt.Errorf("finding password reset: %w", err)
	}

	if reset.IsExpiredAt(s.clock.Now()) {
		return ErrPasswordResetExpired
	}

	if err := aggregate.ValidatePassword(newPassword); err != nil {
		return err
	}

	hash, err := authutil.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.GetUserByIDForUpdate(txCtx, reset.UserID)
		if err != nil {
			return fmt.Errorf("finding user: %w", err)
		}

		now := s.clock.Now()
		user.ChangePassword(hash, now)

		if err := s.repo.UpdateUser(txCtx, user); err != nil {
			return fmt.Errorf("updating user: %w", err)
		}
		if err := s.repo.DeleteUserSessions(txCtx, user.ID); err != nil {
			return fmt.Errorf("invalidating sessions: %w", err)
		}
		if err := s.repo.DeletePasswordReset(txCtx, reset.ID); err != nil {
			return fmt.Errorf("deleting password reset: %w", err)
		}
		if err := s.publisher.Publish(txCtx, NewPasswordResetEvent(user.ID, now)); err != nil {
			return fmt.Errorf("publishing %s: %w", aggregate.EventPasswordReset, err)
		}
		return domainevent.PublishAll(txCtx, s.publisher, user)
	})
}

// generateRefreshToken produces a cryptographically random hex string.
func (s *Service) generateRefreshToken() (string, error) {
	b := make([]byte, RefreshTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
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

// NewPasswordResetRequestedEvent creates an event when a user requests a
// password reset. Notification plugins listen for this to send the reset email.
func NewPasswordResetRequestedEvent(userID, email, token string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.PasswordResetRequestedPayload{
		UserID: userID,
		Email:  email,
		Token:  token,
	}, now, userID)
}

// NewPasswordResetEvent creates an event when a password has been successfully
// reset.
func NewPasswordResetEvent(userID string, now time.Time) domainevent.Event {
	return domainevent.NewTyped(aggregate.PasswordResetPayload{
		UserID: userID,
	}, now, userID)
}
