package service_test

import (
	"context"
	"errors"
	"log/slog"
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

func activeBinding(id, subID, rwUUID string, purpose aggregate.BindingPurpose) *aggregate.RemnawaveBinding {
	now := time.Now()
	return &aggregate.RemnawaveBinding{
		ID:             id,
		SubscriptionID: subID,
		PlatformUserID: "user-1",
		RemnawaveUUID:  rwUUID,
		Purpose:        purpose,
		Status:         aggregate.BindingActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestDeprovision_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewDeprovisioningSaga(repo, gw, pub, sagaRepo, clock.NewReal(), slog.Default())

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
		activeBinding("b-3", "sub-1", "rw-3", aggregate.PurposeFamilyMember),
	}

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").Return(bindings, nil)

	gw.On("DeleteUser", mock.Anything, "rw-1").Return(nil)
	gw.On("DeleteUser", mock.Anything, "rw-2").Return(nil)
	gw.On("DeleteUser", mock.Anything, "rw-3").Return(nil)

	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.Status == aggregate.BindingDeprovisioned
	})).Return(nil).Times(3)

	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil).Times(3)

	err := saga.Deprovision(ctx, "sub-1")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestDeprovision_PartialFailure(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewDeprovisioningSaga(repo, gw, pub, sagaRepo, clock.NewReal(), slog.Default())

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
		activeBinding("b-3", "sub-1", "rw-3", aggregate.PurposeFamilyMember),
	}

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-1").Return(bindings, nil)

	// First and third succeed, second fails
	gw.On("DeleteUser", mock.Anything, "rw-1").Return(nil)
	gw.On("DeleteUser", mock.Anything, "rw-2").Return(errors.New("connection refused"))
	gw.On("DeleteUser", mock.Anything, "rw-3").Return(nil)

	// Two deprovisioned updates + one failed update
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-1" && b.Status == aggregate.BindingDeprovisioned
	})).Return(nil).Once()
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-2" && b.Status == aggregate.BindingFailed
	})).Return(nil).Once()
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-3" && b.Status == aggregate.BindingDeprovisioned
	})).Return(nil).Once()

	// Events published only for the 2 successful deprovisions
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil).Times(2)

	err := saga.Deprovision(ctx, "sub-1")

	// Best-effort: NO error returned even though one binding failed
	require.NoError(t, err)

	// Verify the failed binding was marked as failed
	assert.Equal(t, aggregate.BindingFailed, bindings[1].Status)
	assert.Contains(t, bindings[1].FailReason, "connection refused")

	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestDeprovision_NoBindings(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	sagaRepo := newPermissiveSagaRepo()

	saga := service.NewDeprovisioningSaga(repo, gw, pub, sagaRepo, clock.NewReal(), slog.Default())

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-empty").
		Return([]*aggregate.RemnawaveBinding{}, nil)

	err := saga.Deprovision(ctx, "sub-empty")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	// No gateway or publisher calls expected
	gw.AssertNotCalled(t, "DeleteUser")
	pub.AssertNotCalled(t, "Publish")
}

func TestDeprovision_SagaPersistence(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)
	sagaRepo := new(multisubtest.MockSagaRepo)

	saga := service.NewDeprovisioningSaga(repo, gw, pub, sagaRepo, clock.NewReal(), slog.Default())

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-saga-d", "rw-1", aggregate.PurposeBase),
	}

	repo.On("GetActiveBySubscriptionID", mock.Anything, "sub-saga-d").Return(bindings, nil)
	gw.On("DeleteUser", mock.Anything, "rw-1").Return(nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.Status == aggregate.BindingDeprovisioned
	})).Return(nil)
	pub.On("Publish", mock.Anything, mock.AnythingOfType("domainevent.Event")).Return(nil)

	// Saga persistence expectations
	sagaRepo.On("Create", mock.Anything, mock.MatchedBy(func(s *multisub.SagaInstance) bool {
		return s.SagaType == multisub.SagaTypeDeprovisioning &&
			s.CorrelationID == "sub-saga-d"
	})).Return(&multisub.SagaInstance{ID: "saga-d-1"}, nil)
	sagaRepo.On("UpdateProgress", mock.Anything, "saga-d-1", 1, mock.Anything).Return(nil)
	sagaRepo.On("Complete", mock.Anything, "saga-d-1").Return(nil)

	err := saga.Deprovision(ctx, "sub-saga-d")

	require.NoError(t, err)
	sagaRepo.AssertExpectations(t)
}
