package vo

import (
	"testing"
)

func FuzzMoneyAdd(f *testing.F) {
	f.Add(int64(100), int64(200), "usd", "usd")
	f.Add(int64(0), int64(0), "usd", "usd")
	f.Add(int64(9223372036854775807), int64(1), "usd", "usd")
	f.Add(int64(-9223372036854775808), int64(-1), "usd", "usd")
	f.Add(int64(100), int64(200), "usd", "eur")

	f.Fuzz(func(t *testing.T, a, b int64, curA, curB string) {
		mA := Money{Amount: a, Currency: Currency(curA)}
		mB := Money{Amount: b, Currency: Currency(curB)}

		result, err := mA.Add(mB)
		if err != nil {
			return // overflow or currency mismatch — valid
		}

		// Verify: result - b == a (if no overflow)
		back, err := result.Subtract(mB)
		if err != nil {
			return // overflow in subtraction
		}
		if back.Amount != mA.Amount {
			t.Errorf("Add then Subtract not identity: %d + %d = %d, %d - %d = %d",
				a, b, result.Amount, result.Amount, b, back.Amount)
		}
	})
}

func FuzzMoneyMultiply(f *testing.F) {
	f.Add(int64(100), int64(2))
	f.Add(int64(0), int64(999))
	f.Add(int64(9223372036854775807), int64(2))
	f.Add(int64(-1), int64(-1))

	f.Fuzz(func(t *testing.T, amount, factor int64) {
		m := Money{Amount: amount, Currency: "usd"}
		result, err := m.Multiply(factor)
		if err != nil {
			return // overflow — valid
		}

		expected := amount * factor
		if result.Amount != expected {
			t.Errorf("Multiply(%d, %d) = %d, want %d", amount, factor, result.Amount, expected)
		}
		if result.Currency != m.Currency {
			t.Errorf("Multiply changed currency from %s to %s", m.Currency, result.Currency)
		}
	})
}

func FuzzMoneySubtract(f *testing.F) {
	f.Add(int64(100), int64(50), "usd")
	f.Add(int64(0), int64(0), "usd")
	f.Add(int64(9223372036854775807), int64(1), "usd")
	f.Add(int64(-9223372036854775808), int64(-1), "usd")

	f.Fuzz(func(t *testing.T, a, b int64, cur string) {
		if cur == "" {
			return
		}
		ma := Money{Amount: a, Currency: Currency(cur)}
		mb := Money{Amount: b, Currency: Currency(cur)}

		diff, err := ma.Subtract(mb)
		if err != nil {
			return // overflow — valid
		}

		roundTrip, err := diff.Add(mb)
		if err != nil {
			return // overflow in addition — valid
		}

		if roundTrip.Amount != a {
			t.Errorf("Subtract then Add: got %d, want %d", roundTrip.Amount, a)
		}
	})
}

func FuzzMoneyString(f *testing.F) {
	f.Add(int64(0), "usd")
	f.Add(int64(150), "usd")
	f.Add(int64(-50), "usd")
	f.Add(int64(-1), "eur")
	f.Add(int64(9223372036854775807), "usd")

	f.Fuzz(func(t *testing.T, amount int64, cur string) {
		m := Money{Amount: amount, Currency: Currency(cur)}
		s := m.String()
		if s == "" {
			t.Error("String() returned empty")
		}
		// No panic = success
	})
}
