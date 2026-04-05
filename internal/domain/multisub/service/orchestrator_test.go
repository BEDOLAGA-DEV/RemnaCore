package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// testLogger returns a slog.Logger that suppresses all output below error
// level, keeping test runs quiet.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newOrchestrator creates a MultiSubOrchestrator wired with the given mocks.
// The provisioning and deprovisioning sagas are real instances backed by the
// same mock repo/gateway/publisher so that we can verify end-to-end behavior.
func newOrchestrator(
	repo *multisubtest.MockBindingRepo,
	gw *multisubtest.MockRemnawaveGateway,
	pub *multisubtest.MockEventPublisher,
) *service.MultiSubOrchestrator {
	clk := clock.NewReal()
	calc := service.NewBindingCalculator()
	sagaRepo := newPermissiveSagaRepo()
	provisioning := service.NewProvisioningSaga(repo, gw, pub, calc, sagaRepo, clk)
	deprovisioning := service.NewDeprovisioningSaga(repo, gw, pub, sagaRepo, clk)
	syncSaga := service.NewSyncSaga(repo, gw, pub, sagaRepo, clk)
	syncService := service.NewSyncService(repo, syncSaga, pub)

	return service.NewMultiSubOrchestrator(
		provisioning,
		deprovisioning,
		syncService,
		repo,
		gw,
		pub,
		testLogger(),
		clk,
	)
}

// newPlanSnapshotForOrchestrator returns a PlanSnapshot with a gaming addon
// suitable for orchestrator tests.
func newPlanSnapshotForOrchestrator() multisub.PlanSnapshot {
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

// disabledBinding returns a binding in the disabled state for use in resume tests.
func disabledBinding(id, subID, rwUUID string, purpose aggregate.BindingPurpose) *aggregate.RemnawaveBinding {
	now := time.Now()
	return &aggregate.RemnawaveBinding{
		ID:             id,
		SubscriptionID: subID,
		PlatformUserID: "user-1",
		RemnawaveUUID:  rwUUID,
		Purpose:        purpose,
		Status:         aggregate.BindingDisabled,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// --- OnSubscriptionActivated ---

func TestOnSubscriptionActivated_Idempotent(t *testing.T) {
	tests := []struct {
		name     string
		existing []*aggregate.RemnawaveBinding
	}{
		{
			name: "single existing binding",
			existing: []*aggregate.RemnawaveBinding{
				activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
			},
		},
		{
			name: "multiple existing bindings",
			existing: []*aggregate.RemnawaveBinding{
				activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
				activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := new(multisubtest.MockBindingRepo)
			gw := new(multisubtest.MockRemnawaveGateway)
			pub := new(multisubtest.MockEventPublisher)

			orch := newOrchestrator(repo, gw, pub)

			repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
				Return(tt.existing, nil)

			plan := newPlanSnapshotForOrchestrator()

			err := orch.OnSubscriptionActivated(ctx, "sub-1", "user-1", plan, nil, nil)

			require.NoError(t, err)

			// Provisioning must NOT be called — no Create, no CreateUser
			gw.AssertNotCalled(t, "CreateUser")
			repo.AssertNotCalled(t, "Create")
			repo.AssertExpectations(t)
		})
	}
}

func TestOnSubscriptionActivated_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return(nil, errors.New("db connection lost"))

	plan := newPlanSnapshotForOrchestrator()

	err := orch.OnSubscriptionActivated(ctx, "sub-1", "user-1", plan, nil, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check existing bindings")
	gw.AssertNotCalled(t, "CreateUser")
}

// --- OnSubscriptionCancelled ---

func TestOnSubscriptionCancelled_Idempotent(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return([]*aggregate.RemnawaveBinding{}, nil)

	err := orch.OnSubscriptionCancelled(ctx, "sub-1")

	require.NoError(t, err)
	gw.AssertNotCalled(t, "DeleteUser")
	repo.AssertExpectations(t)
}

func TestOnSubscriptionCancelled_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return(nil, errors.New("db connection lost"))

	err := orch.OnSubscriptionCancelled(ctx, "sub-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "check existing bindings")
	gw.AssertNotCalled(t, "DeleteUser")
}

// --- OnSubscriptionPaused ---

func TestOnSubscriptionPaused_Idempotent(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return([]*aggregate.RemnawaveBinding{}, nil)

	err := orch.OnSubscriptionPaused(ctx, "sub-1")

	require.NoError(t, err)
	gw.AssertNotCalled(t, "DisableUser")
	repo.AssertNotCalled(t, "Update")
}

func TestOnSubscriptionPaused_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return(nil, errors.New("db connection lost"))

	err := orch.OnSubscriptionPaused(ctx, "sub-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get active bindings")
	gw.AssertNotCalled(t, "DisableUser")
}

// --- OnSubscriptionResumed ---

func TestOnSubscriptionResumed_Idempotent(t *testing.T) {
	tests := []struct {
		name     string
		bindings []*aggregate.RemnawaveBinding
	}{
		{
			name:     "no bindings at all",
			bindings: []*aggregate.RemnawaveBinding{},
		},
		{
			name: "all bindings already active",
			bindings: []*aggregate.RemnawaveBinding{
				activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
				activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := new(multisubtest.MockBindingRepo)
			gw := new(multisubtest.MockRemnawaveGateway)
			pub := new(multisubtest.MockEventPublisher)

			orch := newOrchestrator(repo, gw, pub)

			repo.On("GetBySubscriptionID", mock.Anything, "sub-1").
				Return(tt.bindings, nil)

			err := orch.OnSubscriptionResumed(ctx, "sub-1")

			require.NoError(t, err)
			gw.AssertNotCalled(t, "EnableUser")
			repo.AssertNotCalled(t, "Update")
		})
	}
}

func TestOnSubscriptionResumed_EnablesDisabledBindings(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		disabledBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
	}

	repo.On("GetBySubscriptionID", mock.Anything, "sub-1").
		Return(bindings, nil)

	gw.On("EnableUser", mock.Anything, "rw-2").Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-2" && b.Status == aggregate.BindingActive
	})).Return(nil).Once()

	err := orch.OnSubscriptionResumed(ctx, "sub-1")

	require.NoError(t, err)
	gw.AssertExpectations(t)
	repo.AssertExpectations(t)
	// b-1 was already active — EnableUser must NOT be called for it
	gw.AssertNotCalled(t, "EnableUser", mock.Anything, "rw-1")
}

func TestOnSubscriptionResumed_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	orch := newOrchestrator(repo, gw, pub)

	repo.On("GetBySubscriptionID", mock.Anything, "sub-1").
		Return(nil, errors.New("db connection lost"))

	err := orch.OnSubscriptionResumed(ctx, "sub-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get bindings")
	gw.AssertNotCalled(t, "EnableUser")
}
