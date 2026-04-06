package vo

import (
	"testing"
	"time"
)

func FuzzDiscountApply(f *testing.F) {
	f.Add(int64(5000), int64(1000), "usd", false) // 50% on $10
	f.Add(int64(10000), int64(0), "usd", false)   // 100% on $0
	f.Add(int64(1), int64(1), "usd", true)         // expired

	f.Fuzz(func(t *testing.T, percent, priceAmount int64, cur string, expired bool) {
		if percent < 1 || percent > 10000 {
			return // invalid percent, skip
		}

		d, err := NewPercentDiscount(percent, "FUZZ", nil)
		if err != nil {
			return
		}

		if expired {
			past := time.Now().Add(-24 * time.Hour)
			d.ExpiresAt = &past
		}

		price := NewMoney(priceAmount, Currency(cur))
		result, err := d.Apply(price, time.Now())
		if err != nil {
			return // valid error
		}

		// Result should not exceed original price
		if result.Amount > price.Amount && price.Amount >= 0 {
			t.Errorf("discount increased price: %d → %d", price.Amount, result.Amount)
		}
	})
}
