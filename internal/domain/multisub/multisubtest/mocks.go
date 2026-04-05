// Package multisubtest provides mock implementations of multisub domain
// interfaces for use in unit tests. All mocks use testify/mock.
package multisubtest

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent/domaineventtest"
)

// MockEventPublisher is an alias for the shared domaineventtest.MockPublisher.
type MockEventPublisher = domaineventtest.MockPublisher

// --- MockBindingRepo ---

// MockBindingRepo is a testify/mock implementation of multisub.BindingRepository.
type MockBindingRepo struct {
	mock.Mock
}

func (m *MockBindingRepo) GetByID(ctx context.Context, id string) (*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) GetBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx, subID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) GetByPlatformUserID(ctx context.Context, userID string) ([]*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) GetByRemnawaveUUID(ctx context.Context, rwUUID string) (*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx, rwUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) GetActiveBySubscriptionID(ctx context.Context, subID string) ([]*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx, subID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) GetAllActive(ctx context.Context) ([]*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) GetFailedWithRemnawaveUUID(ctx context.Context) ([]*aggregate.RemnawaveBinding, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*aggregate.RemnawaveBinding), args.Error(1)
}

func (m *MockBindingRepo) Create(ctx context.Context, binding *aggregate.RemnawaveBinding) error {
	args := m.Called(ctx, binding)
	return args.Error(0)
}

func (m *MockBindingRepo) Update(ctx context.Context, binding *aggregate.RemnawaveBinding) error {
	args := m.Called(ctx, binding)
	return args.Error(0)
}

func (m *MockBindingRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// --- MockRemnawaveGateway ---

// MockRemnawaveGateway is a testify/mock implementation of multisub.RemnawaveGateway.
type MockRemnawaveGateway struct {
	mock.Mock
}

func (m *MockRemnawaveGateway) CreateUser(ctx context.Context, req multisub.CreateRemnawaveUserRequest) (*multisub.RemnawaveUserResult, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*multisub.RemnawaveUserResult), args.Error(1)
}

func (m *MockRemnawaveGateway) GetUser(ctx context.Context, remnawaveUUID string) (*multisub.RemnawaveUserStatus, error) {
	args := m.Called(ctx, remnawaveUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*multisub.RemnawaveUserStatus), args.Error(1)
}

func (m *MockRemnawaveGateway) DeleteUser(ctx context.Context, remnawaveUUID string) error {
	args := m.Called(ctx, remnawaveUUID)
	return args.Error(0)
}

func (m *MockRemnawaveGateway) EnableUser(ctx context.Context, remnawaveUUID string) error {
	args := m.Called(ctx, remnawaveUUID)
	return args.Error(0)
}

func (m *MockRemnawaveGateway) DisableUser(ctx context.Context, remnawaveUUID string) error {
	args := m.Called(ctx, remnawaveUUID)
	return args.Error(0)
}

func (m *MockRemnawaveGateway) AssignToSquad(ctx context.Context, remnawaveUUID, squadUUID string) error {
	args := m.Called(ctx, remnawaveUUID, squadUUID)
	return args.Error(0)
}

// --- MockSagaRepo ---

// MockSagaRepo is a testify/mock implementation of multisub.SagaRepository.
type MockSagaRepo struct {
	mock.Mock
}

func (m *MockSagaRepo) Create(ctx context.Context, saga *multisub.SagaInstance) (*multisub.SagaInstance, error) {
	args := m.Called(ctx, saga)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*multisub.SagaInstance), args.Error(1)
}

func (m *MockSagaRepo) UpdateProgress(ctx context.Context, id string, step int, stateData []byte) error {
	args := m.Called(ctx, id, step, stateData)
	return args.Error(0)
}

func (m *MockSagaRepo) Complete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSagaRepo) Fail(ctx context.Context, id string, errMsg string) error {
	args := m.Called(ctx, id, errMsg)
	return args.Error(0)
}

func (m *MockSagaRepo) GetRunning(ctx context.Context) ([]*multisub.SagaInstance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*multisub.SagaInstance), args.Error(1)
}

func (m *MockSagaRepo) GetByCorrelation(ctx context.Context, sagaType multisub.SagaType, correlationID string) (*multisub.SagaInstance, error) {
	args := m.Called(ctx, sagaType, correlationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*multisub.SagaInstance), args.Error(1)
}

func (m *MockSagaRepo) Cleanup(ctx context.Context, olderThan time.Duration) error {
	args := m.Called(ctx, olderThan)
	return args.Error(0)
}

