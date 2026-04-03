package identity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// EventPublisher is an alias for the shared domainevent.Publisher so that
// existing callers referencing identity.EventPublisher continue to compile.
type EventPublisher = domainevent.Publisher

// Service implements the core identity use-cases: registration, login, email
// verification, token refresh, and profile retrieval.
type Service struct {
	repo       Repository
	publisher  domainevent.Publisher
	jwt        *authutil.JWTIssuer
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewService creates a Service with the given dependencies. accessTTL and
// refreshTTL control token lifetimes and must be supplied by the caller
// (typically from configuration).
func NewService(repo Repository, publisher domainevent.Publisher, jwt *authutil.JWTIssuer, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       repo,
		publisher:  publisher,
		jwt:        jwt,
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
	User              *PlatformUser
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
	User         *PlatformUser
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

	now := time.Now()
	user, err := NewPlatformUser(input.Email, input.Password, now)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("persisting user: %w", err)
	}

	verification := NewEmailVerification(user.ID, user.Email, now)
	if err := s.repo.CreateEmailVerification(ctx, verification); err != nil {
		return nil, fmt.Errorf("persisting email verification: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewUserRegisteredEvent(user.ID, user.Email)); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(EventUserRegistered)),
			slog.String("error", err.Error()),
		)
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

	now := time.Now()
	session := &Session{
		ID:           uuid.New().String(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(s.refreshTTL),
		CreatedAt:    now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("persisting session: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewUserLoggedInEvent(user.ID)); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(EventUserLoggedIn)),
			slog.String("error", err.Error()),
		)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// VerifyEmail validates the token, marks the user's email as verified, removes
// the verification record, and publishes an EmailVerified event.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	verification, err := s.repo.GetEmailVerification(ctx, token)
	if err != nil {
		return fmt.Errorf("finding verification: %w", err)
	}

	if verification.IsExpiredAt(time.Now()) {
		return ErrTokenExpired
	}

	user, err := s.repo.GetUserByID(ctx, verification.UserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}

	user.VerifyEmail(time.Now())
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	if err := s.repo.DeleteEmailVerification(ctx, verification.ID); err != nil {
		return fmt.Errorf("deleting verification: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewEmailVerifiedEvent(user.ID, user.Email)); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(EventEmailVerified)),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

// RefreshToken validates the existing session, rotates the refresh token, and
// issues a new JWT access token.
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*LoginResult, error) {
	session, err := s.repo.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("finding session: %w", err)
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionExpired
	}

	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	// Rotate: delete old session, create new one.
	if err := s.repo.DeleteSession(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("deleting old session: %w", err)
	}

	newRefreshToken, err := s.generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	now := time.Now()
	newSession := &Session{
		ID:           uuid.New().String(),
		UserID:       user.ID,
		RefreshToken: newRefreshToken,
		ExpiresAt:    now.Add(s.refreshTTL),
		CreatedAt:    now,
	}
	if err := s.repo.CreateSession(ctx, newSession); err != nil {
		return nil, fmt.Errorf("persisting new session: %w", err)
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
func (s *Service) GetMe(ctx context.Context, userID string) (*PlatformUser, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}
	return user, nil
}

// GetByTelegramID retrieves a user by their linked Telegram ID.
func (s *Service) GetByTelegramID(ctx context.Context, telegramID int64) (*PlatformUser, error) {
	user, err := s.repo.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("finding user by telegram id: %w", err)
	}
	return user, nil
}

// UpdateDisplayName updates the user's display name.
func (s *Service) UpdateDisplayName(ctx context.Context, userID, displayName string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	user.DisplayName = displayName
	user.UpdatedAt = time.Now()
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// LinkTelegram links a Telegram ID to the user's account.
func (s *Service) LinkTelegram(ctx context.Context, userID string, telegramID int64) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	now := time.Now()
	user.TelegramID = &telegramID
	user.UpdatedAt = now
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// UnlinkTelegram removes the Telegram ID from the user's account.
func (s *Service) UnlinkTelegram(ctx context.Context, userID string) error {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}
	user.TelegramID = nil
	user.UpdatedAt = time.Now()
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return nil
}

// ListUsers returns a paginated list of all users. Intended for admin endpoints.
func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]*PlatformUser, error) {
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

	// Remove any existing reset tokens for this user.
	if err := s.repo.DeleteUserPasswordResets(ctx, user.ID); err != nil {
		return fmt.Errorf("clearing existing resets: %w", err)
	}

	reset := NewPasswordReset(user.ID, user.Email, time.Now())
	if err := s.repo.CreatePasswordReset(ctx, reset); err != nil {
		return fmt.Errorf("persisting password reset: %w", err)
	}

	// Notification plugins listen for this event to send the actual email.
	if err := s.publisher.Publish(ctx, NewPasswordResetRequestedEvent(user.ID, user.Email, reset.Token)); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(EventPasswordResetRequested)),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

// ResetPassword validates the reset token, sets the new password, invalidates
// all existing sessions, and publishes a PasswordReset event.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	reset, err := s.repo.GetPasswordResetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPasswordResetNotFound
		}
		return fmt.Errorf("finding password reset: %w", err)
	}

	if reset.IsExpiredAt(time.Now()) {
		return ErrPasswordResetExpired
	}

	if err := validatePassword(newPassword); err != nil {
		return err
	}

	hash, err := authutil.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.repo.GetUserByID(ctx, reset.UserID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}

	now := time.Now()
	user.PasswordHash = hash
	user.UpdatedAt = now
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	// Invalidate all sessions so stolen tokens cannot be reused.
	if err := s.repo.DeleteUserSessions(ctx, user.ID); err != nil {
		return fmt.Errorf("invalidating sessions: %w", err)
	}

	// Clean up the used reset token.
	if err := s.repo.DeletePasswordReset(ctx, reset.ID); err != nil {
		return fmt.Errorf("deleting password reset: %w", err)
	}

	if err := s.publisher.Publish(ctx, NewPasswordResetEvent(user.ID)); err != nil {
		slog.Warn("failed to publish event",
			slog.String("event_type", string(EventPasswordReset)),
			slog.String("error", err.Error()),
		)
	}

	return nil
}

// RefreshTokenLen is the number of random bytes used for refresh tokens.
// The resulting hex-encoded string is twice this length (64 chars).
const RefreshTokenLen = 32

// generateRefreshToken produces a cryptographically random hex string.
func (s *Service) generateRefreshToken() (string, error) {
	b := make([]byte, RefreshTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
