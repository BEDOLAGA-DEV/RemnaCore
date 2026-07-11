//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// newTenantUser creates a PlatformUser associated with the given tenant ID.
// Pass an empty string for a platform-level user (nil tenant_id).
func newTenantUser(t *testing.T, tenantID string) *identity.PlatformUser {
	t.Helper()
	now := time.Now().Truncate(time.Microsecond)

	var tid *string
	if tenantID != "" {
		tid = &tenantID
	}

	return &identity.PlatformUser{
		ID: uuid.Must(uuid.NewV7()).String(),
		// Full UUID (not a truncated prefix): UUIDv7's leading bits are the
		// millisecond timestamp, so several users minted in the same tick would
		// share a truncated prefix and collide on the unique email constraint.
		Email:         fmt.Sprintf("user-%s@test.com", uuid.Must(uuid.NewV7()).String()),
		PasswordHash:  "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012",
		EmailVerified: false,
		Role:          identity.RoleCustomer,
		TenantID:      tid,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// newPlatformUser creates a PlatformUser without any tenant (platform-level).
func newPlatformUser(t *testing.T) *identity.PlatformUser {
	t.Helper()
	return newTenantUser(t, "")
}

// TestRLS_TenantIsolation exercises the identity.platform_users tenant-isolation
// policy through a NON-superuser, NON-BYPASSRLS role (connectAsRLSApp). Running
// as the testcontainer superuser would bypass RLS entirely and make every
// assertion pass vacuously — the false-assurance gap flagged by the Phase-C audit.
//
// Post-040/044 read semantics (USING has NO `tenant_id IS NULL` branch):
//   - platform sentinel '*'  → sees ALL rows.
//   - a shop-UUID GUC        → sees ONLY that tenant's rows (never NULL/platform).
//   - unset/empty GUC        → sees NOTHING (fail-closed).
func TestRLS_TenantIsolation(t *testing.T) {
	admin, connStr := setupTestDBWith(t)
	ctx := context.Background()
	// Connect as the shared NON-superuser rls_app role first (it creates the
	// role), then grant it the identity/reseller schemas it needs to read/write.
	pool := connectAsRLSApp(t, admin, connStr)
	grantShopBotSchemasToRLSApp(t, ctx, admin)

	repo := postgres.NewIdentityRepository(pool)
	txm := postgres.NewTxManager(pool)

	tenantA := uuid.Must(uuid.NewV7()).String()
	tenantB := uuid.Must(uuid.NewV7()).String()

	// Seed: one user per tenant + one platform-level (NULL-tenant) user.
	userA := newTenantUser(t, tenantA)
	userB := newTenantUser(t, tenantB)
	userPlatform := newPlatformUser(t)

	// Each insert runs under the GUC its WITH CHECK requires: a shop GUC stamps
	// the matching tenant_id; the platform user (tenant_id NULL) is written under
	// the sentinel scope.
	insertScoped := func(user *identity.PlatformUser, scope context.Context) {
		t.Helper()
		err := txm.RunInTx(scope, func(txCtx context.Context) error {
			return repo.CreateUser(txCtx, user)
		})
		require.NoError(t, err)
	}

	insertScoped(userA, tenantctx.WithTenantID(ctx, tenantA))
	insertScoped(userB, tenantctx.WithTenantID(ctx, tenantB))
	insertScoped(userPlatform, tenantctx.WithPlatformScope(ctx))

	listUnder := func(t *testing.T, scope context.Context) []string {
		t.Helper()
		var users []*identity.PlatformUser
		err := txm.RunInTx(scope, func(txCtx context.Context) error {
			var listErr error
			users, listErr = repo.ListUsers(txCtx, 100, 0)
			return listErr
		})
		require.NoError(t, err)
		return extractUserIDs(users)
	}

	t.Run("tenant_A_sees_only_its_own_users", func(t *testing.T) {
		ids := listUnder(t, tenantctx.WithTenantID(ctx, tenantA))
		assert.Contains(t, ids, userA.ID, "tenant A should see its own user")
		assert.NotContains(t, ids, userB.ID, "tenant A must NOT see tenant B users")
		assert.NotContains(t, ids, userPlatform.ID, "tenant A must NOT see NULL-tenant platform users (USING has no IS NULL branch)")
	})

	t.Run("tenant_B_sees_only_its_own_users", func(t *testing.T) {
		ids := listUnder(t, tenantctx.WithTenantID(ctx, tenantB))
		assert.Contains(t, ids, userB.ID, "tenant B should see its own user")
		assert.NotContains(t, ids, userA.ID, "tenant B must NOT see tenant A users")
		assert.NotContains(t, ids, userPlatform.ID, "tenant B must NOT see NULL-tenant platform users")
	})

	t.Run("platform_scope_sees_all_users", func(t *testing.T) {
		ids := listUnder(t, tenantctx.WithPlatformScope(ctx))
		assert.Contains(t, ids, userA.ID, "platform sentinel should see tenant A users")
		assert.Contains(t, ids, userB.ID, "platform sentinel should see tenant B users")
		assert.Contains(t, ids, userPlatform.ID, "platform sentinel should see platform-level users")
	})

	t.Run("empty_context_sees_nothing", func(t *testing.T) {
		ids := listUnder(t, ctx)
		assert.NotContains(t, ids, userA.ID, "unset GUC must fail closed (no tenant A)")
		assert.NotContains(t, ids, userB.ID, "unset GUC must fail closed (no tenant B)")
		assert.NotContains(t, ids, userPlatform.ID, "unset GUC must fail closed (no platform users)")
	})

	t.Run("get_by_id_respects_rls", func(t *testing.T) {
		// Tenant A can read its own user by ID.
		err := txm.RunInTx(tenantctx.WithTenantID(ctx, tenantA), func(txCtx context.Context) error {
			got, getErr := repo.GetUserByID(txCtx, userA.ID)
			if getErr != nil {
				return getErr
			}
			assert.Equal(t, userA.ID, got.ID)
			return nil
		})
		require.NoError(t, err)

		// Tenant A cannot read tenant B's user by ID.
		err = txm.RunInTx(tenantctx.WithTenantID(ctx, tenantA), func(txCtx context.Context) error {
			_, getErr := repo.GetUserByID(txCtx, userB.ID)
			return getErr
		})
		assert.ErrorIs(t, err, identity.ErrNotFound, "tenant A reading tenant B user by ID should fail")
	})
}

// extractUserIDs collects IDs from a slice of PlatformUser pointers.
func extractUserIDs(users []*identity.PlatformUser) []string {
	ids := make([]string, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
	}
	return ids
}
