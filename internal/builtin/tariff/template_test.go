package tariff

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	gouuid "github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

func TestTariffInput_IsTemplate_RoundTrips(t *testing.T) {
	in := defaultTariffInput()
	in.Name = "Platform Starter"
	in.PriceCurrency = "USD"
	in.DurationDays = 30
	in.IsTemplate = true

	data, err := json.Marshal(in)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"is_template":true`)

	var out TariffInput
	require.NoError(t, json.Unmarshal(data, &out))
	assert.True(t, out.IsTemplate)
}

func TestTariffInput_IsTemplate_DefaultsFalse(t *testing.T) {
	assert.False(t, defaultTariffInput().IsTemplate)
}

func TestHandler_isPlatformActor(t *testing.T) {
	h := &Handler{}

	platformCtx := tenantctx.WithTenantID(context.Background(), tenantctx.PlatformScopeSentinel)
	assert.True(t, h.isPlatformActor(platformCtx))

	shopCtx := tenantctx.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111")
	assert.False(t, h.isPlatformActor(shopCtx))

	assert.False(t, h.isPlatformActor(context.Background()))
}

// fakeStore is an in-memory pluginstore.Store for handler tests. It records
// the raw document bytes passed to InsertDocument so tests can assert what the
// handler persisted.
type fakeStore struct {
	inserted []json.RawMessage
	docs     map[string]*pluginstore.Document // id -> doc (for Get/Update)
}

func newFakeStore() *fakeStore {
	return &fakeStore{docs: map[string]*pluginstore.Document{}}
}

func (f *fakeStore) ListDocuments(_ context.Context, _, _ string) ([]pluginstore.Document, error) {
	out := make([]pluginstore.Document, 0, len(f.docs))
	for _, d := range f.docs {
		out = append(out, *d)
	}
	return out, nil
}

func (f *fakeStore) GetDocument(_ context.Context, _, _, id string) (*pluginstore.Document, error) {
	d, ok := f.docs[id]
	if !ok {
		return nil, pluginstore.ErrDocumentNotFound
	}
	return d, nil
}

func (f *fakeStore) InsertDocument(_ context.Context, pluginSlug, collection string, doc json.RawMessage) (*pluginstore.Document, error) {
	f.inserted = append(f.inserted, doc)
	// Real store IDs are UUIDs; mirror that so DerivePlanID-based paths are
	// testable through the normal InsertDocument API.
	id := gouuid.NewString()
	d := &pluginstore.Document{ID: id, PluginSlug: pluginSlug, Collection: collection, Data: doc}
	f.docs[id] = d
	return d, nil
}

func (f *fakeStore) UpdateDocument(_ context.Context, _, _, id string, doc json.RawMessage) error {
	d, ok := f.docs[id]
	if !ok {
		return pluginstore.ErrDocumentNotFound
	}
	d.Data = doc
	return nil
}

func (f *fakeStore) DeleteDocument(_ context.Context, _, _, id string) error {
	if _, ok := f.docs[id]; !ok {
		return pluginstore.ErrDocumentNotFound
	}
	delete(f.docs, id)
	return nil
}

func newTestHandler(store pluginstore.Store) *Handler {
	return &Handler{collections: store, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func validCreateBody(isTemplate bool) string {
	body := defaultTariffInput()
	body.Name = "Some Tariff"
	body.PriceCurrency = "USD"
	body.DurationDays = 30
	body.IsTemplate = isTemplate
	b, _ := json.Marshal(body)
	return string(b)
}

func TestCreateTariff_ShopActor_CannotSetTemplate(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/api/tariffs", strings.NewReader(validCreateBody(true)))
	req = req.WithContext(tenantctx.WithTenantID(req.Context(), "11111111-1111-1111-1111-111111111111"))
	rec := httptest.NewRecorder()

	h.CreateTariff(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, store.inserted, 1)
	var persisted TariffInput
	require.NoError(t, json.Unmarshal(store.inserted[0], &persisted))
	assert.False(t, persisted.IsTemplate, "shop actor must not be able to persist a template")
}

func TestCreateTariff_PlatformActor_MaySetTemplate(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)

	req := httptest.NewRequest(http.MethodPost, "/api/tariffs", strings.NewReader(validCreateBody(true)))
	req = req.WithContext(tenantctx.WithTenantID(req.Context(), tenantctx.PlatformScopeSentinel))
	rec := httptest.NewRecorder()

	h.CreateTariff(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Len(t, store.inserted, 1)
	var persisted TariffInput
	require.NoError(t, json.Unmarshal(store.inserted[0], &persisted))
	assert.True(t, persisted.IsTemplate, "platform actor must be able to persist a template")
}

// withURLParam attaches a chi route param to the request context so handlers
// that call chi.URLParam(r, key) resolve it under test.
func withURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// seedTemplate inserts a stored template document (is_template=true) and
// returns its ID.
func seedTemplate(t *testing.T, store *fakeStore) string {
	t.Helper()
	tmpl := defaultTariffInput()
	tmpl.Name = "Shared Template"
	tmpl.PriceCurrency = "USD"
	tmpl.DurationDays = 30
	tmpl.IsTemplate = true
	b, err := json.Marshal(tmpl)
	require.NoError(t, err)
	doc, err := store.InsertDocument(context.Background(), PluginSlug, CollectionName, b)
	require.NoError(t, err)
	return doc.ID
}

func TestUpdateTariff_ShopActor_CannotWriteTemplate(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)
	id := seedTemplate(t, store)

	req := httptest.NewRequest(http.MethodPut, "/api/tariffs/"+id, strings.NewReader(validCreateBody(false)))
	req = req.WithContext(tenantctx.WithTenantID(req.Context(), "11111111-1111-1111-1111-111111111111"))
	req = withURLParam(req, "tariffID", id)
	rec := httptest.NewRecorder()

	h.UpdateTariff(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUpdateTariff_PlatformActor_MayWriteTemplate(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)
	id := seedTemplate(t, store)

	req := httptest.NewRequest(http.MethodPut, "/api/tariffs/"+id, strings.NewReader(validCreateBody(true)))
	req = req.WithContext(tenantctx.WithTenantID(req.Context(), tenantctx.PlatformScopeSentinel))
	req = withURLParam(req, "tariffID", id)
	rec := httptest.NewRecorder()

	h.UpdateTariff(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestDeleteTariff_ShopActor_CannotDeleteTemplate(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)
	id := seedTemplate(t, store)

	req := httptest.NewRequest(http.MethodDelete, "/api/tariffs/"+id, nil)
	req = req.WithContext(tenantctx.WithTenantID(req.Context(), "11111111-1111-1111-1111-111111111111"))
	req = withURLParam(req, "tariffID", id)
	rec := httptest.NewRecorder()

	h.DeleteTariff(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// seedShopTariff inserts a non-template tariff and returns its ID.
func seedShopTariff(t *testing.T, store *fakeStore, name string) string {
	t.Helper()
	in := defaultTariffInput()
	in.Name = name
	in.PriceCurrency = "USD"
	in.DurationDays = 30
	in.IsActive = true
	in.IsTemplate = false
	b, err := json.Marshal(in)
	require.NoError(t, err)
	doc, err := store.InsertDocument(context.Background(), PluginSlug, CollectionName, b)
	require.NoError(t, err)
	return doc.ID
}

func listTariffsNames(t *testing.T, h *Handler, ctx context.Context) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/tariffs", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListTariffs(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var got []TariffResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	names := make([]string, 0, len(got))
	for _, tr := range got {
		names = append(names, tr.Name)
	}
	return names
}

func countOccurrences(ss []string, want string) int {
	n := 0
	for _, s := range ss {
		if s == want {
			n++
		}
	}
	return n
}

func TestListTariffs_Merge_PlatformReadDoesNotDoubleCount(t *testing.T) {
	// fakeStore is tenancy-blind: every read returns every row. This test only
	// proves the MERGE dedups by id (a platform actor's own read already
	// includes templates, so the second platform-scoped read must not
	// double-count). It does NOT prove template visibility — see the real-RLS
	// integration test TestListTariffs_RLS_ShopSeesOwnPlusTemplates.
	store := newFakeStore()
	h := newTestHandler(store)
	seedTemplate(t, store)             // "Shared Template" (is_template=true)
	seedShopTariff(t, store, "Shop A") // own-tenant

	platformCtx := tenantctx.WithTenantID(context.Background(), tenantctx.PlatformScopeSentinel)
	names := listTariffsNames(t, h, platformCtx)
	// Each name appears exactly once despite two reads over the same fakeStore.
	assert.Equal(t, 1, countOccurrences(names, "Shared Template"))
	assert.Equal(t, 1, countOccurrences(names, "Shop A"))
}

func TestListTariffs_Merge_OwnTariffPresent(t *testing.T) {
	store := newFakeStore()
	h := newTestHandler(store)
	seedShopTariff(t, store, "Shop A")

	shopCtx := tenantctx.WithTenantID(context.Background(), "11111111-1111-1111-1111-111111111111")
	names := listTariffsNames(t, h, shopCtx)
	assert.Contains(t, names, "Shop A")
}
