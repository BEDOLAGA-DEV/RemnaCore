package aggregate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/vo"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

const (
	MinPasswordLength     = 8
	MaxDisplayNameLen     = 100
	EmailVerificationTTL  = 24 * time.Hour
	PasswordResetTTL      = 1 * time.Hour
	VerificationTokenLen  = 32 // bytes, hex-encoded = 64 chars
	PasswordResetTokenLen = 32 // bytes, hex-encoded = 64 chars
)

// PlatformUser is the aggregate root for a platform identity.
// It embeds EventRecorder to accumulate domain events during mutations.
type PlatformUser struct {
	domainevent.EventRecorder

	ID            string    `json:"id"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	DisplayName   string    `json:"display_name"`
	EmailVerified bool      `json:"email_verified"`
	TelegramID    *int64    `json:"telegram_id,omitempty"`
	Role          vo.Role   `json:"role"`
	TenantID      *string   `json:"tenant_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Session struct {
	ID           string
	UserID       string
	RefreshToken string
	IPAddress    string
	UserAgent    string
	ExpiresAt    time.Time
	CreatedAt    time.Time
}

type EmailVerification struct {
	ID        string
	UserID    string
	Email     string
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewPlatformUser validates inputs, hashes the password, and returns a new
// PlatformUser with a generated UUID and RoleCustomer.
func NewPlatformUser(email, password string, now time.Time) (*PlatformUser, error) {
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
		EmailVerified: false,
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

// NewAdminUser builds the bootstrap administrator: role=admin with a
// pre-verified email (the first admin has no email-verification flow). It
// reuses the same email/password validation and hashing as NewPlatformUser.
func NewAdminUser(email, password string, now time.Time) (*PlatformUser, error) {
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
		Role:          vo.RoleAdmin,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	user.RecordEvent(domainevent.NewTyped(UserRegisteredPayload{
		UserID: user.ID,
		Email:  user.Email,
	}, now, user.ID))
	return user, nil
}

// VerifyEmail marks the user's email as verified and updates the timestamp.
func (u *PlatformUser) VerifyEmail(now time.Time) {
	u.EmailVerified = true
	u.UpdatedAt = now
	u.RecordEvent(domainevent.NewTyped(EmailVerifiedPayload{
		UserID: u.ID,
		Email:  u.Email,
	}, now, u.ID))
}

// ChangeDisplayName validates and sets the user's display name.
func (u *PlatformUser) ChangeDisplayName(name string, now time.Time) error {
	if len(name) > MaxDisplayNameLen {
		return ErrDisplayNameTooLong
	}
	u.DisplayName = name
	u.UpdatedAt = now
	return nil
}

// ChangePassword sets a pre-hashed password on the user. Callers are
// responsible for hashing the raw password before invoking this method.
func (u *PlatformUser) ChangePassword(newHash string, now time.Time) {
	u.PasswordHash = newHash
	u.UpdatedAt = now
	u.RecordEvent(domainevent.NewTyped(PasswordChangedPayload{
		UserID: u.ID,
	}, now, u.ID))
}

// LinkTelegram associates a Telegram account with the user. Returns an error
// if a Telegram account is already linked.
func (u *PlatformUser) LinkTelegram(telegramID int64, now time.Time) error {
	if u.TelegramID != nil {
		return ErrTelegramAlreadyLinked
	}
	u.TelegramID = &telegramID
	u.UpdatedAt = now
	u.RecordEvent(domainevent.NewTyped(TelegramLinkedPayload{
		UserID:     u.ID,
		TelegramID: telegramID,
	}, now, u.ID))
	return nil
}

// UnlinkTelegram removes the Telegram association. Returns an error if no
// Telegram account is currently linked.
func (u *PlatformUser) UnlinkTelegram(now time.Time) error {
	if u.TelegramID == nil {
		return ErrTelegramNotLinked
	}
	oldTelegramID := *u.TelegramID
	u.TelegramID = nil
	u.UpdatedAt = now
	u.RecordEvent(domainevent.NewTyped(TelegramUnlinkedPayload{
		UserID:     u.ID,
		TelegramID: oldTelegramID,
	}, now, u.ID))
	return nil
}

// ValidatePassword checks that the password meets minimum complexity
// requirements: at least MinPasswordLength characters, contains an uppercase
// letter, a lowercase letter, and a digit.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	var hasUpper, hasLower, hasDigit bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		}
	}

	if !hasUpper || !hasLower || !hasDigit {
		return ErrPasswordTooWeak
	}

	return nil
}

// NewEmailVerification generates a new email verification record with a
// cryptographically random hex token and a TTL-based expiration.
func NewEmailVerification(userID, email string, now time.Time) (*EmailVerification, error) {
	tokenBytes := make([]byte, VerificationTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generating verification token: %w", err)
	}

	return &EmailVerification{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    userID,
		Email:     email,
		Token:     hex.EncodeToString(tokenBytes),
		ExpiresAt: now.Add(EmailVerificationTTL),
		CreatedAt: now,
	}, nil
}

// IsExpiredAt returns true if the verification token has passed its expiration
// relative to the given time.
func (v *EmailVerification) IsExpiredAt(now time.Time) bool {
	return now.After(v.ExpiresAt)
}

// PasswordReset represents a token-based password reset request.
type PasswordReset struct {
	ID        string
	UserID    string
	Email     string
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewPasswordReset generates a new password reset record with a
// cryptographically random hex token and a TTL-based expiration.
func NewPasswordReset(userID, email string, now time.Time) (*PasswordReset, error) {
	tokenBytes := make([]byte, PasswordResetTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generating password reset token: %w", err)
	}

	return &PasswordReset{
		ID:        uuid.Must(uuid.NewV7()).String(),
		UserID:    userID,
		Email:     email,
		Token:     hex.EncodeToString(tokenBytes),
		ExpiresAt: now.Add(PasswordResetTTL),
		CreatedAt: now,
	}, nil
}

// IsExpiredAt returns true if the password reset token has passed its expiration
// relative to the given time.
func (pr *PasswordReset) IsExpiredAt(now time.Time) bool {
	return now.After(pr.ExpiresAt)
}
