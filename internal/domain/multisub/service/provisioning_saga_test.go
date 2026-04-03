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

func TestProvision_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	calc := service.NewBindingCalculator()

	saga := service.NewProvisioningSaga(repo, gw, pub, calc)
	plan := newPlanSnapshotForSaga()

	// Expect 3 bindings: base + gaming + 1 family member
	repo.On("Create", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Times(3)
	repo.On("Update", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Times(3)

	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return req.TrafficStrategy == "MONTH" && req.Tag == "PLATFORM"
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()
	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return req.TrafficStrategy == "MONTH"
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-2",
		ShortUUID: "rw-short-2",
	}, nil).Once()
	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return req.TrafficStrategy == "MONTH"
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-3",
		ShortUUID: "rw-short-3",
	}, nil).Once()

	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil).Times(3)

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

	saga := service.NewProvisioningSaga(repo, gw, pub, calc)
	plan := newPlanSnapshotForSaga()

	// First binding: base - succeeds fully
	repo.On("Create", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Times(2)

	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.Status == aggregate.BindingActive
	})).Return(nil).Once()

	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil).Once()

	// Second binding: gaming - Remnawave fails
	rwErr := errors.New("connection refused")
	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(nil, rwErr).Once()

	// Compensation: mark second binding as failed
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
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

	saga := service.NewProvisioningSaga(repo, gw, pub, calc)
	plan := newPlanSnapshotForSaga()

	// Create binding in DB succeeds
	repo.On("Create", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Once()

	// Create user in Remnawave succeeds
	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	// DB update fails
	dbErr := errors.New("database connection lost")
	repo.On("Update", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(dbErr).Once()

	// COMPENSATION: Remnawave user must be deleted (succeeds on first attempt)
	gw.On("DeleteUser", ctx, "rw-uuid-1").Return(nil).Once()

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

	saga := service.NewProvisioningSaga(repo, gw, pub, calc)
	plan := newPlanSnapshotForSaga()

	// Create binding in DB succeeds
	repo.On("Create", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(nil).Once()

	// Create user in Remnawave succeeds
	gw.On("CreateUser", ctx, mock.MatchedBy(func(req multisub.CreateRemnawaveUserRequest) bool {
		return true
	})).Return(&multisub.RemnawaveUserResult{
		UUID:      "rw-uuid-1",
		ShortUUID: "rw-short-1",
	}, nil).Once()

	// DB update fails
	dbErr := errors.New("database connection lost")
	repo.On("Update", ctx, mock.AnythingOfType("*aggregate.RemnawaveBinding")).Return(dbErr).Once()

	// COMPENSATION: first delete fails, cancel context to abort remaining retries
	deleteErr := errors.New("remnawave unavailable")
	gw.On("DeleteUser", ctx, "rw-uuid-1").Return(deleteErr).Once().Run(func(_ mock.Arguments) {
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
	gw.AssertCalled(t, "DeleteUser", ctx, "rw-uuid-1")
	repo.AssertExpectations(t)
}
