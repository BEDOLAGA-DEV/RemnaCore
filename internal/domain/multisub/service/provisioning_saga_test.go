package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func newPlanSnapshotForSaga() multisub.PlanSnapshot {
	return multisub.PlanSnapshot{
		ID:                   "plan-premium",
		TrafficLimitBytes:    100_000_000_000,
		MaxRemnawaveBindings: 4,
		Addons: []multisub.AddonSnapshot{
			{
				ID:                "addon-gaming",
				Name:              "gaming",
				Type:              multisub.AddonSnapshotNodes,
				ExtraTrafficBytes: 50_000_000_000,
				ExtraNodes:        []string{"node-gaming-us"},
			},
		},
	}
}

// newPermissiveSagaRepo creates a MockSagaRepo that accepts all calls
// without assertion, for tests that focus on provisioning behavior rather
// than saga persistence.
func newPermissiveSagaRepo() *multisubtest.MockSagaRepo {
	sagaRepo := new(multisubtest.MockSagaRepo)
	sagaRepo.On("Create", mock.Anything, mock.Anything).Return(&multisub.SagaInstance{ID: "saga-1"}, nil)
	sagaRepo.On("UpdateProgress", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	sagaRepo.On("Complete", mock.Anything, mock.Anything).Return(nil)
	sagaRepo.On("Fail", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return sagaRepo
}

func TestProvision_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())
	plan := newPlanSnapshotForSaga()

	// Expect 3 bindings: base + gaming + 1 family member
	repo.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Times(3)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Times(3)

	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return req.TrafficStrategy == "MONTH" && req.Tag == "PLATFORM"
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()
	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return req.TrafficStrategy == "MONTH"
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-2",
		ShortUUID: "rw-short-2",
	}, nil).Once()
	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return req.TrafficStrategy == "MONTH"
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-3",
		ShortUUID: "rw-short-3",
	}, nil).Once()

	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil).Times(3)

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID:  "sub-1",
		PlatformUserID:  "user-abc12345xyz",
		Plan:            plan,
		AddonIDs:        []string{"addon-gaming"},
		FamilyMemberIDs: []string{"family-1"},
	})

	require.NoError(t, err)
	require.Len(t, results, 3)

	assert.Equal(t, aggregate.PurposeBase, results[0].Purpose)
	assert.Equal(t, "rw-uuid-1", results[0].RemnawaveUUID)

	assert.Equal(t, aggregate.PurposeGaming, results[1].Purpose)
	assert.Equal(t, "rw-uuid-2", results[1].RemnawaveUUID)

	assert.Equal(t, aggregate.PurposeFamilyMember, results[2].Purpose)
	assert.Equal(t, "rw-uuid-3", results[2].RemnawaveUUID)

	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestProvision_RemnawaveFail(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())
	plan := newPlanSnapshotForSaga()

	// First binding: base - succeeds fully
	repo.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Times(2)

	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.Status == aggregate.BindingActive
	})).Return(nil).Once()

	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil).Once()

	// Second binding: gaming - Remnawave fails
	rwErr := errors.New("connection refused")
	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(nil, rwErr).Once()

	// Compensation: mark second binding as failed
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.Status == aggregate.BindingFailed
	})).Return(nil).Once()

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID: "sub-1",
		PlatformUserID: "user-abc12345xyz",
		Plan:           plan,
		AddonIDs:       []string{"addon-gaming"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "remnawave create user")

	// First binding was successful
	require.Len(t, results, 1)
	assert.Equal(t, aggregate.PurposeBase, results[0].Purpose)
	assert.Equal(t, "rw-uuid-1", results[0].RemnawaveUUID)

	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestProvision_CompensationOnDBFail(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())
	plan := newPlanSnapshotForSaga()

	// Create binding in DB succeeds
	repo.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Once()

	// Create user in Remnawave succeeds
	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	// DB update fails
	dbErr := errors.New("database connection lost")
	repo.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(dbErr).Once()

	// COMPENSATION: Remnawave user must be deleted (succeeds on first attempt)
	gw.On("DeleteUser", mock.Anything, "rw-uuid-1").Return(nil).Once()

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID: "sub-1",
		PlatformUserID: "user-abc12345xyz",
		Plan:           plan,
		AddonIDs:       nil,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "update binding")
	assert.Empty(t, results)

	gw.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestProvision_CompensationRetryOnDeleteFail(t *testing.T) {
	// Use a context with short timeout so backoff sleeps are cancelled quickly.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())
	plan := newPlanSnapshotForSaga()

	// Create binding in DB succeeds
	repo.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Once()

	// Create user in Remnawave succeeds
	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	// DB update fails
	dbErr := errors.New("database connection lost")
	repo.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(dbErr).Once()

	// COMPENSATION: first delete fails, cancel context to abort remaining retries
	deleteErr := errors.New("remnawave unavailable")
	gw.On("DeleteUser", mock.Anything, "rw-uuid-1").Return(deleteErr).Once().Run(func(_ mock.Arguments) {
		// Cancel context after first failed attempt to avoid waiting for backoff.
		cancel()
	})

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID: "sub-1",
		PlatformUserID: "user-abc12345xyz",
		Plan:           plan,
		AddonIDs:       nil,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "update binding")
	assert.Empty(t, results)

	// DeleteUser was called at least once (context cancellation prevents further retries)
	gw.AssertCalled(t, "DeleteUser", mock.Anything, "rw-uuid-1")
	repo.AssertExpectations(t)
}

