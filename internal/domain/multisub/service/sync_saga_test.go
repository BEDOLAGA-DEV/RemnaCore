package service_test

import (
	"context"
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

func TestHandleWebhookEvent_TrafficExceeded(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	saga := service.NewSyncSaga(repo, gw, pub, clock.NewReal())

	binding := &aggregate.RemnawaveBinding{
		ID:             "b-1",
		SubscriptionID: "sub-1",
		PlatformUserID: "user-1",
		RemnawaveUUID:  "rw-uuid-1",
		Status:         aggregate.BindingActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	repo.On("GetByRemnawaveUUID", ctx, "rw-uuid-1").Return(binding, nil)
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-1" && b.Status == aggregate.BindingDisabled && b.SyncedAt != nil
	})).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := saga.HandleWebhookEvent(ctx, "rw-uuid-1", multisub.EventBindingTrafficExceeded)

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingDisabled, binding.Status)
	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestHandleWebhookEvent_UnknownBinding(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	saga := service.NewSyncSaga(repo, gw, pub, clock.NewReal())

	repo.On("GetByRemnawaveUUID", ctx, "rw-unknown").
		Return(nil, multisub.ErrBindingNotFound)

	err := saga.HandleWebhookEvent(ctx, "rw-unknown", multisub.EventBindingTrafficExceeded)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "find binding by remnawave uuid")
	repo.AssertExpectations(t)
	pub.AssertNotCalled(t, "Publish")
}

func TestSyncBinding_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	saga := service.NewSyncSaga(repo, gw, pub, clock.NewReal())

	binding := &aggregate.RemnawaveBinding{
		ID:             "b-1",
		SubscriptionID: "sub-1",
		PlatformUserID: "user-1",
		RemnawaveUUID:  "rw-uuid-1",
		Status:         aggregate.BindingActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	repo.On("GetByID", ctx, "b-1").Return(binding, nil)
	gw.On("GetUser", ctx, "rw-uuid-1").Return(&multisub.RemnawaveUserStatus{
		UUID:    "rw-uuid-1",
		Enabled: true,
		Expired: false,
	}, nil)
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-1" && b.SyncedAt != nil
	})).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := saga.SyncBinding(ctx, "b-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingActive, binding.Status)
	assert.NotNil(t, binding.SyncedAt)
	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestSyncBinding_DisabledRemotely(t *testing.T) {
	ctx := context.Background()
	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	pub := new(multisubtest.MockEventPublisher)

	saga := service.NewSyncSaga(repo, gw, pub, clock.NewReal())

	binding := &aggregate.RemnawaveBinding{
		ID:             "b-1",
		SubscriptionID: "sub-1",
		PlatformUserID: "user-1",
		RemnawaveUUID:  "rw-uuid-1",
		Status:         aggregate.BindingActive,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	repo.On("GetByID", ctx, "b-1").Return(binding, nil)
	gw.On("GetUser", ctx, "rw-uuid-1").Return(&multisub.RemnawaveUserStatus{
		UUID:    "rw-uuid-1",
		Enabled: false,
		Expired: false,
	}, nil)
	repo.On("Update", ctx, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
		return b.ID == "b-1" && b.Status == aggregate.BindingDisabled
	})).Return(nil)
	pub.On("Publish", ctx, mock.AnythingOfType("domainevent.Event")).Return(nil)

	err := saga.SyncBinding(ctx, "b-1")

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingDisabled, binding.Status)
	repo.AssertExpectations(t)
	gw.AssertExpectations(t)
	pub.AssertExpectations(t)
}
