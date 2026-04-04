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

	saga := service.NewDeprovisioningSaga(repo, gw, pub, clock.NewReal())

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
		activeBinding("b-3", "sub-1", "rw-3", aggregate.PurposeFamilyMember),
	}

	repo.On("GetActiveBySubscriptionID", ctx, "sub-1").Return(bindings, nil)

	gw.On("DeleteUser", ctx, "rw-1").Return(nil)
	gw.On("DeleteUser", ctx, "rw-2").Return(nil)
	gw.On("DeleteUser", ctx, "rw-3").Return(nil)

	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.Status == aggregate.BindingDeprovisioned
	})).Return(nil).Times(3)

	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil).Times(3)

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

	saga := service.NewDeprovisioningSaga(repo, gw, pub, clock.NewReal())

	bindings := []*aggregate.RemnawaveBinding{
		activeBinding("b-1", "sub-1", "rw-1", aggregate.PurposeBase),
		activeBinding("b-2", "sub-1", "rw-2", aggregate.PurposeGaming),
		activeBinding("b-3", "sub-1", "rw-3", aggregate.PurposeFamilyMember),
	}

	repo.On("GetActiveBySubscriptionID", ctx, "sub-1").Return(bindings, nil)

	// First and third succeed, second fails
	gw.On("DeleteUser", ctx, "rw-1").Return(nil)
	gw.On("DeleteUser", ctx, "rw-2").Return(errors.New("connection refused"))
	gw.On("DeleteUser", ctx, "rw-3").Return(nil)

	// Two deprovisioned updates + one failed update
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-1" && b.Status == aggregate.BindingDeprovisioned
	})).Return(nil).Once()
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-2" && b.Status == aggregate.BindingFailed
	})).Return(nil).Once()
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-3" && b.Status == aggregate.BindingDeprovisioned
	})).Return(nil).Once()

	// Events published only for the 2 successful deprovisions
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil).Times(2)

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

	saga := service.NewDeprovisioningSaga(repo, gw, pub, clock.NewReal())

	repo.On("GetActiveBySubscriptionID", ctx, "sub-empty").
		Return([]*aggregate.RemnawaveBinding{}, nil)

	err := saga.Deprovision(ctx, "sub-empty")

	require.NoError(t, err)
	repo.AssertExpectations(t)
	// No gateway or publisher calls expected
	gw.AssertNotCalled(t, "DeleteUser")
	pub.AssertNotCalled(t, "Publish")
}
