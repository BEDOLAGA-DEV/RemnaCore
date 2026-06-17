//go:build integration

// Package postgres_test — IAM integration tests (Phase B).
//
// Scope: exercises the RBAC repo (AssignRole/RevokeRole/GetRole/CountPlatformAdmins)
// and the identity repo invitation round-trip (CreateInvitation/GetInvitationByToken/
// GetInvitationByID/DeleteInvitation) against a real Postgres 18 instance started
// by testcontainers.
//
// Full IdentityAdminService wiring (SessionIssuer, ShopProvisioner, TxManager,
// publisher, etc.) is out of scope here: the service-layer unit tests in
// internal/domain/identity/service/admin_service_test.go already cover those
// interactions with in-process stubs. This file guards the persistence layer.
//
// Run with:
//
//	go test -tags integration -v ./internal/adapter/postgres/ -run TestIAM
//
// Requires Docker; skipped automatically when Docker is unavailable.
package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
)

// setupIAMTestDB boots a Postgres container with all four migrations required
// by IAM Phase B: identity, reseller, rbac, and invitations.
func setupIAMTestDB(t *testing.T) *postgres.RBACRepository {
	t.Helper()
	pool := setupTestDBWith(t,
		"001_identity.sql",
		"006_reseller.sql",
		"038_rbac.sql",
		"039_invitations.sql",
	)
	repo := postgres.NewRBACRepository(pool)

	// Seed catalog so system-role rows (and their role_ids) exist.
	ctx := context.Background()
	require.NoError(t, repo.SyncCatalog(ctx, rbac.Catalog(), rbac.SystemRoles()))

	return repo
}

// ── RBAC repo: AssignRole / RevokeRole / GetRole / CountPlatformAdmins ───────

func TestIAM_RBACRepo_AssignRevoke(t *testing.T) {
	pool := setupTestDBWith(t,
		"001_identity.sql",
		"006_reseller.sql",
		"038_rbac.sql",
		"039_invitations.sql",
	)
	repo := postgres.NewRBACRepository(pool)
	ctx := context.Background()

	require.NoError(t, repo.SyncCatalog(ctx, rbac.Catalog(), rbac.SystemRoles()))

	// Insert two legacy users to use as actor and target.
	adminID := insertLegacyUser(t, pool, "admin2@iam.test", "admin")
	staffID := insertLegacyUser(t, pool, "staff@iam.test", "customer")

	// GetRole returns the shop_staff role.
	staffRole, err := repo.GetRole(ctx, rbac.RoleShopStaff)
	require.NoError(t, err)
	assert.Equal(t, rbac.RoleShopStaff, staffRole.Key)
	assert.Equal(t, rbac.ScopeShop, staffRole.ScopeKind)

	// ErrRoleNotFound for a non-existent key.
	_, err = repo.GetRole(ctx, "nonexistent_role_key")
	assert.ErrorIs(t, err, rbac.ErrRoleNotFound)

	// AssignRole: give staffID the shop_staff role scoped to a fake tenant.
	tenantID := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, repo.AssignRole(ctx, staffID, staffRole.ID, &tenantID, adminID))

	// Idempotent: second assign does not error.
	require.NoError(t, repo.AssignRole(ctx, staffID, staffRole.ID, &tenantID, adminID))

	// Binding is visible in ListBindingsForUser.
	bindings, err := repo.ListBindingsForUser(ctx, staffID)
	require.NoError(t, err)
	var found bool
	for _, b := range bindings {
		if b.RoleKey == rbac.RoleShopStaff && b.TenantID != nil && *b.TenantID == tenantID {
			found = true
		}
	}
	assert.True(t, found, "shop_staff binding for tenant must be present after AssignRole")

	// RevokeRole: remove the binding.
	n, err := repo.RevokeRole(ctx, staffID, staffRole.ID, &tenantID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)

	// Binding is gone.
	bindings2, err := repo.ListBindingsForUser(ctx, staffID)
	require.NoError(t, err)
	for _, b := range bindings2 {
		assert.False(t, b.RoleKey == rbac.RoleShopStaff && b.TenantID != nil && *b.TenantID == tenantID,
			"shop_staff binding must be absent after RevokeRole")
	}

	// Revoke of a non-existent binding returns 0 rows affected (no error).
	n2, err := repo.RevokeRole(ctx, staffID, staffRole.ID, &tenantID)
	require.NoError(t, err)
	assert.EqualValues(t, 0, n2)
}

