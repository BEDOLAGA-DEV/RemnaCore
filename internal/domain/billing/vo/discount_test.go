package vo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPercentDiscount_Valid(t *testing.T) {
	d, err := NewPercentDiscount(2000, "SAVE20", nil) // 20%

	require.NoError(t, err)
	assert.Equal(t, DiscountPercent, d.Type)
	assert.Equal(t, int64(2000), d.Value)
	assert.Equal(t, "SAVE20", d.Code)
	assert.Nil(t, d.ExpiresAt)
}

func TestNewPercentDiscount_Zero(t *testing.T) {
	_, err := NewPercentDiscount(0, "BAD", nil)

	require.Error(t, err)
	assert.ErrorContains(t, err, "percent")
}

func TestNewPercentDiscount_Over100(t *testing.T) {
	_, err := NewPercentDiscount(10001, "BAD", nil) // > 100%

	require.Error(t, err)
	assert.ErrorContains(t, err, "percent")
}

func TestNewPercentDiscount_Exactly100(t *testing.T) {
	d, err := NewPercentDiscount(10000, "FREE", nil) // 100%

	require.NoError(t, err)
	assert.Equal(t, int64(10000), d.Value)
}

func TestNewPercentDiscount_FractionalPercent(t *testing.T) {
	d, err := NewPercentDiscount(1250, "SAVE12.5", nil) // 12.5%

	require.NoError(t, err)
	assert.Equal(t, int64(1250), d.Value)

	price := NewMoney(10000, CurrencyUSD) // $100.00
	result, err := d.Apply(price, time.Now())
	require.NoError(t, err)
	assert.Equal(t, int64(8750), result.Amount) // $87.50
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
	d, err := NewPercentDiscount(2500, "SAVE25", nil) // 25%
	require.NoError(t, err)

	price := NewMoney(10000, CurrencyUSD) // $100.00

	result, err := d.Apply(price, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(7500), result.Amount) // $75.00
	assert.Equal(t, CurrencyUSD, result.Currency)
}

func TestDiscount_ApplyPercent100(t *testing.T) {
	d, err := NewPercentDiscount(10000, "FREE", nil) // 100%
	require.NoError(t, err)

	price := NewMoney(5000, CurrencyUSD)

	result, err := d.Apply(price, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Amount)
}

func TestDiscount_ApplyFixed(t *testing.T) {
	d, err := NewFixedDiscount(300, CurrencyUSD, "FLAT3", nil)
	require.NoError(t, err)

	price := NewMoney(1000, CurrencyUSD) // $10.00

	result, err := d.Apply(price, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(700), result.Amount) // $7.00
}

func TestDiscount_ApplyFixed_FloorAtZero(t *testing.T) {
	d, err := NewFixedDiscount(2000, CurrencyUSD, "BIG", nil)
	require.NoError(t, err)

	price := NewMoney(500, CurrencyUSD) // $5.00

	result, err := d.Apply(price, time.Now())

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Amount, "fixed discount must not go below zero")
}

func TestDiscount_ApplyExpired(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	d, err := NewPercentDiscount(5000, "EXPIRED", &past) // 50%
	require.NoError(t, err)

	price := NewMoney(1000, CurrencyUSD)

	result, err := d.Apply(price, time.Now())

	require.NoError(t, err)
	assert.Equal(t, price.Amount, result.Amount, "expired discount must return original price")
}

func TestDiscount_IsExpired_NoExpiry(t *testing.T) {
	d, err := NewPercentDiscount(1000, "FOREVER", nil) // 10%
	require.NoError(t, err)

	assert.False(t, d.IsExpiredAt(time.Now()))
}

func TestDiscount_IsExpired_Future(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	d, err := NewPercentDiscount(1000, "VALID", &future) // 10%
	require.NoError(t, err)

	assert.False(t, d.IsExpiredAt(time.Now()))
}

func TestDiscount_IsExpired_Past(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	d, err := NewPercentDiscount(1000, "OLD", &past) // 10%
	require.NoError(t, err)

	assert.True(t, d.IsExpiredAt(time.Now()))
}

func TestDiscount_Immutability(t *testing.T) {
	d, err := NewPercentDiscount(5000, "HALF", nil) // 50%
	require.NoError(t, err)

	price := NewMoney(1000, CurrencyUSD)
	result, err := d.Apply(price, time.Now())
	require.NoError(t, err)

	assert.Equal(t, int64(1000), price.Amount, "original price must not change")
	assert.Equal(t, int64(500), result.Amount)
}
