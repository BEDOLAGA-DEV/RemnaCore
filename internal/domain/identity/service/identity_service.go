package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/telegramauth"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// RefreshTokenLen is the number of random bytes used for refresh tokens.
// The resulting hex-encoded string is twice this length (64 chars).
const RefreshTokenLen = 32

// dummyPasswordHash is a valid argon2 hash verified against on unknown-email
// login so the response timing is constant whether or not the email exists
// (both paths pay the argon2 cost). Computed once at package init.
var dummyPasswordHash = func() string {
	h, err := authutil.HashPassword("constant-time-login-placeholder")
	if err != nil {
		panic(fmt.Sprintf("identity: precompute dummy password hash: %v", err))
	}
	return h
}()

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

	// Invitation operations.
	CreateInvitation(ctx context.Context, inv *aggregate.Invitation) error
	GetInvitationByToken(ctx context.Context, token string) (*aggregate.Invitation, error)
	GetInvitationByID(ctx context.Context, id string) (*aggregate.Invitation, error)
	DeleteInvitation(ctx context.Context, id string) error
	// ListInvitations returns all invitations (all=true) or only those scoped to
	// the given tenantIDs.
	ListInvitations(ctx context.Context, tenantIDs []string, all bool) ([]*aggregate.Invitation, error)
	DeleteExpiredInvitations(ctx context.Context) (int64, error)
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
// TelegramReplayGuard records used Telegram initData so a captured payload
// cannot be replayed within its validity window. Consume atomically claims a
// nonce: it returns fresh=true the first time and fresh=false on any later call
// for the same nonce (until expiresAt passes and the row is pruned).
type TelegramReplayGuard interface {
	Consume(ctx context.Context, nonce string, expiresAt time.Time) (fresh bool, err error)
}

// PlatformAdminGranter grants the platform_admin global role binding to a user.
// CreateFirstAdmin uses it so the bootstrap admin has RBAC authority IMMEDIATELY
// (permission gates read RBAC bindings, not the legacy role column) instead of
// only after the next boot-time backfill. Must run inside the caller's tx.
type PlatformAdminGranter interface {
	GrantPlatformAdmin(ctx context.Context, userID, grantedBy string) error
}

