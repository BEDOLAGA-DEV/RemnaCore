package reseller

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/money"
)

func strPtr(s string) *string { return &s }

func TestNewTenant(t *testing.T) {
	owner := "owner-123"
	tenant := NewTenant("Acme VPN", "acme.vpn.com", &owner, time.Now())

	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, "Acme VPN", tenant.Name)
	assert.Equal(t, "acme.vpn.com", tenant.Domain)
	assert.Equal(t, &owner, tenant.OwnerUserID)
	assert.True(t, tenant.IsActive)
	assert.False(t, tenant.CreatedAt.IsZero())
	assert.False(t, tenant.UpdatedAt.IsZero())
}

func TestTenant_GenerateAPIKey(t *testing.T) {
	tenant := NewTenant("Acme VPN", "acme.vpn.com", strPtr("owner-123"), time.Now())

	plainKey, err := tenant.GenerateAPIKey(time.Now())
	require.NoError(t, err)

	assert.Len(t, plainKey, APIKeyLen*2) // hex-encoded
	assert.NotEmpty(t, tenant.APIKeyHash)
	assert.NotEqual(t, plainKey, tenant.APIKeyHash) // hash != plain key
}

func TestTenant_GenerateAPIKey_HashConsistency(t *testing.T) {
	tenant := NewTenant("Acme VPN", "acme.vpn.com", strPtr("owner-123"), time.Now())

	plainKey, err := tenant.GenerateAPIKey(time.Now())
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
	account, err := NewResellerAccount("tenant-1", "user-1", 20, time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, account.ID)
	assert.Equal(t, "tenant-1", account.TenantID)
	assert.Equal(t, "user-1", account.UserID)
	assert.Equal(t, 20, account.CommissionRate)
	assert.Equal(t, int64(0), account.Balance)
	assert.False(t, account.CreatedAt.IsZero())
}

func TestNewResellerAccount_ZeroRate(t *testing.T) {
	account, err := NewResellerAccount("tenant-1", "user-1", 0, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 0, account.CommissionRate)
}

func TestNewResellerAccount_MaxRate(t *testing.T) {
	account, err := NewResellerAccount("tenant-1", "user-1", 100, time.Now())
	require.NoError(t, err)
	assert.Equal(t, 100, account.CommissionRate)
}

func TestNewResellerAccount_RateTooLow(t *testing.T) {
	_, err := NewResellerAccount("tenant-1", "user-1", -1, time.Now())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCommissionRate)
}

func TestNewResellerAccount_RateTooHigh(t *testing.T) {
	_, err := NewResellerAccount("tenant-1", "user-1", 101, time.Now())
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidCommissionRate)
}

func TestNewCommission(t *testing.T) {
	commission, err := NewCommission("reseller-1", "sale-abc", 10000, 15, "USD", time.Now())
	require.NoError(t, err)

	assert.NotEmpty(t, commission.ID)
	assert.Equal(t, "reseller-1", commission.ResellerID)
	assert.Equal(t, "sale-abc", commission.SaleID)
	assert.Equal(t, int64(1500), commission.Amount.Amount) // 15% of 10000
	assert.True(t, commission.Amount.IsPositive())
	assert.Equal(t, CommissionPending, commission.Status)
	assert.Nil(t, commission.PaidAt)
	assert.False(t, commission.CreatedAt.IsZero())
}

func TestNewCommission_ZeroRate(t *testing.T) {
	commission, err := NewCommission("reseller-1", "sale-abc", 10000, 0, "USD", time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(0), commission.Amount.Amount)
	assert.True(t, commission.Amount.IsZero())
}

func TestNewCommission_FullRate(t *testing.T) {
	commission, err := NewCommission("reseller-1", "sale-abc", 10000, 100, "USD", time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(10000), commission.Amount.Amount)
}

func TestNewCommission_RoundsDown(t *testing.T) {
	// 33% of 100 cents = 33 cents (integer division rounds down).
	commission, err := NewCommission("reseller-1", "sale-abc", 100, 33, "USD", time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(33), commission.Amount.Amount)
}

func TestNewCommission_Overflow(t *testing.T) {
	tests := []struct {
		name       string
		saleAmount int64
		rate       int
	}{
		{
			name:       "MaxInt64 with rate 50",
			saleAmount: math.MaxInt64,
			rate:       50,
		},
		{
			name:       "large amount with rate 100",
			saleAmount: math.MaxInt64,
			rate:       100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCommission("reseller-1", "sale-abc", tt.saleAmount, tt.rate, "USD", time.Now())
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrCommissionOverflow)
		})
	}
}

