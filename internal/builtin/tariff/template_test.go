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
	nextID   int
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
	f.nextID++
	id := "doc-" + string(rune('0'+f.nextID))
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
