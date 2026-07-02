package balance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubWalletGetter implements the narrow walletGetter interface.
type stubWalletGetter struct {
	wallets []Wallet
	err     error
}

func (s *stubWalletGetter) GetWallets(_ context.Context, _ string) ([]Wallet, error) {
	return s.wallets, s.err
}

func TestBalanceReaderAdapter_WalletsByUser_MapsAllFields(t *testing.T) {
	wallets := []Wallet{
		{
			UserID:         "user-1",
			Kind:           WalletMoney,
			Currency:       "usd",
			BalanceCents:   5000,
			AvailableCents: 4000,
		},
		{
			UserID:         "user-1",
			Kind:           WalletBonus,
			Currency:       "usd",
			BalanceCents:   1000,
			AvailableCents: 1000,
		},
	}
	adapter := &BalanceReaderAdapter{svc: &stubWalletGetter{wallets: wallets}}

	views, err := adapter.WalletsByUser(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, views, 2)

	// First wallet: money
	assert.Equal(t, "money", views[0].Kind)
	assert.Equal(t, "usd", views[0].Currency)
	assert.Equal(t, int64(5000), views[0].Balance)
	assert.Equal(t, int64(4000), views[0].Available)

	// Second wallet: bonus
	assert.Equal(t, "bonus", views[1].Kind)
	assert.Equal(t, "usd", views[1].Currency)
	assert.Equal(t, int64(1000), views[1].Balance)
	assert.Equal(t, int64(1000), views[1].Available)
}

func TestBalanceReaderAdapter_WalletsByUser_EmptyList(t *testing.T) {
	adapter := &BalanceReaderAdapter{svc: &stubWalletGetter{wallets: nil}}

	views, err := adapter.WalletsByUser(context.Background(), "user-no-wallets")
	require.NoError(t, err)
	assert.Empty(t, views)
}
