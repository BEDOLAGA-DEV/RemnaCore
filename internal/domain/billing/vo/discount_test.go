package vo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPercentDiscount_Valid(t *testing.T) {
	d, err := NewPercentDiscount(20, "SAVE20", nil)

	require.NoError(t, err)
	assert.Equal(t, DiscountPercent, d.Type)
	assert.Equal(t, int64(20), d.Value)
	assert.Equal(t, "SAVE20", d.Code)
	assert.Nil(t, d.ExpiresAt)
}

func TestNewPercentDiscount_Zero(t *testing.T) {
	_, err := NewPercentDiscount(0, "BAD", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "percent")
}

func TestNewPercentDiscount_Over100(t *testing.T) {
	_, err := NewPercentDiscount(101, "BAD", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "percent")
}

func TestNewPercentDiscount_Exactly100(t *testing.T) {
	d, err := NewPercentDiscount(100, "FREE", nil)

	require.NoError(t, err)
	assert.Equal(t, int64(100), d.Value)
}

func TestNewFixedDiscount_Valid(t *testing.T) {
	d, err := NewFixedDiscount(500, CurrencyUSD, "FLAT5", nil)

	require.NoError(t, err)
	assert.Equal(t, DiscountFixed, d.Type)
	assert.Equal(t, int64(500), d.Value)
	assert.Equal(t, "FLAT5", d.Code)
}

func TestNewFixedDiscount_ZeroAmount(t *testing.T) {
	_, err := NewFixedDiscount(0, CurrencyUSD, "BAD", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "amount")
}

func TestNewFixedDiscount_NegativeAmount(t *testing.T) {
	_, err := NewFixedDiscount(-100, CurrencyUSD, "BAD", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "amount")
}

func TestDiscount_ApplyPercent(t *testing.T) {
	d, err := NewPercentDiscount(25, "SAVE25", nil)
	require.NoError(t, err)

	price := NewMoney(10000, CurrencyUSD) // $100.00

	result, err := d.Apply(price)

	require.NoError(t, err)
	assert.Equal(t, int64(7500), result.Amount) // $75.00
	assert.Equal(t, CurrencyUSD, result.Currency)
}

func TestDiscount_ApplyPercent100(t *testing.T) {
	d, err := NewPercentDiscount(100, "FREE", nil)
	require.NoError(t, err)

	price := NewMoney(5000, CurrencyUSD)

	result, err := d.Apply(price)

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Amount)
}

func TestDiscount_ApplyFixed(t *testing.T) {
	d, err := NewFixedDiscount(300, CurrencyUSD, "FLAT3", nil)
	require.NoError(t, err)

	price := NewMoney(1000, CurrencyUSD) // $10.00

	result, err := d.Apply(price)

	require.NoError(t, err)
	assert.Equal(t, int64(700), result.Amount) // $7.00
}

func TestDiscount_ApplyFixed_FloorAtZero(t *testing.T) {
	d, err := NewFixedDiscount(2000, CurrencyUSD, "BIG", nil)
	require.NoError(t, err)

	price := NewMoney(500, CurrencyUSD) // $5.00

	result, err := d.Apply(price)

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Amount, "fixed discount must not go below zero")
}

func TestDiscount_ApplyExpired(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	d, err := NewPercentDiscount(50, "EXPIRED", &past)
	require.NoError(t, err)

	price := NewMoney(1000, CurrencyUSD)

	_, err = d.Apply(price)

	require.Error(t, err)
	assert.ErrorContains(t, err, "expired")
}

func TestDiscount_IsExpired_NoExpiry(t *testing.T) {
	d, err := NewPercentDiscount(10, "FOREVER", nil)
	require.NoError(t, err)

	assert.False(t, d.IsExpired())
}

func TestDiscount_IsExpired_Future(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	d, err := NewPercentDiscount(10, "VALID", &future)
	require.NoError(t, err)

	assert.False(t, d.IsExpired())
}

func TestDiscount_IsExpired_Past(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	d, err := NewPercentDiscount(10, "OLD", &past)
	require.NoError(t, err)

	assert.True(t, d.IsExpired())
}

func TestDiscount_Immutability(t *testing.T) {
	d, err := NewPercentDiscount(50, "HALF", nil)
	require.NoError(t, err)

	price := NewMoney(1000, CurrencyUSD)
	result, err := d.Apply(price)
	require.NoError(t, err)

	assert.Equal(t, int64(1000), price.Amount, "original price must not change")
	assert.Equal(t, int64(500), result.Amount)
}
