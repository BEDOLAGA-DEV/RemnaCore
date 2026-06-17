package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// sessionRepo is the narrow persistence interface required by SessionIssuer.
// *postgres.IdentityRepository satisfies this via structural typing.
type sessionRepo interface {
	CreateSession(ctx context.Context, s *aggregate.Session) error
}

// SessionIssuer signs an access token, persists a session, and publishes the
// login event — the single source of session issuance, shared by Service.Login,
// Service.CreateFirstAdmin, and IdentityAdminService.AcceptInvitation.
type SessionIssuer struct {
	repo       sessionRepo
	publisher  domainevent.Publisher
	jwt        *authutil.JWTIssuer
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewSessionIssuer constructs a SessionIssuer. repo must satisfy sessionRepo
// (CreateSession); in practice this is identity.Repository or the postgres impl.
func NewSessionIssuer(repo sessionRepo, pub domainevent.Publisher, jwt *authutil.JWTIssuer, accessTTL, refreshTTL time.Duration) *SessionIssuer {
	return &SessionIssuer{
		repo:       repo,
		publisher:  pub,
		jwt:        jwt,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// Issue performs token and session issuance within the provided (transaction)
// context. It signs a JWT access token, generates a refresh token, persists the
// session, and publishes a UserLoggedIn event.
func (si *SessionIssuer) Issue(txCtx context.Context, user *aggregate.PlatformUser, ip, userAgent string, now time.Time) (*LoginResult, error) {
	accessToken, err := si.jwt.Sign(authutil.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
	}, si.accessTTL)
	if err != nil {
		return nil, fmt.Errorf("signing access token: %w", err)
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generating refresh token: %w", err)
	}

	session := &aggregate.Session{
		ID:           uuid.Must(uuid.NewV7()).String(),
		UserID:       user.ID,
		RefreshToken: refreshToken,
		IPAddress:    ip,
		UserAgent:    userAgent,
		ExpiresAt:    now.Add(si.refreshTTL),
		CreatedAt:    now,
	}

	if err := si.repo.CreateSession(txCtx, session); err != nil {
		return nil, fmt.Errorf("persisting session: %w", err)
	}
	if err := si.publisher.Publish(txCtx, NewUserLoggedInEvent(user.ID, now)); err != nil {
		return nil, fmt.Errorf("publishing %s: %w", aggregate.EventUserLoggedIn, err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

// generateRefreshToken produces a cryptographically random hex string.
// Package-level so it is shared with RefreshToken in identity_service.go.
func generateRefreshToken() (string, error) {
	b := make([]byte, RefreshTokenLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
