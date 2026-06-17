package aggregate_test

import (
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInvitation(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tenant := "11111111-1111-1111-1111-111111111111"
	inv, err := aggregate.NewInvitation("Staff@Example.com", string(vo.RoleCustomer), &tenant, nil, "admin-id", now)
	require.NoError(t, err)
	assert.NotEmpty(t, inv.Token)
	assert.Len(t, inv.Token, 64) // 32 bytes hex
	assert.Equal(t, now.Add(aggregate.InvitationTTL), inv.ExpiresAt)
	assert.False(t, inv.IsExpiredAt(now))
	assert.True(t, inv.IsExpiredAt(now.Add(aggregate.InvitationTTL+time.Second)))
}

func TestNewInvitedUser(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	u, err := aggregate.NewInvitedUser("staff@example.com", "StrongP4ss", now)
	require.NoError(t, err)
	assert.True(t, u.EmailVerified)         // invited users are email-verified
	assert.Equal(t, vo.RoleCustomer, u.Role) // legacy column = customer; binding is authoritative
	assert.NotEmpty(t, u.PasswordHash)
}
