package vo

import "github.com/BEDOLAGA-DEV/RemnaCore/pkg/money"

// Money and Currency are re-exported from pkg/money so that billing code
// continues to compile without changes. New cross-context code should import
// pkg/money directly.
type (
	Money        = money.Money
	Currency     = money.Currency
	RoundingMode = money.RoundingMode
)

// Re-export currency constants.
const (
	CurrencyUSD = money.CurrencyUSD
	CurrencyEUR = money.CurrencyEUR
	CurrencyRUB = money.CurrencyRUB
	CurrencyGBP = money.CurrencyGBP
)

// Re-export formatting and calculation constants.
const CentsPerUnit = money.CentsPerUnit

// Re-export rounding mode constants.
const (
	RoundFloor  = money.RoundFloor
	RoundHalfUp = money.RoundHalfUp
)

// Re-export sentinel errors.
var (
	ErrCurrencyMismatch = money.ErrCurrencyMismatch
	ErrMoneyOverflow    = money.ErrMoneyOverflow
)

// Re-export constructors.
var (
	NewMoney = money.NewMoney
	Zero     = money.Zero
)
