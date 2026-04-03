package vo

import (
	"errors"
	"fmt"
)

// Currency represents a supported monetary currency.
type Currency string

const (
	CurrencyUSD Currency = "usd"
	CurrencyEUR Currency = "eur"
	CurrencyRUB Currency = "rub"
	CurrencyGBP Currency = "gbp"
)

// CentsPerUnit is the number of minor units (cents / kopecks) per major
// currency unit. Used for display formatting.
const CentsPerUnit = 100

var errCurrencyMismatch = errors.New("currency mismatch")

// Money represents a monetary amount in the smallest unit (cents / kopecks).
// Money is immutable: arithmetic methods return new values.
type Money struct {
	Amount   int64    // cents (or kopecks for RUB)
	Currency Currency
}

// NewMoney creates a Money value with the given amount and currency.
func NewMoney(amount int64, currency Currency) Money {
	return Money{Amount: amount, Currency: currency}
}

// Zero returns a Money value of zero in the given currency.
func Zero(currency Currency) Money {
	return Money{Amount: 0, Currency: currency}
}

// Add returns the sum of m and other. Both must share the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errCurrencyMismatch
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

// Subtract returns m minus other. Both must share the same currency.
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errCurrencyMismatch
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

// Multiply returns m scaled by factor.
func (m Money) Multiply(factor int64) Money {
	return Money{Amount: m.Amount * factor, Currency: m.Currency}
}

// IsZero reports whether the amount is exactly zero.
func (m Money) IsZero() bool {
	return m.Amount == 0
}

// IsPositive reports whether the amount is strictly greater than zero.
func (m Money) IsPositive() bool {
	return m.Amount > 0
}

// IsNegative reports whether the amount is strictly less than zero.
func (m Money) IsNegative() bool {
	return m.Amount < 0
}

// String returns a human-readable representation such as "12.99 USD".
func (m Money) String() string {
	major := m.Amount / CentsPerUnit
	minor := m.Amount % CentsPerUnit

	// Handle negative amounts correctly: -50.50 not -50.-50
	if m.Amount < 0 && minor != 0 {
		// minor is already negative, we need the absolute value
		if minor < 0 {
			minor = -minor
		}
	}

	return fmt.Sprintf("%d.%02d %s", major, minor, m.Currency)
}
