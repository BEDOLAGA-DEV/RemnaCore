// Package billingtest provides mock implementations of billing domain interfaces
// for use in unit tests. All mocks use testify/mock.
package billingtest

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent/domaineventtest"
)

// MockEventPublisher is an alias for the shared domaineventtest.MockPublisher
// so that existing test code referencing billingtest.MockEventPublisher
// continues to compile.
type MockEventPublisher = domaineventtest.MockPublisher

// --- MockPlanRepo ---

// MockPlanRepo is a testify/mock implementation of billing.PlanRepository.
type MockPlanRepo struct {
	mock.Mock
}

func (m *MockPlanRepo) GetByID(ctx context.Context, id string) (*aggregate.Plan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.Plan), args.Error(1)
}

func (m *MockPlanRepo) GetAll(ctx context.Context) ([]*aggregate.Plan, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Plan), args.Error(1)
}

func (m *MockPlanRepo) GetActive(ctx context.Context) ([]*aggregate.Plan, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Plan), args.Error(1)
}

func (m *MockPlanRepo) Create(ctx context.Context, plan *aggregate.Plan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

func (m *MockPlanRepo) Update(ctx context.Context, plan *aggregate.Plan) error {
	args := m.Called(ctx, plan)
	return args.Error(0)
}

// --- MockSubscriptionRepo ---

// MockSubscriptionRepo is a testify/mock implementation of billing.SubscriptionRepository.
type MockSubscriptionRepo struct {
	mock.Mock
}

func (m *MockSubscriptionRepo) GetByID(ctx context.Context, id string) (*aggregate.Subscription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepo) GetByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepo) GetActiveByUserID(ctx context.Context, userID string) ([]*aggregate.Subscription, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepo) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Subscription, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Subscription), args.Error(1)
}

func (m *MockSubscriptionRepo) Create(ctx context.Context, sub *aggregate.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockSubscriptionRepo) Update(ctx context.Context, sub *aggregate.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

// --- MockInvoiceRepo ---

// MockInvoiceRepo is a testify/mock implementation of billing.InvoiceRepository.
type MockInvoiceRepo struct {
	mock.Mock
}

func (m *MockInvoiceRepo) GetByID(ctx context.Context, id string) (*aggregate.Invoice, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.Invoice), args.Error(1)
}

func (m *MockInvoiceRepo) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.Invoice, error) {
	args := m.Called(ctx, subID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Invoice), args.Error(1)
}

func (m *MockInvoiceRepo) GetPendingByUserID(ctx context.Context, userID string) ([]*aggregate.Invoice, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Invoice), args.Error(1)
}

func (m *MockInvoiceRepo) GetAll(ctx context.Context, limit, offset int) ([]*aggregate.Invoice, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.Invoice), args.Error(1)
}

func (m *MockInvoiceRepo) Create(ctx context.Context, inv *aggregate.Invoice) error {
	args := m.Called(ctx, inv)
	return args.Error(0)
}

func (m *MockInvoiceRepo) Update(ctx context.Context, inv *aggregate.Invoice) error {
	args := m.Called(ctx, inv)
	return args.Error(0)
}

// --- MockFamilyRepo ---

// MockFamilyRepo is a testify/mock implementation of billing.FamilyRepository.
type MockFamilyRepo struct {
	mock.Mock
}

func (m *MockFamilyRepo) GetByID(ctx context.Context, id string) (*aggregate.FamilyGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.FamilyGroup), args.Error(1)
}

func (m *MockFamilyRepo) GetByOwnerID(ctx context.Context, ownerID string) (*aggregate.FamilyGroup, error) {
	args := m.Called(ctx, ownerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.FamilyGroup), args.Error(1)
}

func (m *MockFamilyRepo) Create(ctx context.Context, fg *aggregate.FamilyGroup) error {
	args := m.Called(ctx, fg)
	return args.Error(0)
}

func (m *MockFamilyRepo) Update(ctx context.Context, fg *aggregate.FamilyGroup) error {
	args := m.Called(ctx, fg)
	return args.Error(0)
}

func (m *MockFamilyRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// --- MockPaymentGateway ---

// MockPaymentGateway is a testify/mock implementation of billing.PaymentGateway.
// This allows billing tests to mock the payment ACL boundary without importing
// the payment domain.
type MockPaymentGateway struct {
	mock.Mock
}

func (m *MockPaymentGateway) CreateCharge(ctx context.Context, req billing.CreateChargeRequest) (*billing.CreateChargeResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*billing.CreateChargeResult), args.Error(1)
}

// --- MockDomainRateLimiter ---

// MockDomainRateLimiter is a testify/mock implementation of billing.DomainRateLimiter.
type MockDomainRateLimiter struct {
	mock.Mock
}

func (m *MockDomainRateLimiter) AllowCheckout(ctx context.Context, userID string) (bool, error) {
	args := m.Called(ctx, userID)
	return args.Bool(0), args.Error(1)
}

// --- NoopTxRunner ---

// NoopTxRunner implements txmanager.Runner by executing fn directly without a
// real database transaction. This is suitable for unit tests where repositories
// and publishers are mocked.
type NoopTxRunner struct{}

// RunInTx executes fn with the original context, providing pass-through
// semantics for tests.
func (NoopTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
