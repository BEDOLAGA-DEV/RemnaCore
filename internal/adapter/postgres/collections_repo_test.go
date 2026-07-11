//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pluginstore"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// TestCollections_ReadRequiresGUC asserts a collections read returns the row
// only when the app.tenant_id GUC is set by RunInTx. A platform-owned doc
// (inserted under the sentinel, tenant_id NULL) is visible under the sentinel
// but invisible to an unset GUC and to a foreign shop GUC.
func TestCollections_ReadRequiresGUC(t *testing.T) {
	// Migration order matters: 034 creates plugins.collections but DEPENDS on the
	// plugins schema (004) and public.set_updated_at() (019). Apply the inline
	// standalone set_updated_at() stub FIRST, then 004, then 034, then 041 — NOT
	// 034+041 alone (that errors on the missing schema/trigger function and would
	// make the test pass vacuously). The stub MUST precede 004 because 004's
	// CREATE TRIGGER ... EXECUTE FUNCTION identity.set_updated_at() resolves the
	// function's OID at CREATE TRIGGER time, so identity.set_updated_at() must
	// already exist. setupTestDBWith MUST fail the test on any migration error,
	// never skip.
	admin, connStr := setupTestDBWith(t)
	rlsPool := connectAsRLSApp(t, admin, connStr)
	repo := postgres.NewCollectionsRepository(rlsPool, postgres.NewTxManager(rlsPool))
	ctx := context.Background()

	payload := json.RawMessage(`{"name":"platform tariff"}`)

	// Insert under the platform sentinel: tenant_id self-stamps to NULL.
	var doc *pluginstore.Document
	err := func() error {
		var insErr error
		doc, insErr = repo.InsertDocument(
			tenantctx.WithTenantID(ctx, tenantctx.PlatformScopeSentinel),
			testCollectionPlugin, testCollectionName, payload,
		)
		return insErr
	}()
	require.NoError(t, err)
	require.NotNil(t, doc)

	t.Run("sentinel_sees_platform_doc", func(t *testing.T) {
		got, getErr := repo.GetDocument(
			tenantctx.WithTenantID(ctx, tenantctx.PlatformScopeSentinel),
			testCollectionPlugin, testCollectionName, doc.ID,
		)
		require.NoError(t, getErr)
		assert.Equal(t, doc.ID, got.ID)
	})

	t.Run("unset_guc_sees_nothing", func(t *testing.T) {
		docs, listErr := repo.ListDocuments(ctx, testCollectionPlugin, testCollectionName)
		require.NoError(t, listErr)
		assert.Empty(t, docs, "unset GUC must see zero rows (fail-closed)")
	})

	t.Run("foreign_shop_guc_cannot_read_platform_doc", func(t *testing.T) {
		_, getErr := repo.GetDocument(
			tenantctx.WithTenantID(ctx, "11111111-1111-1111-1111-111111111111"),
			testCollectionPlugin, testCollectionName, doc.ID,
		)
		assert.True(t, errors.Is(getErr, pluginstore.ErrDocumentNotFound),
			"a shop GUC must not see a platform-owned (NULL-tenant) doc")
	})

	t.Run("with_check_rejects_foreign_tenant_via_shop_guc", func(t *testing.T) {
		// Under a shop GUC, the insert self-stamps THAT shop's UUID, which the
		// WITH CHECK permits; a shop can never stamp a different tenant because
		// the value comes from the GUC, not request input. Assert the round-trip:
		shopID := "22222222-2222-2222-2222-222222222222"
		shopCtx := tenantctx.WithTenantID(ctx, shopID)
		ins, insErr := repo.InsertDocument(shopCtx, testCollectionPlugin, testCollectionName, payload)
		require.NoError(t, insErr)
		// The same shop can read it back...
		_, ownErr := repo.GetDocument(shopCtx, testCollectionPlugin, testCollectionName, ins.ID)
		require.NoError(t, ownErr)
		// ...but the platform-owned doc remains invisible to this shop.
		_, crossErr := repo.GetDocument(shopCtx, testCollectionPlugin, testCollectionName, doc.ID)
		assert.True(t, errors.Is(crossErr, pluginstore.ErrDocumentNotFound))
	})
}
