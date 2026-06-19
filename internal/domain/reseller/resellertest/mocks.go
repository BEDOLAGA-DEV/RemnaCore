package resellertest

import (
	"context"
	"time"

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

func (m *MockTenantRepository) GetTenantByIDForUpdate(ctx context.Context, id string) (*reseller.Tenant, error) {
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

func (m *MockTenantRepository) SetTenantOwnerUserID(ctx context.Context, tenantID, userID string, now time.Time) error {
	args := m.Called(ctx, tenantID, userID, now)
	return args.Error(0)
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

func (m *MockCommissionRepository) GetCommissionByID(ctx context.Context, id string) (*reseller.Commission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.Commission), args.Error(1)
}

func (m *MockCommissionRepository) GetCommissionByIDForUpdate(ctx context.Context, id string) (*reseller.Commission, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*reseller.Commission), args.Error(1)
}

func (m *MockCommissionRepository) GetPendingCommissions(ctx context.Context, resellerID string) ([]*reseller.Commission, error) {
	args := m.Called(ctx, resellerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*reseller.Commission), args.Error(1)
}

func (m *MockCommissionRepository) ListCommissionsByTenant(ctx context.Context, tenantID string) ([]*reseller.Commission, error) {
	args := m.Called(ctx, tenantID)
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

// MockCustomerRepository is a testify mock for reseller.CustomerRepository.
type MockCustomerRepository struct {
	mock.Mock
}

func (m *MockCustomerRepository) ListCustomersByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*reseller.CustomerSummary, error) {
	args := m.Called(ctx, tenantID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*reseller.CustomerSummary), args.Error(1)
}

func (m *MockCustomerRepository) CountActiveCustomersByTenant(ctx context.Context, tenantID string) (int, error) {
	args := m.Called(ctx, tenantID)
	return args.Int(0), args.Error(1)
}

func (m *MockCustomerRepository) CountActiveSubscriptionsByTenant(ctx context.Context, tenantID string) (int, error) {
	args := m.Called(ctx, tenantID)
	return args.Int(0), args.Error(1)
}

func (m *MockCustomerRepository) SumPendingCommissionByTenant(ctx context.Context, tenantID string) (int64, string, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).(int64), args.String(1), args.Error(2)
}

// Ensure MockCustomerRepository satisfies reseller.CustomerRepository at compile time.
var _ reseller.CustomerRepository = (*MockCustomerRepository)(nil)
