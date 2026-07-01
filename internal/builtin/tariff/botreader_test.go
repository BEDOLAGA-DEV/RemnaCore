package tariff

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram/bothost"
)

// testLogger returns a logger that discards output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubVisibleLister is a test double for visibleTariffLister.
type stubVisibleLister struct {
	tariffs []TariffResponse
	byPlan  *TariffResponse
}

func (s *stubVisibleLister) ListVisibleTariffs(_ context.Context, _ string) ([]TariffResponse, error) {
	return s.tariffs, nil
}

func (s *stubVisibleLister) GetTariffByPlanID(_ context.Context, _ string) (*TariffResponse, error) {
	return s.byPlan, nil
}

// ── TariffReaderAdapter.ListVisible ──────────────────────────────────────────

func TestTariffReaderAdapter_TwoPeriods_CorrectPlanIDs(t *testing.T) {
	const docID = "550e8400-e29b-41d4-a716-446655440001"

	resp := TariffResponse{}
	resp.ID = docID
	resp.Name = "Pro Plan"
	resp.Description = "Great plan"
	resp.PriceCurrency = "USD"
	resp.IsActive = true
	resp.VisibleInTelegram = true
	resp.PricingPeriods = []PricingPeriod{
		{DurationDays: 30, PriceAmount: 999, Label: "1 month"},
		{DurationDays: 90, PriceAmount: 2499, Label: "3 months"},
	}

	adapter := NewTariffReaderAdapter(&stubVisibleLister{tariffs: []TariffResponse{resp}}, testLogger())

	// Compile-time interface check (also asserted by the var _ in botreader.go).
	var _ bothost.TariffReader = adapter

	offers, err := adapter.ListVisible(context.Background(), "telegram")
	require.NoError(t, err)
	require.Len(t, offers, 1)

	offer := offers[0]
	assert.Equal(t, docID, offer.PlanID)
	assert.Equal(t, "Pro Plan", offer.Name)
	assert.Equal(t, "Great plan", offer.Description)
	require.Len(t, offer.Periods, 2)

	// per-period PlanIDs must match DerivePlanID (same derivation as syncTariffToPlan)
	want30, err := DerivePlanID(docID, 30, true)
	require.NoError(t, err)
	want90, err := DerivePlanID(docID, 90, true)
	require.NoError(t, err)

	assert.Equal(t, want30, offer.Periods[0].PlanID)
	assert.Equal(t, want90, offer.Periods[1].PlanID)
	assert.Equal(t, 30, offer.Periods[0].Days)
	assert.Equal(t, 90, offer.Periods[1].Days)
	assert.Equal(t, int64(999), offer.Periods[0].Amount)
	assert.Equal(t, int64(2499), offer.Periods[1].Amount)
	assert.Equal(t, "USD", offer.Periods[0].Currency)
	assert.Equal(t, "1 month", offer.Periods[0].Label)
	assert.Equal(t, "3 months", offer.Periods[1].Label)
}

func TestTariffReaderAdapter_SinglePeriod_UsesDocIDasPlanID(t *testing.T) {
	const docID = "550e8400-e29b-41d4-a716-446655440002"

	resp := TariffResponse{}
	resp.ID = docID
	resp.Name = "Basic"
	resp.PriceCurrency = "EUR"
	resp.IsActive = true
	resp.VisibleInTelegram = true
	resp.DurationDays = 30
	resp.PriceAmount = 500
	// no PricingPeriods → single-period tariff

	adapter := NewTariffReaderAdapter(&stubVisibleLister{tariffs: []TariffResponse{resp}}, testLogger())

	offers, err := adapter.ListVisible(context.Background(), "telegram")
	require.NoError(t, err)
	require.Len(t, offers, 1)

	offer := offers[0]
	assert.Equal(t, docID, offer.PlanID)
	require.Len(t, offer.Periods, 1)
	assert.Equal(t, docID, offer.Periods[0].PlanID, "single-period PlanID must equal docID")
	assert.Equal(t, 30, offer.Periods[0].Days)
	assert.Equal(t, int64(500), offer.Periods[0].Amount)
	assert.Equal(t, "EUR", offer.Periods[0].Currency)
}

func TestTariffReaderAdapter_Get_DelegatesAndMaps(t *testing.T) {
	const docID = "550e8400-e29b-41d4-a716-446655440003"

	resp := TariffResponse{}
	resp.ID = docID
	resp.Name = "Standard"
	resp.PriceCurrency = "USD"
	resp.PricingPeriods = []PricingPeriod{
		{DurationDays: 30, PriceAmount: 700, Label: "month"},
	}

	want30, err := DerivePlanID(docID, 30, true)
	require.NoError(t, err)

	adapter := NewTariffReaderAdapter(&stubVisibleLister{byPlan: &resp}, testLogger())

	offer, err := adapter.Get(context.Background(), want30)
	require.NoError(t, err)
	require.NotNil(t, offer)
	assert.Equal(t, docID, offer.PlanID)
	require.Len(t, offer.Periods, 1)
	assert.Equal(t, want30, offer.Periods[0].PlanID)
}
