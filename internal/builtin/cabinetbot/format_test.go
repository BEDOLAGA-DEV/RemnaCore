package cabinetbot

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

func TestFormatMoney(t *testing.T) {
	assert.Equal(t, "0.00 USD", formatMoney(0, "USD"))
	assert.Equal(t, "9.99 USD", formatMoney(999, "USD"))
	assert.Equal(t, "10.00 RUB", formatMoney(1000, "RUB"))
	assert.Equal(t, "-1.05 USD", formatMoney(-105, "USD"))
	// Sub-unit negative must keep its sign (regression: naive division dropped it).
	assert.Equal(t, "-0.50 USD", formatMoney(-50, "USD"))
}

func TestFormatOffers_Empty(t *testing.T) {
	text, kb := formatOffers(nil)
	assert.Equal(t, msgNoOffers, text)
	assert.Empty(t, kb.Rows)
}

func TestFormatOffers_ButtonsCarryPlanCallback(t *testing.T) {
	offers := []bothost.TariffOffer{
		{PlanID: "p1", Name: "Basic", Periods: []bothost.TariffPrice{{Days: 30, Amount: 999, Currency: "USD", Label: "1 месяц", PlanID: "p1"}}},
		{PlanID: "p2", Name: "Pro"},
	}

	text, kb := formatOffers(offers)

	assert.Equal(t, msgOffersHeader, text)
	require.Len(t, kb.Rows, 2)
	require.Len(t, kb.Rows[0], 1)
	assert.Equal(t, "Basic — от 9.99 USD", kb.Rows[0][0].Text)
	assert.Equal(t, "plan:p1", kb.Rows[0][0].CallbackData)
	assert.Equal(t, "Pro", kb.Rows[1][0].Text)
	assert.Equal(t, "plan:p2", kb.Rows[1][0].CallbackData)
}

func TestFormatOfferDetail_BuyButtonsPerPeriod(t *testing.T) {
	offer := bothost.TariffOffer{
		PlanID:      "doc-1",
		Name:        "Basic",
		Description: "Простой тариф",
		Periods: []bothost.TariffPrice{
			{Days: 30, Amount: 999, Currency: "USD", Label: "1 месяц", PlanID: "period-30"},
			{Days: 90, Amount: 2499, Currency: "USD", Label: "3 месяца", PlanID: "period-90"},
		},
	}

	text, kb := formatOfferDetail(offer)

	assert.Contains(t, text, "Basic")
	assert.Contains(t, text, "Простой тариф")
	assert.Contains(t, text, "1 месяц — 9.99 USD")
	require.Len(t, kb.Rows, 2)
	assert.Equal(t, "buy:period-30", kb.Rows[0][0].CallbackData)
	assert.Equal(t, "buy:period-90", kb.Rows[1][0].CallbackData)
}

func TestFormatSubscriptions(t *testing.T) {
	assert.Equal(t, msgNoSubscriptions, formatSubscriptions(nil, nil))

	subs := []bothost.Subscription{
		{ID: "s1", PlanID: "p1", Status: "active", ExpiresAt: "2026-08-01T00:00:00Z"},
		{ID: "s2", PlanID: "p2", Status: "cancelled"},
	}
	got := formatSubscriptions(subs, map[string]string{"p1": "Basic"})

	assert.Contains(t, got, "Basic — active")
	assert.Contains(t, got, "(до 2026-08-01T00:00:00Z)")
	// Missing plan name falls back to the raw PlanID.
	assert.Contains(t, got, "p2 — cancelled")
}

func TestFormatWallets(t *testing.T) {
	assert.Equal(t, msgNoWallets, formatWallets(nil))

	got := formatWallets([]bothost.Wallet{{Kind: "main", Currency: "USD", Balance: 1500, Available: 1000}})
	assert.Contains(t, got, "main: 15.00 USD (доступно 10.00 USD)")
}

func TestFormatInvoices(t *testing.T) {
	assert.Equal(t, msgNoInvoices, formatInvoices(nil))

	got := formatInvoices([]bothost.Invoice{{ID: "inv-1", Status: "pending", Amount: 999, Currency: "USD"}})
	assert.Contains(t, got, "inv-1 — 9.99 USD (pending)")
}
