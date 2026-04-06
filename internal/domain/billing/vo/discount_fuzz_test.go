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

func FuzzFixedDiscountApply(f *testing.F) {
	f.Add(int64(1000), int64(200), "usd")
	f.Add(int64(100), int64(100), "usd")  // exact match
	f.Add(int64(50), int64(1000), "usd")  // discount exceeds price
	f.Add(int64(0), int64(1), "usd")      // zero price

	f.Fuzz(func(t *testing.T, priceAmount, discountAmount int64, cur string) {
		if cur == "" || priceAmount < 0 || discountAmount < 0 {
			return
		}

		price := Money{Amount: priceAmount, Currency: Currency(cur)}
		fd, err := NewFixedDiscount(discountAmount, Currency(cur), "FUZZ", nil)
		if err != nil {
			return // discountAmount <= 0 — valid rejection
		}

		result, err := fd.Apply(price, time.Now())
		if err != nil {
			return
		}

		// Property: result == max(price - discount, 0)
		expected := max(priceAmount-discountAmount, 0)
		if result.Amount != expected {
			t.Errorf("FixedDiscount.Apply(%d, %d) = %d, want %d",
				priceAmount, discountAmount, result.Amount, expected)
		}
		if result.Currency != price.Currency {
			t.Errorf("FixedDiscount.Apply changed currency from %s to %s",
				price.Currency, result.Currency)
		}
	})
}
