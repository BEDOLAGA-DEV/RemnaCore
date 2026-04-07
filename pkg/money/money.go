// Package money provides the Money value object for representing monetary
// amounts. It is a shared kernel type used across bounded contexts.
package money

import (
	"errors"
	"fmt"
	"math"
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

// ValidCurrencies enumerates the currencies supported by the platform.
// Used by Validate to reject unknown currency codes.
var ValidCurrencies = map[Currency]bool{
	CurrencyUSD: true,
	CurrencyEUR: true,
	CurrencyRUB: true,
	CurrencyGBP: true,
}

var (
	// ErrCurrencyMismatch is returned when an arithmetic or comparison
	// operation is attempted on Money values with different currencies.
	ErrCurrencyMismatch = errors.New("currency mismatch")

	// ErrMoneyOverflow is returned when an arithmetic operation would
	// overflow or underflow the int64 amount.
	ErrMoneyOverflow = errors.New("monetary amount overflow")

	// ErrInvalidCurrency is returned when a currency code is not in the
	// ValidCurrencies set.
	ErrInvalidCurrency = errors.New("unsupported currency")
)

// Money represents a monetary amount in the smallest unit (cents / kopecks).
// Money is immutable: arithmetic methods return new values.
type Money struct {
	Amount   int64    // cents (or kopecks for RUB)
	Currency Currency
}

// ValidateCurrency checks whether the given currency code is in the
// platform's supported set. Returns ErrInvalidCurrency if not.
func ValidateCurrency(c Currency) error {
	if !ValidCurrencies[c] {
		return fmt.Errorf("%w: %q", ErrInvalidCurrency, c)
	}
	return nil
}

// NewMoney creates a Money value with the given amount and currency.
func NewMoney(amount int64, currency Currency) Money {
	return Money{Amount: amount, Currency: currency}
}

// Validate checks that the Money value uses a supported currency.
// Callers should invoke this at domain boundaries (e.g., plan creation,
// invoice construction) rather than relying on constructor enforcement,
// to preserve backward compatibility with existing struct-literal usage.
func (m Money) Validate() error {
	return ValidateCurrency(m.Currency)
}

// Zero returns a Money value of zero in the given currency.
func Zero(currency Currency) Money {
	return Money{Amount: 0, Currency: currency}
}

// Add returns the sum of m and other. Both must share the same currency.
// Returns ErrMoneyOverflow if the result would overflow int64.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	sum := m.Amount + other.Amount
	if (other.Amount > 0 && sum < m.Amount) || (other.Amount < 0 && sum > m.Amount) {
		return Money{}, ErrMoneyOverflow
	}
	return Money{Amount: sum, Currency: m.Currency}, nil
}

// Subtract returns m minus other. Both must share the same currency.
// Returns ErrMoneyOverflow if the result would overflow int64.
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	diff := m.Amount - other.Amount
	if (other.Amount < 0 && diff < m.Amount) || (other.Amount > 0 && diff > m.Amount) {
		return Money{}, ErrMoneyOverflow
	}
	return Money{Amount: diff, Currency: m.Currency}, nil
}

