package identity

import (
	"strings"
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

func TestNewAdminUser(t *testing.T) {
	t.Run("creates admin with verified email", func(t *testing.T) {
		user, err := NewAdminUser("admin@example.com", "StrongP4ss", time.Now())
		require.NoError(t, err)
		assert.NotEmpty(t, user.ID)
		assert.Equal(t, "admin@example.com", user.Email)
		assert.Equal(t, RoleAdmin, user.Role)
		assert.True(t, user.EmailVerified)
		assert.NotEmpty(t, user.PasswordHash)
		assert.NotEqual(t, "StrongP4ss", user.PasswordHash)
	})

	t.Run("rejects invalid email", func(t *testing.T) {
		_, err := NewAdminUser("not-an-email", "StrongP4ss", time.Now())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})

	t.Run("rejects weak password", func(t *testing.T) {
		_, err := NewAdminUser("admin@example.com", "weak", time.Now())
		require.Error(t, err)
	})
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
	v, err := NewEmailVerification("user-123", "alice@example.com", time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, v.ID)
	assert.Equal(t, "user-123", v.UserID)
	assert.Equal(t, "alice@example.com", v.Email)
	assert.NotEmpty(t, v.Token)
	assert.Len(t, v.Token, VerificationTokenLen*2) // hex-encoded
	assert.False(t, v.IsExpiredAt(time.Now()))
	assert.False(t, v.CreatedAt.IsZero())
	assert.True(t, v.ExpiresAt.After(time.Now()))
}

func TestEmailVerification_IsExpired(t *testing.T) {
	v, err := NewEmailVerification("user-123", "alice@example.com", time.Now())
	require.NoError(t, err)
	v.ExpiresAt = time.Now().Add(-time.Hour)

	assert.True(t, v.IsExpiredAt(time.Now()))
}

func TestPlatformUser_ChangeDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		display string
		wantErr error
	}{
		{
			name:    "valid name",
			display: "Alice",
			wantErr: nil,
		},
		{
			name:    "empty name is allowed",
			display: "",
			wantErr: nil,
		},
		{
			name:    "exactly at max length",
			display: strings.Repeat("a", MaxDisplayNameLen),
			wantErr: nil,
		},
		{
			name:    "exceeds max length",
			display: strings.Repeat("a", MaxDisplayNameLen+1),
			wantErr: ErrDisplayNameTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &PlatformUser{ID: "u-1", UpdatedAt: time.Now()}
			before := user.UpdatedAt

			err := user.ChangeDisplayName(tt.display, before.Add(time.Second))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, user.UpdatedAt, "UpdatedAt must not change on error")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.display, user.DisplayName)
				assert.True(t, user.UpdatedAt.After(before))
			}
		})
	}
}

func TestPlatformUser_ChangePassword(t *testing.T) {
	user := &PlatformUser{
		ID:           "u-1",
		PasswordHash: "old-hash",
		UpdatedAt:    time.Now(),
	}
	before := user.UpdatedAt

	user.ChangePassword("new-hash", before.Add(time.Second))

	assert.Equal(t, "new-hash", user.PasswordHash)
	assert.True(t, user.UpdatedAt.After(before))
}

func TestPlatformUser_LinkTelegram(t *testing.T) {
	tests := []struct {
		name     string
		existing *int64
		wantErr  error
	}{
		{
			name:     "link when none linked",
			existing: nil,
			wantErr:  nil,
		},
		{
			name:     "link when already linked",
			existing: ptrInt64(999),
			wantErr:  ErrTelegramAlreadyLinked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &PlatformUser{ID: "u-1", TelegramID: tt.existing, UpdatedAt: time.Now()}
			before := user.UpdatedAt
			newID := int64(12345)

			err := user.LinkTelegram(newID, before.Add(time.Second))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, user.UpdatedAt, "UpdatedAt must not change on error")
			} else {
				require.NoError(t, err)
				require.NotNil(t, user.TelegramID)
				assert.Equal(t, newID, *user.TelegramID)
				assert.True(t, user.UpdatedAt.After(before))
			}
		})
	}
}

func TestPlatformUser_UnlinkTelegram(t *testing.T) {
	tests := []struct {
		name     string
		existing *int64
		wantErr  error
	}{
		{
			name:     "unlink when linked",
			existing: ptrInt64(12345),
			wantErr:  nil,
		},
		{
			name:     "unlink when not linked",
			existing: nil,
			wantErr:  ErrTelegramNotLinked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &PlatformUser{ID: "u-1", TelegramID: tt.existing, UpdatedAt: time.Now()}
			before := user.UpdatedAt

			err := user.UnlinkTelegram(before.Add(time.Second))

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, before, user.UpdatedAt, "UpdatedAt must not change on error")
			} else {
				require.NoError(t, err)
				assert.Nil(t, user.TelegramID)
				assert.True(t, user.UpdatedAt.After(before))
			}
		})
	}
}

func ptrInt64(v int64) *int64 { return &v }
