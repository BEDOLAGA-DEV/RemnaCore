package resellertest

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent/domaineventtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

// MockPublisher is an alias for the shared domaineventtest.MockPublisher.
type MockPublisher = domaineventtest.MockPublisher

// MockTenantRepository is a testify mock implementation of reseller.TenantRepository.
type MockTenantRepository struct {
	mock.Mock
}

func (m *MockTenantRepository) CreateTenant(ctx context.Context, tenant *reseller.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepository) GetTenantByID(ctx context.Context, id string) (*reseller.Tenant, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.Tenant), args.Error(1)
}

func (m *MockTenantRepository) GetTenantByDomain(ctx context.Context, domain string) (*reseller.Tenant, error) {
	args := m.Called(ctx, domain)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.Tenant), args.Error(1)
}

func (m *MockTenantRepository) GetTenantByAPIKeyHash(ctx context.Context, keyHash string) (*reseller.Tenant, error) {
	args := m.Called(ctx, keyHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.Tenant), args.Error(1)
}

func (m *MockTenantRepository) UpdateTenant(ctx context.Context, tenant *reseller.Tenant) error {
	args := m.Called(ctx, tenant)
	return args.Error(0)
}

func (m *MockTenantRepository) ListTenants(ctx context.Context, limit, offset int) ([]*reseller.Tenant, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*reseller.Tenant), args.Error(1)
}

// Ensure MockTenantRepository satisfies reseller.TenantRepository at compile time.
var _ reseller.TenantRepository = (*MockTenantRepository)(nil)

// MockCommissionRepository is a testify mock implementation of reseller.CommissionRepository.
type MockCommissionRepository struct {
	mock.Mock
}

func (m *MockCommissionRepository) CreateResellerAccount(ctx context.Context, account *reseller.ResellerAccount) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockCommissionRepository) GetResellerAccountByID(ctx context.Context, id string) (*reseller.ResellerAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.ResellerAccount), args.Error(1)
}

func (m *MockCommissionRepository) GetResellerAccountByUserAndTenant(ctx context.Context, userID, tenantID string) (*reseller.ResellerAccount, error) {
	args := m.Called(ctx, userID, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.ResellerAccount), args.Error(1)
}

func (m *MockCommissionRepository) CreateCommission(ctx context.Context, commission *reseller.Commission) error {
	args := m.Called(ctx, commission)
	return args.Error(0)
}

func (m *MockCommissionRepository) GetPendingCommissions(ctx context.Context, resellerID string) ([]*reseller.Commission, error) {
	args := m.Called(ctx, resellerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*reseller.Commission), args.Error(1)
}

func (m *MockCommissionRepository) UpdateCommission(ctx context.Context, commission *reseller.Commission) error {
	args := m.Called(ctx, commission)
	return args.Error(0)
}

func (m *MockCommissionRepository) UpdateResellerBalance(ctx context.Context, resellerID string, balance int64) error {
	args := m.Called(ctx, resellerID, balance)
	return args.Error(0)
}

// Ensure MockCommissionRepository satisfies reseller.CommissionRepository at compile time.
var _ reseller.CommissionRepository = (*MockCommissionRepository)(nil)

// NoopTxRunner is an alias for the shared txmanagertest.NoopTxRunner.
type NoopTxRunner = txmanagertest.NoopTxRunner