func TestProvision_MaxBindingsExceeded(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())

	// Plan allows only 2 bindings, but base + gaming + 2 family = 4 specs
	plan := multisub.PlanSnapshot{
		ID:                   "plan-limited",
		TrafficLimitBytes:    100_000_000_000,
		MaxRemnawaveBindings: 2,
		Addons: []multisub.AddonSnapshot{
			{
				ID:                "addon-gaming",
				Name:              "gaming",
				Type:              multisub.AddonSnapshotNodes,
				ExtraTrafficBytes: 50_000_000_000,
				ExtraNodes:        []string{"node-gaming-us"},
			},
		},
	}

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID:  "sub-1",
		PlatformUserID:  "user-abc12345xyz",
		Plan:            plan,
		AddonIDs:        []string{"addon-gaming"},
		FamilyMemberIDs: []string{"family-1", "family-2"},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, multisub.ErrMaxBindingsExceeded)
	assert.Nil(t, results)

	// No repo/gateway calls should have been made
	repo.AssertNotCalled(t, "Create")
	gw.AssertNotCalled(t, "CreateUser")
}

func TestProvision_ZeroMaxBindings_NoLimit(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())

	// MaxRemnawaveBindings=0 means no limit enforced
	plan := multisub.PlanSnapshot{
		ID:                   "plan-unlimited",
		TrafficLimitBytes:    100_000_000_000,
		MaxRemnawaveBindings: 0,
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Once()
	repo.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Once()

	gw.On("CreateUser", mock.Anything, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil).Once()

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID: "sub-1",
		PlatformUserID: "user-abc12345xyz",
		Plan:           plan,
		AddonIDs:       nil,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, aggregate.PurposeBase, results[0].Purpose)

	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestProvision_SagaPersistence(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()
	sagaRepo := new(multisubtest.MockSagaRepo)

	saga := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clock.NewReal())

	plan := multisub.PlanSnapshot{
		ID:                   "plan-basic",
		TrafficLimitBytes:    100_000_000_000,
		MaxRemnawaveBindings: 0,
	}

	// Saga repo expectations: create, checkpoint progress, complete
	sagaRepo.On("Create", mock.Anything, mock.MatchedBy(func(s *multisub.SagaInstance) bool {
		return s.SagaType == multisub.SagaTypeProvisioning &&
			s.CorrelationID == "sub-saga" &&
			s.Status == multisub.SagaStatusRunning
	})).Return(&multisub.SagaInstance{ID: "saga-test-1"}, nil)
	sagaRepo.On("UpdateProgress", mock.Anything, "saga-test-1", 1, mock.Anything).Return(nil)
	sagaRepo.On("Complete", mock.Anything, "saga-test-1").Return(nil)

	repo.On("Create", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil)
	gw.On("CreateUser", mock.Anything, mock.Anything).Return(&multisub.RemnawaveUserResult{
		UUID: "rw-1", ShortUUID: "rw-s-1",
	}, nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	results, err := saga.Provision(ctx, service.ProvisionRequest{
		SubscriptionID: "sub-saga",
		PlatformUserID: "user-abc12345xyz",
		Plan:           plan,
	})

	require.NoError(t, err)
	require.Len(t, results, 1)

	sagaRepo.AssertExpectations(t)
	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
}
