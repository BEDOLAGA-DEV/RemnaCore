package reseller_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller/resellertest"
)

func newTestService(t *testing.T) (
	*reseller.ResellerService,
	*resellertest.MockTenantRepository,
	*resellertest.MockCommissionRepository,
	*resellertest.MockPublisher,
) {
	t.Helper()

	tenantRepo := new(resellertest.MockTenantRepository)
	commissionRepo := new(resellertest.MockCommissionRepository)
	pub := new(resellertest.MockPublisher)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	svc := reseller.NewResellerService(tenantRepo, commissionRepo, pub, logger)
	return svc, tenantRepo, commissionRepo, pub
}

func TestCreateTenant_Success(t *testing.T) {
	svc, tenantRepo, _, pub := newTestService(t)
	ctx := context.Background()

	tenantRepo.On("CreateTenant", ctx, mock.AnythingOfType("*reseller.Tenant")).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	tenant, plainKey, err := svc.CreateTenant(ctx, "Acme VPN", "acme.vpn.com", "owner-1")
	require.NoError(t, err)

	assert.NotEmpty(t, tenant.ID)
	assert.Equal(t, "Acme VPN", tenant.Name)
	assert.Equal(t, "acme.vpn.com", tenant.Domain)
	assert.Equal(t, "owner-1", tenant.OwnerUserID)
	assert.True(t, tenant.IsActive)
	assert.NotEmpty(t, plainKey)
	assert.Len(t, plainKey, reseller.APIKeyLen*2)

	// Verify the stored hash matches the plain key.
	assert.Equal(t, reseller.HashAPIKey(plainKey), tenant.APIKeyHash)

	tenantRepo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestGetTenant_Success(t *testing.T) {
	svc, tenantRepo, _, _ := newTestService(t)
	ctx := context.Background()

	expected := reseller.NewTenant("Acme VPN", "acme.vpn.com", "owner-1")
	tenantRepo.On("GetTenantByID", ctx, expected.ID).Return(expected, nil)

	tenant, err := svc.GetTenant(ctx, expected.ID)
	require.NoError(t, err)
	assert.Equal(t, expected.ID, tenant.ID)

	tenantRepo.AssertExpectations(t)
}

func TestGetTenant_NotFound(t *testing.T) {
	svc, tenantRepo, _, _ := newTestService(t)
	ctx := context.Background()

	tenantRepo.On("GetTenantByID", ctx, "missing-id").Return(nil, reseller.ErrTenantNotFound)

	_, err := svc.GetTenant(ctx, "missing-id")
	require.Error(t, err)

	tenantRepo.AssertExpectations(t)
}

func TestCreateResellerAccount_Success(t *testing.T) {
	svc, _, commissionRepo, pub := newTestService(t)
	ctx := context.Background()

	commissionRepo.On("CreateResellerAccount", ctx, mock.AnythingOfType("*reseller.ResellerAccount")).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	account, err := svc.CreateResellerAccount(ctx, "tenant-1", "user-1", 25)
	require.NoError(t, err)

	assert.NotEmpty(t, account.ID)
	assert.Equal(t, "tenant-1", account.TenantID)
	assert.Equal(t, "user-1", account.UserID)
	assert.Equal(t, 25, account.CommissionRate)
	assert.Equal(t, int64(0), account.Balance)

	commissionRepo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestCreateResellerAccount_InvalidRate(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.CreateResellerAccount(ctx, "tenant-1", "user-1", 150)
	require.Error(t, err)
	assert.ErrorIs(t, err, reseller.ErrInvalidCommissionRate)
}

func TestRecordCommission_Success(t *testing.T) {
	svc, _, commissionRepo, pub := newTestService(t)
	ctx := context.Background()

	account := &reseller.ResellerAccount{
		ID:       "reseller-1",
		Balance:  5000,
		TenantID: "tenant-1",
		UserID:   "user-1",
	}

	commissionRepo.On("CreateCommission", ctx, mock.AnythingOfType("*reseller.Commission")).Return(nil)
	commissionRepo.On("GetResellerAccountByID", ctx, "reseller-1").Return(account, nil)
	commissionRepo.On("UpdateResellerBalance", ctx, "reseller-1", int64(6500)).Return(nil) // 5000 + 1500
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	commission, err := svc.RecordCommission(ctx, "reseller-1", "sale-1", 10000, 15, "USD")
	require.NoError(t, err)

	assert.NotEmpty(t, commission.ID)
	assert.Equal(t, "reseller-1", commission.ResellerID)
	assert.Equal(t, int64(1500), commission.Amount) // 15% of 10000
	assert.Equal(t, reseller.CommissionPending, commission.Status)

	commissionRepo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestValidateAPIKey_Success(t *testing.T) {
	svc, tenantRepo, _, _ := newTestService(t)
	ctx := context.Background()

	tenant := reseller.NewTenant("Acme VPN", "acme.vpn.com", "owner-1")
	plainKey, _ := tenant.GenerateAPIKey()
	expectedHash := reseller.HashAPIKey(plainKey)

	tenantRepo.On("GetTenantByAPIKeyHash", ctx, expectedHash).Return(tenant, nil)

	result, err := svc.ValidateAPIKey(ctx, plainKey)
	require.NoError(t, err)
	assert.Equal(t, tenant.ID, result.ID)

	tenantRepo.AssertExpectations(t)
}

func TestValidateAPIKey_InvalidKey(t *testing.T) {
	svc, tenantRepo, _, _ := newTestService(t)
	ctx := context.Background()

	tenantRepo.On("GetTenantByAPIKeyHash", ctx, mock.AnythingOfType("string")).Return(nil, reseller.ErrTenantNotFound)

	_, err := svc.ValidateAPIKey(ctx, "invalid-key")
	require.Error(t, err)
	assert.ErrorIs(t, err, reseller.ErrInvalidAPIKey)

	tenantRepo.AssertExpectations(t)
}

func TestValidateAPIKey_InactiveTenant(t *testing.T) {
	svc, tenantRepo, _, _ := newTestService(t)
	ctx := context.Background()

	tenant := reseller.NewTenant("Acme VPN", "acme.vpn.com", "owner-1")
	tenant.IsActive = false
	plainKey, _ := tenant.GenerateAPIKey()
	expectedHash := reseller.HashAPIKey(plainKey)

	tenantRepo.On("GetTenantByAPIKeyHash", ctx, expectedHash).Return(tenant, nil)

	_, err := svc.ValidateAPIKey(ctx, plainKey)
	require.Error(t, err)
	assert.ErrorIs(t, err, reseller.ErrTenantInactive)

	tenantRepo.AssertExpectations(t)
}

func TestUpdateBranding_Success(t *testing.T) {
	svc, tenantRepo, _, _ := newTestService(t)
	ctx := context.Background()

	tenant := reseller.NewTenant("Acme VPN", "acme.vpn.com", "owner-1")
	tenantRepo.On("GetTenantByID", ctx, tenant.ID).Return(tenant, nil)
	tenantRepo.On("UpdateTenant", ctx, mock.AnythingOfType("*reseller.Tenant")).Return(nil)

	branding := reseller.BrandingConfig{
		Logo:         "https://acme.com/logo.png",
		PrimaryColor: "#FF5500",
		AppName:      "Acme VPN Pro",
		SupportEmail: "support@acme.com",
		SupportURL:   "https://acme.com/support",
	}

	updated, err := svc.UpdateBranding(ctx, tenant.ID, branding)
	require.NoError(t, err)
	assert.Equal(t, branding, updated.BrandingConfig)

	tenantRepo.AssertExpectations(t)
}

func TestGetPendingCommissions_Success(t *testing.T) {
	svc, _, commissionRepo, _ := newTestService(t)
	ctx := context.Background()

	expected := []*reseller.Commission{
		{ID: "c1", ResellerID: "reseller-1", Amount: 1000, Status: reseller.CommissionPending},
		{ID: "c2", ResellerID: "reseller-1", Amount: 2000, Status: reseller.CommissionPending},
	}

	commissionRepo.On("GetPendingCommissions", ctx, "reseller-1").Return(expected, nil)

	result, err := svc.GetPendingCommissions(ctx, "reseller-1")
	require.NoError(t, err)
	assert.Len(t, result, 2)

	commissionRepo.AssertExpectations(t)
}