type Service struct {
	repo        Repository
	publisher   domainevent.Publisher
	txRunner    txmanager.Runner
	jwt         *authutil.JWTIssuer
	clock       clock.Clock
	accessTTL   time.Duration
	refreshTTL  time.Duration
	sessions    *SessionIssuer
	replayGuard TelegramReplayGuard  // optional; nil disables the replay check
	adminGrant  PlatformAdminGranter // optional; nil skips the RBAC binding
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

// WithTelegramReplayGuard installs a replay guard for Telegram Mini App logins
// and returns the same Service for chaining. Wiring calls this after
// construction; when unset, LoginViaTelegramWebApp skips the single-use check
// (validation-only, as before).
func (s *Service) WithTelegramReplayGuard(g TelegramReplayGuard) *Service {
	s.replayGuard = g
	return s
}

// WithPlatformAdminGranter installs the RBAC granter used by CreateFirstAdmin to
// give the first admin its platform_admin binding atomically. Returns the same
// Service for chaining; when unset, the first admin gets its binding only on the
// next boot-time RBAC backfill.
func (s *Service) WithPlatformAdminGranter(g PlatformAdminGranter) *Service {
	s.adminGrant = g
	return s
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
	now := s.clock.Now()
	user, err := aggregate.NewPlatformUser(input.Email, input.Password, now)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	verification, err := aggregate.NewEmailVerification(user.ID, user.Email, now)
	if err != nil {
		return nil, fmt.Errorf("creating email verification: %w", err)
	}

	// Platform sentinel: the duplicate-email check and CreateUser both touch
	// identity.platform_users (FORCE RLS); pre-auth registration has no tenant
	// context, so without it the dup-check is blind (RLS-hidden) and the insert's
	// scope is undefined. The check runs inside the same tx to close the TOCTOU.
	if err := s.txRunner.RunInTx(tenantctx.WithPlatformScope(ctx), func(txCtx context.Context) error {
		existing, cerr := s.repo.GetUserByEmail(txCtx, input.Email)
		if cerr != nil && !errors.Is(cerr, ErrNotFound) {
			return fmt.Errorf("checking email: %w", cerr)
		}
		if existing != nil {
			return ErrEmailTaken
		}
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
	// The email lookup reads identity.platform_users, which is FORCE RLS. Login
	// is pre-auth (no tenant known yet) and must see users across every tenant,
	// so it runs under the platform sentinel; otherwise the RLS policy hides
	// every row and login always fails as "invalid credentials".
	var user *aggregate.PlatformUser
	if err := s.txRunner.RunInTx(tenantctx.WithPlatformScope(ctx), func(txCtx context.Context) error {
		u, uerr := s.repo.GetUserByEmail(txCtx, input.Email)
		if uerr != nil {
			return uerr
		}
		user = u
		return nil
	}); err != nil {
		if errors.Is(err, ErrNotFound) {
			// Equalize timing: an unknown email would otherwise return instantly
			// while a known email pays the argon2 cost, giving an existence oracle.
			// Run a verify against a fixed dummy hash and discard the result.
			_, _ = authutil.VerifyPassword(input.Password, dummyPasswordHash)
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

	// GetUserByID reads identity.platform_users (FORCE RLS). Refresh is pre-auth
	// (the JWT is expired/absent, no tenant scope), so it runs under the platform
	// sentinel — otherwise RLS hides the user row and every refresh fails,
	// logging the operator out when the access token expires.
	var user *aggregate.PlatformUser
	if err := s.txRunner.RunInTx(tenantctx.WithPlatformScope(ctx), func(txCtx context.Context) error {
		u, uerr := s.repo.GetUserByID(txCtx, session.UserID)
		if uerr != nil {
			return uerr
		}
		user = u
		return nil
	}); err != nil {
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
	// CountAdmins reads identity.platform_users (FORCE RLS). This is a public,
	// pre-auth endpoint with no tenant context, so it must run under the platform
	// sentinel — otherwise RLS hides the admin rows and the count is always 0,
	// which wrongly reports "setup needed" forever (and traps operators on the
	// /setup wizard after logout instead of showing /login).
	var n int64
	if err := s.txRunner.RunInTx(tenantctx.WithPlatformScope(ctx), func(txCtx context.Context) error {
		var cerr error
		n, cerr = s.repo.CountAdmins(txCtx)
		return cerr
	}); err != nil {
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
	// Runs under the platform sentinel: CountAdmins / CreateUser touch
	// identity.platform_users (FORCE RLS), and without it the count would always
	// see 0 admins (RLS-hidden) and create a duplicate admin on every call.
	if err := s.txRunner.RunInTx(tenantctx.WithPlatformScope(ctx), func(txCtx context.Context) error {
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
		// Grant the platform_admin RBAC binding in the SAME tx. Permission gates
		// (ShopResolver/RequirePermission) authorize off RBAC bindings, not the
		// legacy role column, so without this the first admin can log in but is
		// denied every admin API (empty /users, etc.) until the next restart.
		if s.adminGrant != nil {
			if err := s.adminGrant.GrantPlatformAdmin(txCtx, user.ID, user.ID); err != nil {
				return fmt.Errorf("granting platform_admin: %w", err)
			}
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

// RegisterViaTelegram finds-or-creates a Telegram-native customer for the given
// shop. It MUST run inside RunInTx(WithTenantID) so the app.tenant_id GUC is set:
// the GetUserByTelegramID read is then RLS-scoped to the tenant, and the insert's
// tenant_id passes the platform_users RLS WITH CHECK.
func (s *Service) RegisterViaTelegram(ctx context.Context, telegramID int64, tenantID, displayName string) (*aggregate.PlatformUser, error) {
	var result *aggregate.PlatformUser
	err := s.txRunner.RunInTx(tenantctx.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
		existing, err := s.repo.GetUserByTelegramID(txCtx, telegramID)
		if err == nil && existing != nil {
			result = existing
			return nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("lookup telegram user: %w", err)
		}
		user, err := aggregate.NewTelegramUser(telegramID, tenantID, displayName, s.clock.Now())
		if err != nil {
			return fmt.Errorf("creating telegram user: %w", err)
		}
		if err := s.repo.CreateUser(txCtx, user); err != nil {
			if errors.Is(err, ErrAlreadyExists) { // race: another /start created it
				existing, gerr := s.repo.GetUserByTelegramID(txCtx, telegramID)
				if gerr != nil {
					return fmt.Errorf("refetch after race: %w", gerr)
				}
				result = existing
				return nil
			}
			return fmt.Errorf("persisting telegram user: %w", err)
		}
		if err := domainevent.PublishAll(txCtx, s.publisher, user); err != nil {
			return err
		}
		result = user
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// LoginViaTelegramWebApp validates a Telegram Mini App initData payload, finds
// or creates the Telegram user via RegisterViaTelegram, then issues a session
// inside a tenant-scoped transaction. botToken is resolved by the gateway
// (from the reseller/shop config) and passed in so that the identity domain does
// not import the reseller domain.
func (s *Service) LoginViaTelegramWebApp(ctx context.Context, initData, botToken, tenantID, ip, ua string) (*LoginResult, error) {
	if botToken == "" {
		return nil, ErrTelegramAuthUnavailable
	}

	tgUser, err := telegramauth.ValidateWebAppInitData(initData, botToken, s.clock.Now(), telegramauth.DefaultInitDataMaxAge)
	if err != nil {
		return nil, fmt.Errorf("validate telegram initData: %w", err)
	}

	// Single-use enforcement: a valid signature only proves the payload came from
	// Telegram, not that it is fresh. Claim a nonce derived from the whole
	// initData so a captured payload cannot be replayed within its 24h window.
	if s.replayGuard != nil {
		sum := sha256.Sum256([]byte(initData))
		nonce := hex.EncodeToString(sum[:])
		expiresAt := s.clock.Now().Add(telegramauth.DefaultInitDataMaxAge)
		fresh, cerr := s.replayGuard.Consume(ctx, nonce, expiresAt)
		if cerr != nil {
			return nil, fmt.Errorf("telegram replay guard: %w", cerr)
		}
		if !fresh {
			return nil, ErrTelegramInitDataReplayed
		}
	}

	displayName := strings.TrimSpace(tgUser.FirstName + " " + tgUser.LastName)
	if displayName == "" {
		displayName = tgUser.Username
	}

	user, err := s.RegisterViaTelegram(ctx, tgUser.ID, tenantID, displayName)
	if err != nil {
		return nil, fmt.Errorf("register telegram user: %w", err)
	}

	var result *LoginResult
	if err := s.txRunner.RunInTx(tenantctx.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
		r, ierr := s.issueSession(txCtx, user, ip, ua, s.clock.Now())
		if ierr != nil {
			return ierr
		}
		result = r
		return nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}