func TestIAM_RBACRepo_CountPlatformAdmins(t *testing.T) {
	pool := setupTestDBWith(t,
		"001_identity.sql",
		"006_reseller.sql",
		"038_rbac.sql",
		"039_invitations.sql",
	)
	repo := postgres.NewRBACRepository(pool)
	ctx := context.Background()

	require.NoError(t, repo.SyncCatalog(ctx, rbac.Catalog(), rbac.SystemRoles()))

	// SyncCatalog backfills the legacy admin user inserted by insertLegacyUser
	// (seeded in setupRBACTestDB-style). Here we have no legacy admin yet.
	count0, err := repo.CountPlatformAdmins(ctx)
	require.NoError(t, err)

	// Insert a legacy admin and re-sync to trigger backfill.
	insertLegacyUser(t, pool, "pa@iam.test", "admin")
	require.NoError(t, repo.SyncCatalog(ctx, rbac.Catalog(), rbac.SystemRoles()))

	count1, err := repo.CountPlatformAdmins(ctx)
	require.NoError(t, err)
	assert.Equal(t, count0+1, count1)
}

// ── Identity repo: invitation round-trip ─────────────────────────────────────

func TestIAM_InvitationRoundTrip(t *testing.T) {
	pool := setupTestDBWith(t,
		"001_identity.sql",
		"006_reseller.sql",
		"038_rbac.sql",
		"039_invitations.sql",
	)
	idRepo := postgres.NewIdentityRepository(pool)
	ctx := context.Background()

	invitedBy := insertLegacyUser(t, pool, "inviter@iam.test", "admin")
	now := time.Now().UTC().Truncate(time.Microsecond)

	inv, err := aggregate.NewInvitation(
		"newstaff@iam.test",
		rbac.RoleShopStaff,
		nil,  // global invitation (no tenant scope)
		nil,  // no commission
		invitedBy,
		now,
	)
	require.NoError(t, err)

	// CreateInvitation persists without error.
	require.NoError(t, idRepo.CreateInvitation(ctx, inv))

	// GetInvitationByToken retrieves the same record.
	got, err := idRepo.GetInvitationByToken(ctx, inv.Token)
	require.NoError(t, err)
	assert.Equal(t, inv.ID, got.ID)
	assert.Equal(t, inv.Email, got.Email)
	assert.Equal(t, inv.RoleKey, got.RoleKey)
	assert.Nil(t, got.TenantID)

	// GetInvitationByID also works.
	gotByID, err := idRepo.GetInvitationByID(ctx, inv.ID)
	require.NoError(t, err)
	assert.Equal(t, inv.ID, gotByID.ID)

	// DeleteInvitation removes it.
	require.NoError(t, idRepo.DeleteInvitation(ctx, inv.ID))

	// Token lookup now returns not-found.
	_, err = idRepo.GetInvitationByToken(ctx, inv.Token)
	assert.ErrorIs(t, err, identity.ErrNotFound)
}

func TestIAM_InvitationWithTenant(t *testing.T) {
	pool := setupTestDBWith(t,
		"001_identity.sql",
		"006_reseller.sql",
		"038_rbac.sql",
		"039_invitations.sql",
	)
	idRepo := postgres.NewIdentityRepository(pool)
	ctx := context.Background()

	invitedBy := insertLegacyUser(t, pool, "admin2@iam.test", "admin")
	tenantID := uuid.Must(uuid.NewV7()).String()
	rate := 20
	now := time.Now().UTC().Truncate(time.Microsecond)

	inv, err := aggregate.NewInvitation(
		"owner@iam.test",
		rbac.RoleShopOwner,
		&tenantID,
		&rate,
		invitedBy,
		now,
	)
	require.NoError(t, err)

	require.NoError(t, idRepo.CreateInvitation(ctx, inv))

	got, err := idRepo.GetInvitationByToken(ctx, inv.Token)
	require.NoError(t, err)
	assert.Equal(t, rbac.RoleShopOwner, got.RoleKey)
	require.NotNil(t, got.TenantID)
	assert.Equal(t, tenantID, *got.TenantID)
	require.NotNil(t, got.CommissionRate)
	assert.Equal(t, rate, *got.CommissionRate)

	// ListInvitations scoped to this tenant returns it.
	list, err := idRepo.ListInvitations(ctx, []string{tenantID}, false)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, inv.ID, list[0].ID)

	// ListInvitations all=true also returns it.
	all, err := idRepo.ListInvitations(ctx, nil, true)
	require.NoError(t, err)
	var found bool
	for _, li := range all {
		if li.ID == inv.ID {
			found = true
		}
	}
	assert.True(t, found, "invitation must appear in all-invitations list")
}
