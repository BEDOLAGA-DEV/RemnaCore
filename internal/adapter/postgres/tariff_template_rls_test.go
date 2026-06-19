//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// PLACEMENT NOTE: this real-RLS test for C6.5's ListTariffs two-read merge lives
// in package postgres_test (NOT internal/builtin/tariff as the C6.5 spec sketch
// suggested) because it MUST reuse the single shared non-superuser helper
// connectAsRLSApp (rls_testutil_test.go, C2) plus setupTestDBWith — both are
// unexported test helpers in package postgres_test and cannot be imported across
// package boundaries. Per the standing dedup rule, connectAsRLSApp is NOT
// redefined here. The handler under test is the real tariff.Handler wired over
// the real CollectionsRepository, so the assertions exercise the 041 RLS policy
// end-to-end exactly as the spec intended.

// tariffTemplateRLSMigrations is the minimal migration chain to stand up
// plugins.collections with its 041 tenant_id + RLS policy. Mirrors the chain in
// collections_repo_test.go (inline set_updated_at stub → 004 → 034 → 041).
var tariffTemplateRLSMigrations = []string{
	inlineSetUpdatedAt,
	"004_plugins.sql",
	"034_plugin_collections.sql",
	"041_plugin_collections_tenant.sql",
}

// newTariffRLSHandler wires the real CollectionsRepository over appPool into a
// real tariff.Handler. ListTariffs uses only the collections store and logger;
// pluginRepo and planRepo are unused on the list path and left nil.
func newTariffRLSHandler(t *testing.T, appPool *pgxpool.Pool) *tariff.Handler {
	t.Helper()
	repo := postgres.NewCollectionsRepository(appPool, postgres.NewTxManager(appPool))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return tariff.NewHandler(repo, nil, nil, logger)
}

// seedCollectionTariffRow inserts a tariffs-collection row via the admin pool
// with an explicit tenant_id (pass the empty string for a NULL-tenant platform
// template). Seeding runs under the platform sentinel GUC so any FORCE-RLS touch
// is permitted. name and isTemplate are stamped into the JSON document so the
// handler's documentToTariff conversion surfaces them.
func seedCollectionTariffRow(t *testing.T, admin *pgxpool.Pool, tenantID, name string, isTemplate bool) {
	t.Helper()
	ctx := context.Background()
	in := map[string]any{
		"name":           name,
		"price_currency": "USD",
		"duration_days":  30,
		"is_active":      true,
		"is_template":    isTemplate,
	}
	doc, err := json.Marshal(in)
	require.NoError(t, err)

	tx, err := admin.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantctx.PlatformScopeSentinel)
	require.NoError(t, err)

	id := uuid.Must(uuid.NewV7()).String()
	if tenantID == "" {
		_, err = tx.Exec(ctx,
			`INSERT INTO plugins.collections (id, plugin_slug, collection, document, tenant_id)
			 VALUES ($1, $2, $3, $4, NULL)`,
			id, tariff.PluginSlug, tariff.CollectionName, doc)
	} else {
		_, err = tx.Exec(ctx,
			`INSERT INTO plugins.collections (id, plugin_slug, collection, document, tenant_id)
			 VALUES ($1, $2, $3, $4, $5)`,
			id, tariff.PluginSlug, tariff.CollectionName, doc, tenantID)
	}
	require.NoError(t, err, "seed tariff row %q", name)
	require.NoError(t, tx.Commit(ctx))
}

// listTariffNamesUnder drives the real handler's ListTariffs under the given
// GUC and returns the tariff names in the JSON response.
func listTariffNamesUnder(t *testing.T, h *tariff.Handler, ctx context.Context) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/tariffs", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListTariffs(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var got []struct {
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	names := make([]string, 0, len(got))
	for _, tr := range got {
		names = append(names, tr.Name)
	}
	return names
}

// TestListTariffs_RLS_ShopSeesOwnPlusTemplates is the positive template-visibility
// + foreign-row-invisibility assertion the tenancy-blind fakeStore cannot make.
// Under shop A's GUC the merged list contains "Shop A" (own-tenant, normal RLS
// read), contains "Shared Template" (NULL-tenant template surfaced by the
// WithPlatformScope read inside ListTariffs), and does NOT contain "Shop B Secret"
// (foreign non-template, never returned by either read — RLS keeps it out).
func TestListTariffs_RLS_ShopSeesOwnPlusTemplates(t *testing.T) {
	admin, connStr := setupTestDBWith(t, tariffTemplateRLSMigrations...)
	ctx := context.Background()
	// Connect as the shared NON-superuser rls_app role so FORCE RLS is genuinely
	// enforced on the handler's reads (a superuser pool would pass vacuously).
	appPool := connectAsRLSApp(t, admin, connStr)

	shopA := uuid.Must(uuid.NewV7()).String()
	shopB := uuid.Must(uuid.NewV7()).String()
	seedCollectionTariffRow(t, admin, "", "Shared Template", true)         // tenant_id NULL, is_template=true
	seedCollectionTariffRow(t, admin, shopA, "Shop A", false)              // tenant_id = shopA
	seedCollectionTariffRow(t, admin, shopB, "Shop B Secret", false)       // tenant_id = shopB, non-template

	h := newTariffRLSHandler(t, appPool)
	shopCtx := tenantctx.WithTenantID(ctx, shopA)
	names := listTariffNamesUnder(t, h, shopCtx)

	assert.Contains(t, names, "Shop A", "own-tenant tariff via normal RLS read")
	assert.Contains(t, names, "Shared Template", "template via WithPlatformScope read")
	assert.NotContains(t, names, "Shop B Secret", "foreign non-template never surfaced")
}
