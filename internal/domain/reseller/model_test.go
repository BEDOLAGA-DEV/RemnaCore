package reseller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTenant(t *testing.T) {
	tenant := NewTenant("Acme VPN", "acme.vpn.com", "owner-123")

	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, "Acme VPN", tenant.Name)
	assert.Equal(t, "acme.vpn.com", tenant.Domain)
	assert.Equal(t, "owner-123", tenant.OwnerUserID)
	assert.True(t, tenant.IsActive)
	assert.False(t, tenant.CreatedAt.IsZero())
	assert.False(t, tenant.UpdatedAt.IsZero())
}

func TestTenant_GenerateAPIKey(t *testing.T) {
	tenant := NewTenant("Acme VPN", "acme.vpn.com", "owner-123")

	plainKey, err := tenant.GenerateAPIKey()
	require.NoError(t, err)

	assert.Len(t, plainKey, APIKeyLen*2) // hex-encoded
	assert.NotEmpty(t, tenant.APIKeyHash)
	assert.NotEqual(t, plainKey, tenant.APIKeyHash) // hash != plain key
}

func TestTenant_GenerateAPIKey_HashConsistency(t *testing.T) {
	tenant := NewTenant("Acme VPN", "acme.vpn.com", "owner-123")

	plainKey, err := tenant.GenerateAPIKey()
	require.NoError(t, err)

	// Hashing the same key should produce the same hash.
	computed := HashAPIKey(plainKey)
	assert.Equal(t, tenant.APIKeyHash, computed)
}

func TestHashAPIKey_DifferentKeysProduceDifferentHashes(t *testing.T) {
	hash1 := HashAPIKey("key-one")
	hash2 := HashAPIKey("key-two")

	assert.NotEqual(t, hash1, hash2)
}

func TestNewResellerAccount_Valid(t *testing.T) {
	account, err := NewResellerAccount("tenant-1", "user-1", 20)
	require.NoError(t, err)

	assert.NotEmpty(t, account.ID)
	assert.Equal(t, "tenant-1", account.TenantID)
	assert.Equal(t, "user-1", account.UserID)
	assert.Equal(t, 20, account.CommissionRate)
	assert.Equal(t, int64(0), account.Balance)
	assert.False(t, account.CreatedAt.IsZero())
}

func TestNewResellerAccount_ZeroRate(t *testing.T) {
	account, err := NewResellerAccount("tenant-1", "user-1", 0)
	require.NoError(t, err)
	assert.Equal(t, 0, account.CommissionRate)
}

func TestNewResellerAccount_MaxRate(t *testing.T) {
	account, err := NewResellerAccount("tenant-1", "user-1", 100)
	require.NoError(t, err)
	assert.Equal(t, 100, account.CommissionRate)
}

func TestNewResellerAccount_RateTooLow(t *testing.T) {
	_, err := NewResellerAccount("tenant-1", "user-1", -1)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCommissionRate)
}

func TestNewResellerAccount_RateTooHigh(t *testing.T) {
	_, err := NewResellerAccount("tenant-1", "user-1", 101)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCommissionRate)
}

func TestNewCommission(t *testing.T) {
	commission := NewCommission("reseller-1", "sale-abc", 10000, 15, "USD")

	assert.NotEmpty(t, commission.ID)
	assert.Equal(t, "reseller-1", commission.ResellerID)
	assert.Equal(t, "sale-abc", commission.SaleID)
	assert.Equal(t, int64(1500), commission.Amount) // 15% of 10000
	assert.Equal(t, "USD", commission.Currency)
	assert.Equal(t, CommissionPending, commission.Status)
	assert.Nil(t, commission.PaidAt)
	assert.False(t, commission.CreatedAt.IsZero())
}

func TestNewCommission_ZeroRate(t *testing.T) {
	commission := NewCommission("reseller-1", "sale-abc", 10000, 0, "USD")
	assert.Equal(t, int64(0), commission.Amount)
}

func TestNewCommission_FullRate(t *testing.T) {
	commission := NewCommission("reseller-1", "sale-abc", 10000, 100, "USD")
	assert.Equal(t, int64(10000), commission.Amount)
}

func TestNewCommission_RoundsDown(t *testing.T) {
	// 33% of 100 cents = 33 cents (integer division rounds down).
	commission := NewCommission("reseller-1", "sale-abc", 100, 33, "USD")
	assert.Equal(t, int64(33), commission.Amount)
}
