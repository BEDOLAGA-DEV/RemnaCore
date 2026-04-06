package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

func newLifecycleService(
	repo *multisubtest.MockBindingRepo,
	gw *multisubtest.MockRemnawaveGateway,
	pub *multisubtest.MockEventPublisher,
) *service.BindingLifecycleService {
	return service.NewBindingLifecycleService(repo, gw, pub, clock.NewReal(), testLogger())
}

// --- DisableAllForSubscription ---

func TestDisableAllForSubscription(t *testing.T) {
	tests := []struct {
		name            string
		bindings        []*aggregate.RemnawaveBinding
		gatewayErr      error
		wantDisableCnt  int
		wantUpdateCnt   int
		wantFinalStatus aggregate.BindingStatus
	}{
		{
			name:           "no active bindings is a no-op",
			bindings:       []*aggregate.RemnawaveBinding{},
			wantDisableCnt: 0,
			wantUpdateCnt:  0,
		},
		{
			name: "three active bindings all disabled",
			bindings: []*aggregate.RemnawaveBinding{
				activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
				activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
				activeBinding("b-3", "sub-1", "rw-3", aggregate.PurposeStreaming),
			},
			wantDisableCnt:  3,
			wantUpdateCnt:   3,
			wantFinalStatus: aggregate.BindingDisabled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := new(multisubtest.MockBindingRepo)
			gw := new(multisubtest.MockRemnawaveGateway)
			pub := new(multisubtest.MockEventPublisher)

			svc := newLifecycleService(repo, gw, pub)

			repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
				Return(tt.bindings, nil)

			for _, b := range tt.bindings {
				gw.On("DisableUser", mock.Anything, b.RemnawaveUUID).Return(nil)
				repo.On("Update", mock.Anything, mock.MatchedBy(func(upd *aggregate.RemnawaveBinding) bool {
					return upd.ID == b.ID && upd.Status == aggregate.BindingDisabled
				})).Return(nil)
			}
			pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

			err := svc.DisableAllForSubscription(ctx, "sub-1")

			require.NoError(t, err)
			gw.AssertNumberOfCalls(t, "DisableUser", tt.wantDisableCnt)
			repo.AssertNumberOfCalls(t, "Update", tt.wantUpdateCnt)

			for _, b := range tt.bindings {
				if tt.wantFinalStatus != "" {
					assert.Equal(t, tt.wantFinalStatus, b.Status, "binding %s", b.ID)
				}
			}
			repo.AssertExpectations(t)
			gw.AssertExpectations(t)
		})
	}
}

func TestDisableAllForSubscription_GatewayFailure(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	svc := newLifecycleService(repo, gw, pub)

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
	}

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return(bindings, nil)

	// First binding: gateway fails -> should be marked failed
	gw.On("DisableUser", mock.Anything, "rw-1").
		Return(errors.New("connection refused"))
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-1" && b.Status == aggregate.BindingFailed
	})).Return(nil)

	// Second binding: gateway succeeds -> should be disabled
	gw.On("DisableUser", mock.Anything, "rw-2").Return(nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-2" && b.Status == aggregate.BindingDisabled
	})).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.DisableAllForSubscription(ctx, "sub-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingFailed, bindings[0].Status)
	assert.Equal(t, aggregate.BindingDisabled, bindings[1].Status)
	gw.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestDisableAllForSubscription_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	svc := newLifecycleService(repo, gw, pub)

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return(nil, errors.New("db connection lost"))

	err := svc.DisableAllForSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get active bindings")
	gw.AssertNotCalled(t, "DisableUser")
}

