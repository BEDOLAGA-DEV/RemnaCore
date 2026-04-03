package vo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMoney(t *testing.T) {
	m := NewMoney(1299, CurrencyUSD)

	assert.Equal(t, int64(1299), m.Amount)
	assert.Equal(t, CurrencyUSD, m.Currency)
}

func TestZero(t *testing.T) {
	m := Zero(CurrencyEUR)

	assert.Equal(t, int64(0), m.Amount)
	assert.Equal(t, CurrencyEUR, m.Currency)
	assert.True(t, m.IsZero())
}

func TestMoney_Add_SameCurrency(t *testing.T) {
	a := NewMoney(1000, CurrencyUSD)
	b := NewMoney(500, CurrencyUSD)

	result, err := a.Add(b)

	require.NoError(t, err)
	assert.Equal(t, int64(1500), result.Amount)
	assert.Equal(t, CurrencyUSD, result.Currency)
}

func TestMoney_Add_DifferentCurrency(t *testing.T) {
	a := NewMoney(1000, CurrencyUSD)
	b := NewMoney(500, CurrencyEUR)

	_, err := a.Add(b)

	require.Error(t, err)
	assert.ErrorContains(t, err, "currency mismatch")
}

func TestMoney_Subtract_SameCurrency(t *testing.T) {
	a := NewMoney(1000, CurrencyUSD)
	b := NewMoney(300, CurrencyUSD)

	result, err := a.Subtract(b)

	require.NoError(t, err)
	assert.Equal(t, int64(700), result.Amount)
	assert.Equal(t, CurrencyUSD, result.Currency)
}

func TestMoney_Subtract_DifferentCurrency(t *testing.T) {
	a := NewMoney(1000, CurrencyUSD)
	b := NewMoney(300, CurrencyEUR)

	_, err := a.Subtract(b)

	require.Error(t, err)
	assert.ErrorContains(t, err, "currency mismatch")
}

func TestMoney_Subtract_GoesNegative(t *testing.T) {
	a := NewMoney(100, CurrencyUSD)
	b := NewMoney(300, CurrencyUSD)

	result, err := a.Subtract(b)

	require.NoError(t, err)
	assert.Equal(t, int64(-200), result.Amount)
	assert.True(t, result.IsNegative())
}

func TestMoney_Multiply(t *testing.T) {
	m := NewMoney(500, CurrencyRUB)

	result := m.Multiply(3)

	assert.Equal(t, int64(1500), result.Amount)
	assert.Equal(t, CurrencyRUB, result.Currency)
}

func TestMoney_IsZero(t *testing.T) {
	assert.True(t, Zero(CurrencyUSD).IsZero())
	assert.False(t, NewMoney(1, CurrencyUSD).IsZero())
}

func TestMoney_IsPositive(t *testing.T) {
	assert.True(t, NewMoney(100, CurrencyUSD).IsPositive())
	assert.False(t, Zero(CurrencyUSD).IsPositive())
	assert.False(t, NewMoney(-1, CurrencyUSD).IsPositive())
}

func TestMoney_IsNegative(t *testing.T) {
	assert.True(t, NewMoney(-100, CurrencyUSD).IsNegative())
	assert.False(t, Zero(CurrencyUSD).IsNegative())
	assert.False(t, NewMoney(1, CurrencyUSD).IsNegative())
}

func TestMoney_String(t *testing.T) {
	tests := []struct {
		name     string
		money    Money
		expected string
	}{
		{
			name:     "USD positive",
			money:    NewMoney(1299, CurrencyUSD),
			expected: "12.99 usd",
		},
		{
			name:     "EUR zero",
			money:    Zero(CurrencyEUR),
			expected: "0.00 eur",
		},
		{
			name:     "RUB negative",
			money:    NewMoney(-5050, CurrencyRUB),
			expected: "-50.50 rub",
		},
		{
			name:     "GBP single digit cents",
			money:    NewMoney(105, CurrencyGBP),
			expected: "1.05 gbp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.money.String())
		})
	}
}

func TestMoney_Immutability(t *testing.T) {
	original := NewMoney(1000, CurrencyUSD)
	other := NewMoney(500, CurrencyUSD)

	result, err := original.Add(other)
	require.NoError(t, err)

	assert.Equal(t, int64(1000), original.Amount, "original must not change")
	assert.Equal(t, int64(1500), result.Amount)
}