func TestNewCommission_NoOverflowForSafeValues(t *testing.T) {
	// A large but safe sale amount that fits in int64 after multiplication.
	commission, err := NewCommission("reseller-1", "sale-abc", 1_000_000_000_00, 50, "USD", time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(500_000_000_00), commission.Amount.Amount)
}

func TestNewCommission_RecordsCreationEvent(t *testing.T) {
	commission, err := NewCommission("reseller-1", "sale-abc", 10000, 15, "USD", time.Now())
	require.NoError(t, err)

	events := commission.DomainEvents()
	require.Len(t, events, 1)
	assert.Equal(t, EventCommissionCreated, events[0].Type)

	payload, ok := events[0].Data.(CommissionCreatedPayload)
	require.True(t, ok)
	assert.Equal(t, commission.ID, payload.CommissionID)
	assert.Equal(t, "reseller-1", payload.ResellerID)
	assert.Equal(t, int64(1500), payload.Amount) // event payload keeps raw int64 for serialization
}

func TestCommission_MarkPaid(t *testing.T) {
	tests := []struct {
		name        string
		status      CommissionStatus
		wantErr     error
		wantStatus  CommissionStatus
		wantPaidAt  bool
		wantEventN  int
	}{
		{
			name:       "pending to paid succeeds",
			status:     CommissionPending,
			wantErr:    nil,
			wantStatus: CommissionPaid,
			wantPaidAt: true,
			wantEventN: 1,
		},
		{
			name:       "already paid returns error",
			status:     CommissionPaid,
			wantErr:    ErrCommissionAlreadyPaid,
			wantStatus: CommissionPaid,
			wantPaidAt: false,
			wantEventN: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commission := &Commission{
				ID:         "comm-1",
				ResellerID: "reseller-1",
				SaleID:     "sale-1",
				Amount:     money.NewMoney(1500, money.Currency("USD")),
				Status:     tt.status,
				CreatedAt:  time.Now(),
			}
			now := time.Now()

			err := commission.MarkPaid(now)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, commission.PaidAt)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStatus, commission.Status)
			}

			if tt.wantPaidAt {
				require.NotNil(t, commission.PaidAt)
				assert.Equal(t, now, *commission.PaidAt)
			}

			events := commission.DomainEvents()
			assert.Len(t, events, tt.wantEventN)

			if tt.wantEventN > 0 {
				assert.Equal(t, EventCommissionPaid, events[0].Type)
				payload, ok := events[0].Data.(CommissionPaidPayload)
				require.True(t, ok)
				assert.Equal(t, "comm-1", payload.CommissionID)
				assert.Equal(t, "reseller-1", payload.ResellerID)
				assert.Equal(t, int64(1500), payload.Amount)
			}
		})
	}
}

func TestCommission_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   CommissionStatus
		to     CommissionStatus
		expect bool
	}{
		{"pending to paid", CommissionPending, CommissionPaid, true},
		{"paid to paid", CommissionPaid, CommissionPaid, false},
		{"paid to pending", CommissionPaid, CommissionPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Commission{Status: tt.from}
			assert.Equal(t, tt.expect, c.CanTransitionTo(tt.to))
		})
	}
}

func TestTenant_Deactivate(t *testing.T) {
	tests := []struct {
		name     string
		isActive bool
		wantErr  error
	}{
		{
			name:     "active tenant deactivates successfully",
			isActive: true,
			wantErr:  nil,
		},
		{
			name:     "inactive tenant returns error",
			isActive: false,
			wantErr:  ErrTenantAlreadyInactive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := NewTenant("Acme", "acme.com", strPtr("owner-1"), time.Now())
			tenant.IsActive = tt.isActive
			now := time.Now()

			err := tenant.Deactivate(now)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.False(t, tenant.IsActive)
				assert.Equal(t, now, tenant.UpdatedAt)
			}
		})
	}
}

func TestTenant_Activate(t *testing.T) {
	tests := []struct {
		name     string
		isActive bool
		wantErr  error
	}{
		{
			name:     "inactive tenant activates successfully",
			isActive: false,
			wantErr:  nil,
		},
		{
			name:     "active tenant returns error",
			isActive: true,
			wantErr:  ErrTenantAlreadyActive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := NewTenant("Acme", "acme.com", strPtr("owner-1"), time.Now())
			tenant.IsActive = tt.isActive
			now := time.Now()

			err := tenant.Activate(now)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.True(t, tenant.IsActive)
				assert.Equal(t, now, tenant.UpdatedAt)
			}
		})
	}
}
