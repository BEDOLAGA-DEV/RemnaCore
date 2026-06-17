package aggregate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"time"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// InvitationTTL is how long an invitation token remains valid.
const InvitationTTL = 48 * time.Hour

// InvitationTokenLen is the random token byte length (hex-encoded to 2x chars).
const InvitationTokenLen = 32

// Invitation is a pending account-creation grant addressed to an email. The
// invitee may not exist yet, so there is no UserID. role_key/tenant_id carry the
// binding to apply on accept; commission_rate is set only for shop_owner invites.
type Invitation struct {
	ID             string
	Email          string
	Token          string
	RoleKey        string
	TenantID       *string
	CommissionRate *int
	InvitedBy      string
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

// NewInvitation validates the email and mints a single-use token with a TTL.
func NewInvitation(email, roleKey string, tenantID *string, commissionRate *int, invitedBy string, now time.Time) (*Invitation, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}
	tokenBytes := make([]byte, InvitationTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generating invitation token: %w", err)
	}
	return &Invitation{
		ID:             uuid.Must(uuid.NewV7()).String(),
		Email:          email,
		Token:          hex.EncodeToString(tokenBytes),
		RoleKey:        roleKey,
		TenantID:       tenantID,
		CommissionRate: commissionRate,
		InvitedBy:      invitedBy,
		ExpiresAt:      now.Add(InvitationTTL),
		CreatedAt:      now,
	}, nil
}

// IsExpiredAt reports whether the invitation has passed its expiry.
func (i *Invitation) IsExpiredAt(now time.Time) bool { return now.After(i.ExpiresAt) }

// NewInvitedUser builds an email-verified user from an accepted invitation. The
// legacy role column stays RoleCustomer; the real authorization is the role
// binding written alongside this user. email_verified=true is intentional — the
// invite token was delivered to that address (see spec §6).
func NewInvitedUser(email, password string, now time.Time) (*PlatformUser, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := authutil.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}
	user := &PlatformUser{
		ID:            uuid.Must(uuid.NewV7()).String(),
		Email:         email,
		PasswordHash:  hash,
		EmailVerified: true,
		Role:          vo.RoleCustomer,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	user.RecordEvent(domainevent.NewTyped(UserRegisteredPayload{
		UserID: user.ID,
		Email:  user.Email,
	}, now, user.ID))
	return user, nil
}
