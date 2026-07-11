//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// setupRLSDB starts a PostgreSQL 18 container with identity, reseller, and
// RLS migrations applied. Returns a connected pool.
func setupRLSDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgres:18",
		tcpostgres.WithDatabase(testDBName),
		tcpostgres.WithUsername(testDBUser),
		tcpostgres.WithPassword(testDBPass),
		tcpostgres.WithInitScripts(allMigrationScripts(t)...),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(testContainerStartupTimeout),
		),
	)
	if err != nil {
		failOrSkip(t, "skipping integration test: could not start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = ctr.Terminate(context.Background()) })

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}

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
		ID:            uuid.Must(uuid.NewV7()).String(),
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

func TestRLS_TenantIsolation(t *testing.T) {
	pool := setupRLSDB(t)
	repo := postgres.NewIdentityRepository(pool)
	txm := postgres.NewTxManager(pool)
	ctx := context.Background()

	tenantA := uuid.Must(uuid.NewV7()).String()
	tenantB := uuid.Must(uuid.NewV7()).String()

	// Seed: one user per tenant + one platform-level user.
	userA := newTenantUser(t, tenantA)
	userB := newTenantUser(t, tenantB)
	userPlatform := newPlatformUser(t)

	// Insert all users without tenant scoping (using raw pool to bypass RLS
	// via SET LOCAL for each insert — we need to set the matching tenant).
	insertWithTenant := func(user *identity.PlatformUser, tenantID string) {
		t.Helper()
		err := txm.RunInTx(tenantctx.WithTenantID(ctx, tenantID), func(txCtx context.Context) error {
			return repo.CreateUser(txCtx, user)
		})
		require.NoError(t, err)
	}

	insertWithTenant(userA, tenantA)
	insertWithTenant(userB, tenantB)
	// Platform user has no tenant — insert without tenant context.
	insertWithTenant(userPlatform, "")

	t.Run("tenant_A_sees_own_users_and_platform", func(t *testing.T) {
		var users []*identity.PlatformUser
		err := txm.RunInTx(tenantctx.WithTenantID(ctx, tenantA), func(txCtx context.Context) error {
			var listErr error
			users, listErr = repo.ListUsers(txCtx, 100, 0)
			return listErr
		})
		require.NoError(t, err)

		ids := extractUserIDs(users)
		assert.Contains(t, ids, userA.ID, "tenant A should see its own user")
		assert.Contains(t, ids, userPlatform.ID, "tenant A should see platform-level users")
		assert.NotContains(t, ids, userB.ID, "tenant A must NOT see tenant B users")
	})

	t.Run("tenant_B_sees_own_users_and_platform", func(t *testing.T) {
		var users []*identity.PlatformUser
		err := txm.RunInTx(tenantctx.WithTenantID(ctx, tenantB), func(txCtx context.Context) error {
			var listErr error
			users, listErr = repo.ListUsers(txCtx, 100, 0)
			return listErr
		})
		require.NoError(t, err)

		ids := extractUserIDs(users)
		assert.Contains(t, ids, userB.ID, "tenant B should see its own user")
		assert.Contains(t, ids, userPlatform.ID, "tenant B should see platform-level users")
		assert.NotContains(t, ids, userA.ID, "tenant B must NOT see tenant A users")
	})

	t.Run("no_tenant_sees_only_platform_users", func(t *testing.T) {
		var users []*identity.PlatformUser
		err := txm.RunInTx(ctx, func(txCtx context.Context) error {
			var listErr error
			users, listErr = repo.ListUsers(txCtx, 100, 0)
			return listErr
		})
		require.NoError(t, err)

		ids := extractUserIDs(users)
		assert.Contains(t, ids, userPlatform.ID, "no-tenant context should see platform-level users")
		assert.NotContains(t, ids, userA.ID, "no-tenant context must NOT see tenant A users")
		assert.NotContains(t, ids, userB.ID, "no-tenant context must NOT see tenant B users")
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
