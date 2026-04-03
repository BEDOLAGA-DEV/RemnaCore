package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/mail"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
)

const (
	RoleCustomer Role = "customer"
	RoleReseller Role = "reseller"
	RoleAdmin    Role = "admin"

	MinPasswordLength    = 8
	EmailVerificationTTL = 24 * time.Hour
	PasswordResetTTL     = 1 * time.Hour
	VerificationTokenLen = 32 // bytes, hex-encoded = 64 chars
	PasswordResetTokenLen = 32 // bytes, hex-encoded = 64 chars
)

type Role string

type PlatformUser struct {
	ID            string
	Email         string
	PasswordHash  string
	DisplayName   string
	EmailVerified bool
	TelegramID    *int64
	Role          Role
	TenantID      *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Session struct {
	ID           string
	UserID       string
	RefreshToken string
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
func NewPlatformUser(email, password string) (*PlatformUser, error) {
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := authutil.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now()
	return &PlatformUser{
		ID:            uuid.New().String(),
		Email:         email,
		PasswordHash:  hash,
		EmailVerified: false,
		Role:          RoleCustomer,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// VerifyEmail marks the user's email as verified and updates the timestamp.
func (u *PlatformUser) VerifyEmail() {
	u.EmailVerified = true
	u.UpdatedAt = time.Now()
}

// validatePassword checks that the password meets minimum complexity
// requirements: at least MinPasswordLength characters, contains an uppercase
// letter, a lowercase letter, and a digit.
func validatePassword(password string) error {
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
func NewEmailVerification(userID, email string) *EmailVerification {
	tokenBytes := make([]byte, VerificationTokenLen)
	// crypto/rand.Read always returns len(p) bytes on supported platforms;
	// a failure here indicates a broken runtime, so panic is appropriate.
	if _, err := rand.Read(tokenBytes); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}

	now := time.Now()
	return &EmailVerification{
		ID:        uuid.New().String(),
		UserID:    userID,
		Email:     email,
		Token:     hex.EncodeToString(tokenBytes),
		ExpiresAt: now.Add(EmailVerificationTTL),
		CreatedAt: now,
	}
}

// IsExpired returns true if the verification token has passed its expiration.
func (v *EmailVerification) IsExpired() bool {
	return time.Now().After(v.ExpiresAt)
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
func NewPasswordReset(userID, email string) *PasswordReset {
	tokenBytes := make([]byte, PasswordResetTokenLen)
	if _, err := rand.Read(tokenBytes); err != nil {
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}

	now := time.Now()
	return &PasswordReset{
		ID:        uuid.New().String(),
		UserID:    userID,
		Email:     email,
		Token:     hex.EncodeToString(tokenBytes),
		ExpiresAt: now.Add(PasswordResetTTL),
		CreatedAt: now,
	}
}

// IsExpired returns true if the password reset token has passed its expiration.
func (pr *PasswordReset) IsExpired() bool {
	return time.Now().After(pr.ExpiresAt)
}