func TestDisableAllForSubscription_SkipsBindingsWithoutRemnawaveUUID(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	svc := newLifecycleService(repo, gw, pub)

	noUUID := &aggregate.RemnawaveBinding{
		ID:             "b-1",
		SubscriptionID: "sub-1",
		PlatformUserID: "user-1",
		RemnawaveUUID:  "", // no UUID
		Status:         aggregate.BindingActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").
		Return([]*aggregate.RemnawaveBinding{noUUID}, nil)

	err := svc.DisableAllForSubscription(ctx, "sub-1")

	require.NoError(t, err)
	gw.AssertNotCalled(t, "DisableUser")
	repo.AssertNotCalled(t, "Update")
}

// --- EnableAllForSubscription ---

func TestEnableAllForSubscription(t *testing.T) {
	tests := []struct {
		name           string
		bindings       []*aggregate.RemnawaveBinding
		wantEnableCnt  int
		wantUpdateCnt  int
	}{
		{
			name:          "no bindings is a no-op",
			bindings:      []*aggregate.RemnawaveBinding{},
			wantEnableCnt: 0,
			wantUpdateCnt: 0,
		},
		{
			name: "all active is a no-op",
			bindings: []*aggregate.RemnawaveBinding{
				activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
			},
			wantEnableCnt: 0,
			wantUpdateCnt: 0,
		},
		{
			name: "two disabled and one deprovisioned enables only disabled",
			bindings: []*aggregate.RemnawaveBinding{
				disabledBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
				disabledBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
				deprovisionedBinding("b-3", "sub-1", "rw-3", aggregate.PurposeStreaming),
			},
			wantEnableCnt: 2,
			wantUpdateCnt: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := new(multisubtest.MockBindingRepo)
			gw := new(multisubtest.MockRemnawaveGateway)
			pub := new(multisubtest.MockEventPublisher)

			svc := newLifecycleService(repo, gw, pub)

			repo.On("GetBySubscriptionID", mock.Anything, "sub-1").
				Return(tt.bindings, nil)

			for _, b := range tt.bindings {
				if b.Status == aggregate.BindingDisabled {
					gw.On("EnableUser", mock.Anything, b.RemnawaveUUID).Return(nil)
					repo.On("Update", mock.Anything, mock.MatchedBy(func(upd *aggregate.RemnawaveBinding) bool {
						return upd.ID == b.ID && upd.Status == aggregate.BindingActive
					})).Return(nil)
				}
			}
			pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

			err := svc.EnableAllForSubscription(ctx, "sub-1")

			require.NoError(t, err)
			gw.AssertNumberOfCalls(t, "EnableUser", tt.wantEnableCnt)
			repo.AssertNumberOfCalls(t, "Update", tt.wantUpdateCnt)

			for _, b := range tt.bindings {
				if tt.wantEnableCnt > 0 && b.Status == aggregate.BindingActive {
					// Disabled bindings should now be active
				}
			}
			repo.AssertExpectations(t)
			gw.AssertExpectations(t)
		})
	}
}

func TestEnableAllForSubscription_GatewayFailure(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	svc := newLifecycleService(repo, gw, pub)

	bindings := []*aggregate.RemnawaveBinding{
		disabledBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		disabledBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
	}

	repo.On("GetBySubscriptionID", mock.Anything, "sub-1").
		Return(bindings, nil)

	// First binding: gateway fails. The disabled -> failed transition is not
	// valid in the state machine, so MarkFailed logs a warning and the binding
	// stays disabled. No Update call is made for this binding.
	gw.On("EnableUser", mock.Anything, "rw-1").
		Return(errors.New("timeout"))

	// Second binding: gateway succeeds -> should be active
	gw.On("EnableUser", mock.Anything, "rw-2").Return(nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-2" && b.Status == aggregate.BindingActive
	})).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := svc.EnableAllForSubscription(ctx, "sub-1")

	require.NoError(t, err)
	// b-1 stays disabled because disabled -> failed is not a valid transition
	assert.Equal(t, aggregate.BindingDisabled, bindings[0].Status)
	assert.Equal(t, aggregate.BindingActive, bindings[1].Status)
	gw.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestEnableAllForSubscription_RepoError(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	svc := newLifecycleService(repo, gw, pub)

	repo.On("GetBySubscriptionID", mock.Anything, "sub-1").
		Return(nil, errors.New("db connection lost"))

	err := svc.EnableAllForSubscription(ctx, "sub-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "get bindings")
	gw.AssertNotCalled(t, "EnableUser")
}

// deprovisionedBinding returns a binding in the deprovisioned state.
func deprovisionedBinding(id, subID, rwUUID string, purpose aggregate.BindingPurpose) *aggregate.RemnawaveBinding {
	now := time.Now()
	return &aggregate.RemnawaveBinding{
		ID:             id,
		SubscriptionID: subID,
		PlatformUserID: "user-1",
		RemnawaveUUID:  rwUUID,
		Purpose:        purpose,
		Status:         aggregate.BindingDeprovisioned,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
