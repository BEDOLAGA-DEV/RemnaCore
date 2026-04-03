package identity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPlatformUser_Valid(t *testing.T) {
	user, err := NewPlatformUser("alice@example.com", "StrongP4ss", time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, user.ID)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.NotEmpty(t, user.PasswordHash)
	assert.Equal(t, RoleCustomer, user.Role)
	assert.False(t, user.EmailVerified)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestNewPlatformUser_InvalidEmail(t *testing.T) {
	_, err := NewPlatformUser("not-an-email", "StrongP4ss", time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestNewPlatformUser_WeakPassword(t *testing.T) {
	_, err := NewPlatformUser("alice@example.com", "123", time.Now())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "password")
}

func TestPlatformUser_VerifyEmail(t *testing.T) {
	user, err := NewPlatformUser("alice@example.com", "StrongP4ss", time.Now())
	require.NoError(t, err)
	assert.False(t, user.EmailVerified)

	before := user.UpdatedAt
	time.Sleep(time.Millisecond) // ensure time advances
	user.VerifyEmail(time.Now())

	assert.True(t, user.EmailVerified)
	assert.True(t, user.UpdatedAt.After(before))
}

func TestEmailVerification_Generate(t *testing.T) {
	v := NewEmailVerification("user-123", "alice@example.com", time.Now())

	assert.NotEmpty(t, v.ID)
	assert.Equal(t, "user-123", v.UserID)
	assert.Equal(t, "alice@example.com", v.Email)
	assert.NotEmpty(t, v.Token)
	assert.Len(t, v.Token, VerificationTokenLen*2) // hex-encoded
	assert.False(t, v.IsExpired())
	assert.False(t, v.CreatedAt.IsZero())
	assert.True(t, v.ExpiresAt.After(time.Now()))
}

func TestEmailVerification_IsExpired(t *testing.T) {
	v := NewEmailVerification("user-123", "alice@example.com", time.Now())
	v.ExpiresAt = time.Now().Add(-time.Hour)

	assert.True(t, v.IsExpired())
}
