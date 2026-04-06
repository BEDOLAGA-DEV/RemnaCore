package vo

import (
	"math"
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

func TestMoney_Add(t *testing.T) {
	tests := []struct {
		name      string
		a         Money
		b         Money
		want      Money
		wantErr   error
		wantNoErr bool
	}{
		{
			name:      "same currency",
			a:         NewMoney(1000, CurrencyUSD),
			b:         NewMoney(500, CurrencyUSD),
			want:      NewMoney(1500, CurrencyUSD),
			wantNoErr: true,
		},
		{
			name:    "different currency",
			a:       NewMoney(1000, CurrencyUSD),
			b:       NewMoney(500, CurrencyEUR),
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:      "add negative",
			a:         NewMoney(1000, CurrencyUSD),
			b:         NewMoney(-300, CurrencyUSD),
			want:      NewMoney(700, CurrencyUSD),
			wantNoErr: true,
		},
		{
			name:      "add zero",
			a:         NewMoney(1000, CurrencyUSD),
			b:         Zero(CurrencyUSD),
			want:      NewMoney(1000, CurrencyUSD),
			wantNoErr: true,
		},
		{
			name:    "overflow positive",
			a:       NewMoney(math.MaxInt64, CurrencyUSD),
			b:       NewMoney(1, CurrencyUSD),
			wantErr: ErrMoneyOverflow,
		},
		{
			name:    "overflow negative",
			a:       NewMoney(math.MinInt64, CurrencyUSD),
			b:       NewMoney(-1, CurrencyUSD),
			wantErr: ErrMoneyOverflow,
		},
		{
			name:      "large values no overflow",
			a:         NewMoney(math.MaxInt64-1, CurrencyUSD),
			b:         NewMoney(1, CurrencyUSD),
			want:      NewMoney(math.MaxInt64, CurrencyUSD),
			wantNoErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.Add(tt.b)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_Subtract(t *testing.T) {
	tests := []struct {
		name      string
		a         Money
		b         Money
		want      Money
		wantErr   error
		wantNoErr bool
	}{
		{
			name:      "same currency",
			a:         NewMoney(1000, CurrencyUSD),
			b:         NewMoney(300, CurrencyUSD),
			want:      NewMoney(700, CurrencyUSD),
			wantNoErr: true,
		},
		{
			name:    "different currency",
			a:       NewMoney(1000, CurrencyUSD),
			b:       NewMoney(300, CurrencyEUR),
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:      "goes negative",
			a:         NewMoney(100, CurrencyUSD),
			b:         NewMoney(300, CurrencyUSD),
			want:      NewMoney(-200, CurrencyUSD),
			wantNoErr: true,
		},
		{
			name:    "overflow subtracting negative from max",
			a:       NewMoney(math.MaxInt64, CurrencyUSD),
			b:       NewMoney(-1, CurrencyUSD),
			wantErr: ErrMoneyOverflow,
		},
		{
			name:    "underflow subtracting positive from min",
			a:       NewMoney(math.MinInt64, CurrencyUSD),
			b:       NewMoney(1, CurrencyUSD),
			wantErr: ErrMoneyOverflow,
		},
		{
			name:      "subtract zero",
			a:         NewMoney(500, CurrencyRUB),
			b:         Zero(CurrencyRUB),
			want:      NewMoney(500, CurrencyRUB),
			wantNoErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.Subtract(tt.b)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_Multiply(t *testing.T) {
	tests := []struct {
		name    string
		money   Money
		factor  int64
		want    Money
		wantErr error
	}{
		{
			name:   "positive factor",
			money:  NewMoney(500, CurrencyRUB),
			factor: 3,
			want:   NewMoney(1500, CurrencyRUB),
		},
		{
			name:   "multiply by zero",
			money:  NewMoney(500, CurrencyUSD),
			factor: 0,
			want:   Zero(CurrencyUSD),
		},
		{
			name:   "multiply by one",
			money:  NewMoney(999, CurrencyGBP),
			factor: 1,
			want:   NewMoney(999, CurrencyGBP),
		},
		{
			name:   "multiply by negative",
			money:  NewMoney(100, CurrencyEUR),
			factor: -2,
			want:   NewMoney(-200, CurrencyEUR),
		},
		{
			name:    "overflow positive",
			money:   NewMoney(math.MaxInt64, CurrencyUSD),
			factor:  2,
			wantErr: ErrMoneyOverflow,
		},
		{
			name:    "overflow negative factor",
			money:   NewMoney(math.MinInt64, CurrencyUSD),
			factor:  2,
			wantErr: ErrMoneyOverflow,
		},
		{
			name:   "zero amount with large factor",
			money:  Zero(CurrencyUSD),
			factor: math.MaxInt64,
			want:   Zero(CurrencyUSD),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.money.Multiply(tt.factor)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_Equal(t *testing.T) {
	tests := []struct {
		name string
		a    Money
		b    Money
		want bool
	}{
		{
			name: "same amount and currency",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "different amount",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(2000, CurrencyUSD),
			want: false,
		},
		{
			name: "different currency",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyEUR),
			want: false,
		},
		{
			name: "both zero different currency",
			a:    Zero(CurrencyUSD),
			b:    Zero(CurrencyEUR),
			want: false,
		},
		{
			name: "both zero same currency",
			a:    Zero(CurrencyRUB),
			b:    Zero(CurrencyRUB),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.a.Equal(tt.b))
		})
	}
}

func TestMoney_GreaterThan(t *testing.T) {
	tests := []struct {
		name    string
		a       Money
		b       Money
		want    bool
		wantErr error
	}{
		{
			name: "greater",
			a:    NewMoney(2000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "equal",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: false,
		},
		{
			name: "less",
			a:    NewMoney(500, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: false,
		},
		{
			name:    "currency mismatch",
			a:       NewMoney(2000, CurrencyUSD),
			b:       NewMoney(1000, CurrencyEUR),
			wantErr: ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.GreaterThan(tt.b)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_LessThan(t *testing.T) {
	tests := []struct {
		name    string
		a       Money
		b       Money
		want    bool
		wantErr error
	}{
		{
			name: "less",
			a:    NewMoney(500, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "equal",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: false,
		},
		{
			name: "greater",
			a:    NewMoney(2000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: false,
		},
		{
			name:    "currency mismatch",
			a:       NewMoney(500, CurrencyUSD),
			b:       NewMoney(1000, CurrencyGBP),
			wantErr: ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.LessThan(tt.b)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_GreaterThanOrEqual(t *testing.T) {
	tests := []struct {
		name    string
		a       Money
		b       Money
		want    bool
		wantErr error
	}{
		{
			name: "greater",
			a:    NewMoney(2000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "equal",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "less",
			a:    NewMoney(500, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: false,
		},
		{
			name:    "currency mismatch",
			a:       NewMoney(2000, CurrencyUSD),
			b:       NewMoney(1000, CurrencyRUB),
			wantErr: ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.GreaterThanOrEqual(tt.b)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_LessThanOrEqual(t *testing.T) {
	tests := []struct {
		name    string
		a       Money
		b       Money
		want    bool
		wantErr error
	}{
		{
			name: "less",
			a:    NewMoney(500, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "equal",
			a:    NewMoney(1000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: true,
		},
		{
			name: "greater",
			a:    NewMoney(2000, CurrencyUSD),
			b:    NewMoney(1000, CurrencyUSD),
			want: false,
		},
		{
			name:    "currency mismatch",
			a:       NewMoney(500, CurrencyUSD),
			b:       NewMoney(1000, CurrencyEUR),
			wantErr: ErrCurrencyMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.a.LessThanOrEqual(tt.b)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMoney_IsZero(t *testing.T) {
	assert.True(t, Zero(CurrencyUSD).IsZero())
	assert.False(t, NewMoney(1, CurrencyUSD).IsZero())
	assert.False(t, NewMoney(-1, CurrencyUSD).IsZero())
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
