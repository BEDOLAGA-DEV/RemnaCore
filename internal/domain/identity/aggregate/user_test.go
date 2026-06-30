package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/vo"
)

func TestNewTelegramUser(t *testing.T) {
	now := time.Now()
	u1, err := NewTelegramUser(12345, "11111111-1111-1111-1111-111111111111", "Alice", now)
	require.NoError(t, err)
	require.NotNil(t, u1.TelegramID)
	require.Equal(t, int64(12345), *u1.TelegramID)
	require.NotNil(t, u1.TenantID)
	require.Equal(t, "11111111-1111-1111-1111-111111111111", *u1.TenantID)
	require.Equal(t, vo.RoleCustomer, u1.Role)
	require.Equal(t, "Alice", u1.DisplayName)
	require.NotEmpty(t, u1.PasswordHash)
	require.Contains(t, u1.Email, "tg-12345-")           // synthetic, includes telegram id
	require.Contains(t, u1.Email, "@"+TelegramSyntheticEmailDomain)

	// Same telegram id in a DIFFERENT tenant → different synthetic email (no email_lower collision).
	u2, err := NewTelegramUser(12345, "22222222-2222-2222-2222-222222222222", "Alice", now)
	require.NoError(t, err)
	require.NotEqual(t, u1.Email, u2.Email)
}
