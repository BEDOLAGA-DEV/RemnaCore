package tariff

import (
	"context"
	"encoding/json"
	"testing"

	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
)

// ── DerivePlanID ──────────────────────────────────────────────────────────────

func TestDerivePlanID_SinglePeriod_ReturnsDocID(t *testing.T) {
	const docID = "550e8400-e29b-41d4-a716-446655440000"
	planID, err := DerivePlanID(docID, 30, false)
	require.NoError(t, err)
	assert.Equal(t, docID, planID, "single-period tariff must use docID as planID")
}

func TestDerivePlanID_MultiPeriod_MatchesSyncSideDerivation(t *testing.T) {
	const docID = "550e8400-e29b-41d4-a716-446655440000"

	// Reproduce the sync-side derivation directly.
	ns, _ := gouuid.Parse(docID)
	want30 := gouuid.NewSHA1(ns, []byte("period_30")).String()
	want90 := gouuid.NewSHA1(ns, []byte("period_90")).String()

	got30, err := DerivePlanID(docID, 30, true)
	require.NoError(t, err)
	assert.Equal(t, want30, got30, "30-day period PlanID must match sync-side derivation")

	got90, err := DerivePlanID(docID, 90, true)
	require.NoError(t, err)
	assert.Equal(t, want90, got90, "90-day period PlanID must match sync-side derivation")

	assert.NotEqual(t, got30, got90, "different durations must produce different PlanIDs")
	assert.NotEqual(t, docID, got30, "multi-period PlanID must differ from docID")
}

func TestDerivePlanID_InvalidUUID_ReturnsError(t *testing.T) {
	_, err := DerivePlanID("not-a-uuid", 30, true)
	assert.Error(t, err)
}

// ── ListVisibleTariffs ────────────────────────────────────────────────────────

func TestListVisibleTariffs_Telegram_FiltersActiveAndVisibility(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	seedTariff := func(name string, isActive, visibleInTelegram bool) {
		t.Helper()
		in := defaultTariffInput()
		in.Name = name
		in.PriceCurrency = "USD"
		in.DurationDays = 30
		in.IsActive = isActive
		in.VisibleInTelegram = visibleInTelegram
		b, _ := json.Marshal(in)
		_, err := store.InsertDocument(context.Background(), PluginSlug, CollectionName, b)
		require.NoError(t, err)
	}

	seedTariff("Active+Visible", true, true)    // must appear
	seedTariff("Active+Hidden", true, false)    // filtered: not visible in telegram
	seedTariff("Inactive+Visible", false, true) // filtered: not active

	result, err := h.ListVisibleTariffs(context.Background(), "telegram")
	require.NoError(t, err)

	require.Len(t, result, 1)
	assert.Equal(t, "Active+Visible", result[0].Name)
}

func TestListVisibleTariffs_UnknownChannel_DefaultsToTelegram(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	in := defaultTariffInput()
	in.Name = "TelegramOnly"
	in.IsActive = true
	in.VisibleInTelegram = true
	in.VisibleInCabinet = false
	b, _ := json.Marshal(in)
	_, err := store.InsertDocument(context.Background(), PluginSlug, CollectionName, b)
	require.NoError(t, err)

	result, err := h.ListVisibleTariffs(context.Background(), "unknown-channel")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "TelegramOnly", result[0].Name)
}

// ── GetTariffByPlanID ─────────────────────────────────────────────────────────

func TestGetTariffByPlanID_SinglePeriod_DocIDIsPlanID(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	in := defaultTariffInput()
	in.Name = "Solo"
	in.IsActive = true
	in.VisibleInTelegram = true
	in.DurationDays = 30
	b, err := json.Marshal(in)
	require.NoError(t, err)
	doc, err := store.InsertDocument(context.Background(), PluginSlug, CollectionName, b)
	require.NoError(t, err)

	got, err := h.GetTariffByPlanID(context.Background(), doc.ID)
	require.NoError(t, err)
	assert.Equal(t, "Solo", got.Name)
}

func TestGetTariffByPlanID_MultiPeriod_DerivedPlanID(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	in := defaultTariffInput()
	in.Name = "Multi"
	in.IsActive = true
	in.VisibleInTelegram = true
	in.PricingPeriods = []PricingPeriod{{DurationDays: 30}, {DurationDays: 90}}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	doc, err := store.InsertDocument(context.Background(), PluginSlug, CollectionName, b)
	require.NoError(t, err)

	derived, err := DerivePlanID(doc.ID, 90, true)
	require.NoError(t, err)

	got, err := h.GetTariffByPlanID(context.Background(), derived)
	require.NoError(t, err)
	assert.Equal(t, "Multi", got.Name)
}

func TestGetTariffByPlanID_NotFound_WrapsErrDocumentNotFound(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	_, err := h.GetTariffByPlanID(context.Background(), "00000000-0000-0000-0000-000000000000")
	require.ErrorIs(t, err, pluginstore.ErrDocumentNotFound)
}