// Multiply returns m scaled by factor.
// Returns ErrMoneyOverflow if the result would overflow int64.
func (m Money) Multiply(factor int64) (Money, error) {
	if m.Amount == 0 || factor == 0 {
		return Money{Amount: 0, Currency: m.Currency}, nil
	}
	// Special-case MinInt64 * -1 which would overflow to itself.
	if factor == -1 {
		if m.Amount == math.MinInt64 {
			return Money{}, ErrMoneyOverflow
		}
		return Money{Amount: -m.Amount, Currency: m.Currency}, nil
	}
	if m.Amount == -1 {
		if factor == math.MinInt64 {
			return Money{}, ErrMoneyOverflow
		}
		return Money{Amount: -factor, Currency: m.Currency}, nil
	}
	if factor > 0 {
		if m.Amount > math.MaxInt64/factor || m.Amount < math.MinInt64/factor {
			return Money{}, ErrMoneyOverflow
		}
	} else {
		// factor < -1 here (factor == -1 handled above).
		if m.Amount < math.MaxInt64/factor || m.Amount > math.MinInt64/factor {
			return Money{}, ErrMoneyOverflow
		}
	}
	return Money{Amount: m.Amount * factor, Currency: m.Currency}, nil
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

// Equal reports whether m and other represent the same amount in the same currency.
func (m Money) Equal(other Money) bool {
	return m.Amount == other.Amount && m.Currency == other.Currency
}

// GreaterThan reports whether m is strictly greater than other.
// Returns ErrCurrencyMismatch if currencies differ.
func (m Money) GreaterThan(other Money) (bool, error) {
	if m.Currency != other.Currency {
		return false, ErrCurrencyMismatch
	}
	return m.Amount > other.Amount, nil
}

// LessThan reports whether m is strictly less than other.
// Returns ErrCurrencyMismatch if currencies differ.
func (m Money) LessThan(other Money) (bool, error) {
	if m.Currency != other.Currency {
		return false, ErrCurrencyMismatch
	}
	return m.Amount < other.Amount, nil
}

// GreaterThanOrEqual reports whether m is greater than or equal to other.
// Returns ErrCurrencyMismatch if currencies differ.
func (m Money) GreaterThanOrEqual(other Money) (bool, error) {
	if m.Currency != other.Currency {
		return false, ErrCurrencyMismatch
	}
	return m.Amount >= other.Amount, nil
}

// LessThanOrEqual reports whether m is less than or equal to other.
// Returns ErrCurrencyMismatch if currencies differ.
func (m Money) LessThanOrEqual(other Money) (bool, error) {
	if m.Currency != other.Currency {
		return false, ErrCurrencyMismatch
	}
	return m.Amount <= other.Amount, nil
}

// RoundingMode defines how fractional cents are handled in percent-of-money
// calculations.
type RoundingMode int

const (
	// RoundFloor truncates toward zero (Go integer division).
	// This is the platform's default for percent discounts: rounding in favor
	// of a smaller discount ensures the merchant never undercharges.
	RoundFloor RoundingMode = iota

	// RoundHalfUp rounds to nearest, with ties going away from zero.
	// Available for future use (e.g., tax calculations).
	RoundHalfUp
)

// PercentBase is the denominator for percent calculations.
// A value of 10000 means percentages are expressed in basis points:
// 10000 = 100.00%, 5000 = 50%, 1250 = 12.5%, 100 = 1%, 1 = 0.01%.
const PercentBase = 10000

// PercentOf calculates pct/PercentBase of the money amount using the
// specified rounding mode. pct is in basis-point units where PercentBase
// (10000) represents 100%.
//
// Policy: the platform uses RoundFloor for percent discounts so that
// fractional cents are truncated, producing a smaller discount amount.
// This guarantees the merchant is never undercharged.
//
// Examples with RoundFloor:
//
//	Money{Amount: 1000}.PercentOf(3333, RoundFloor) -> Money{Amount: 333}
//	  (33.33% of $10.00 = $3.33, not $3.334)
//	Money{Amount: 999}.PercentOf(5000, RoundFloor)  -> Money{Amount: 499}
//	  (50% of $9.99 = $4.99, not $4.995)
func (m Money) PercentOf(pct int64, rounding RoundingMode) Money {
	switch rounding {
	case RoundHalfUp:
		return Money{
			Amount:   (m.Amount*pct + PercentBase/2) / PercentBase,
			Currency: m.Currency,
		}
	default: // RoundFloor — platform default
		return Money{
			Amount:   m.Amount * pct / PercentBase,
			Currency: m.Currency,
		}
	}
}

// String returns a human-readable representation such as "12.99 USD".
func (m Money) String() string {
	major := m.Amount / CentsPerUnit
	minor := m.Amount % CentsPerUnit
	if minor < 0 {
		minor = -minor
	}

	// When Amount is negative but major truncates to 0 (e.g. -50 cents),
	// the sign is lost. Restore it explicitly.
	sign := ""
	if m.Amount < 0 && major == 0 {
		sign = "-"
	}

	return fmt.Sprintf("%s%d.%02d %s", sign, major, minor, m.Currency)
}
