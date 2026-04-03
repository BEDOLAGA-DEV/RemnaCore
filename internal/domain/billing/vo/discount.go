package vo

import (
	"errors"
	"time"
)

// DiscountType distinguishes between percentage and fixed-amount discounts.
type DiscountType string

const (
	DiscountPercent DiscountType = "percent"
	DiscountFixed   DiscountType = "fixed"
)

// PercentBase is the divisor used when converting integer percentages (1-100)
// to fractional multipliers.
const PercentBase = 100

const (
	minPercent = 1
	maxPercent = PercentBase
)

// Discount represents a promo-code or coupon that can be applied to a price.
// Discount is immutable.
type Discount struct {
	Type      DiscountType
	Value     int64  // percent (1-100) or fixed amount in cents
	Code      string // promo code
	ExpiresAt *time.Time
}

// NewPercentDiscount creates a percentage discount. percent must be in [1, 100].
func NewPercentDiscount(percent int64, code string, expiresAt *time.Time) (Discount, error) {
	if percent < minPercent || percent > maxPercent {
		return Discount{}, errors.New("percent must be between 1 and 100")
	}
	return Discount{
		Type:      DiscountPercent,
		Value:     percent,
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}

// NewFixedDiscount creates a fixed-amount discount. amount must be > 0.
func NewFixedDiscount(amount int64, _ Currency, code string, expiresAt *time.Time) (Discount, error) {
	if amount <= 0 {
		return Discount{}, errors.New("fixed discount amount must be greater than zero")
	}
	return Discount{
		Type:      DiscountFixed,
		Value:     amount,
		Code:      code,
		ExpiresAt: expiresAt,
	}, nil
}

// Apply calculates the discounted price. Returns the original price unchanged
// if the discount has expired. The result is floored at zero for fixed discounts.
func (d Discount) Apply(price Money, now time.Time) (Money, error) {
	if d.IsExpiredAt(now) {
		return price, nil // expired discount, return original price
	}

	switch d.Type {
	case DiscountPercent:
		discounted := price.Amount * (maxPercent - d.Value) / maxPercent
		return NewMoney(discounted, price.Currency), nil
	case DiscountFixed:
		result := price.Amount - d.Value
		if result < 0 {
			result = 0
		}
		return NewMoney(result, price.Currency), nil
	default:
		return price, nil
	}
}

// IsExpiredAt reports whether the discount has passed its expiration time
// relative to the given time. Discounts with no expiry never expire.
func (d Discount) IsExpiredAt(now time.Time) bool {
	if d.ExpiresAt == nil {
		return false
	}
	return now.After(*d.ExpiresAt)
}

