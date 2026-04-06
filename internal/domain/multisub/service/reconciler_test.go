package service_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager/txmanagertest"
)

func TestBindingReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway)
		assertMocks func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway)
	}{
		{
			name: "no orphaned bindings",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				repo.On("ResetAbandonedReconciling", mock.Anything, service.AbandonedClaimThreshold).
					Return(int64(0), nil)
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return([]*aggregate.RemnawaveBinding{}, nil)
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertNotCalled(t, "DeleteUser")
			},
		},
		{
			name: "successfully cleans up orphaned user",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				orphan := &aggregate.RemnawaveBinding{
					ID:            "binding-1",
					RemnawaveUUID: "rw-orphan-1",
					Status:        aggregate.BindingFailed,
				}
				repo.On("ResetAbandonedReconciling", mock.Anything, service.AbandonedClaimThreshold).
					Return(int64(0), nil)
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return([]*aggregate.RemnawaveBinding{orphan}, nil)
				// Claim: Update to reconciling status.
				repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
					return b.ID == "binding-1" && b.Status == aggregate.BindingReconciling
				})).Return(nil).Once()
				gw.On("DeleteUser", mock.Anything, "rw-orphan-1").Return(nil)
				// Release: Update with cleared UUID and back to failed.
				repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
					return b.ID == "binding-1" && b.RemnawaveUUID == "" && b.RemnawaveShortUUID == "" && b.Status == aggregate.BindingFailed
				})).Return(nil).Once()
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertExpectations(t)
			},
		},
		{
			name: "delete fails - binding released back to failed",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				orphan := &aggregate.RemnawaveBinding{
					ID:            "binding-2",
					RemnawaveUUID: "rw-orphan-2",
					Status:        aggregate.BindingFailed,
				}
				repo.On("ResetAbandonedReconciling", mock.Anything, service.AbandonedClaimThreshold).
					Return(int64(0), nil)
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return([]*aggregate.RemnawaveBinding{orphan}, nil)
				// Claim: Update to reconciling.
				repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
					return b.ID == "binding-2" && b.Status == aggregate.BindingReconciling
				})).Return(nil).Once()
				gw.On("DeleteUser", mock.Anything, "rw-orphan-2").
					Return(errors.New("connection refused"))
				// Release: Update back to failed (with remnawave_uuid still set).
				repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
					return b.ID == "binding-2" && b.Status == aggregate.BindingFailed && b.RemnawaveUUID == "rw-orphan-2"
				})).Return(nil).Once()
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertExpectations(t)
			},
		},
		{
			name: "query fails gracefully",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				repo.On("ResetAbandonedReconciling", mock.Anything, service.AbandonedClaimThreshold).
					Return(int64(0), nil)
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return(nil, errors.New("database unavailable"))
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertNotCalled(t, "DeleteUser")
			},
		},
		{
			name: "abandoned claims are reset",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				repo.On("ResetAbandonedReconciling", mock.Anything, service.AbandonedClaimThreshold).
					Return(int64(3), nil)
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return([]*aggregate.RemnawaveBinding{}, nil)
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertNotCalled(t, "DeleteUser")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(multisubtest.MockBindingRepo)
			gw := new(multisubtest.MockRemnawaveGateway)
			txRunner := txmanagertest.NoopTxRunner{}
			logger := slog.Default()

			tt.setupMocks(repo, gw)

			reconciler := service.NewBindingReconciler(repo, gw, txRunner, logger)
			reconciler.ReconcileOnce(context.Background())

			tt.assertMocks(t, repo, gw)
			_ = assert.ObjectsAreEqual // keep assert import
		})
	}
}

// TestBindingReconciler_ConcurrentClaim verifies that when two goroutines call
// ReconcileOnce concurrently, each binding is processed by exactly one
// reconciler. This simulates the TOCTOU race the status-based claim prevents.
//
// The claim-via-status approach means GetFailedForReconciliation only returns
// bindings with status='failed'. Once claimed (status='reconciling'), subsequent
// calls will not see them. We simulate this with Once/subsequent empty returns.
func TestBindingReconciler_ConcurrentClaim(t *testing.T) {
	// Track how many times each binding's DeleteUser is called.
	var deleteCountA atomic.Int32
	var deleteCountB atomic.Int32

	repo := new(multisubtest.MockBindingRepo)
	gw := new(multisubtest.MockRemnawaveGateway)
	txRunner := txmanagertest.NoopTxRunner{}
	logger := slog.Default()

	repo.On("ResetAbandonedReconciling", mock.Anything, service.AbandonedClaimThreshold).
		Return(int64(0), nil)

	// First call returns both bindings; second returns empty (already claimed).
	repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
		Return([]*aggregate.RemnawaveBinding{
			{
				ID:            "binding-a",
				RemnawaveUUID: "rw-a",
				Status:        aggregate.BindingFailed,
			},
			{
				ID:            "binding-b",
				RemnawaveUUID: "rw-b",
				Status:        aggregate.BindingFailed,
			},
		}, nil).Once()
	repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
		Return([]*aggregate.RemnawaveBinding{}, nil)

	// Accept all Update calls (claim + release).
	repo.On("Update", mock.Anything, mock.Anything).Return(nil)

	gw.On("DeleteUser", mock.Anything, "rw-a").
		Run(func(_ mock.Arguments) { deleteCountA.Add(1) }).
		Return(nil)
	gw.On("DeleteUser", mock.Anything, "rw-b").
		Run(func(_ mock.Arguments) { deleteCountB.Add(1) }).
		Return(nil)

	reconciler := service.NewBindingReconciler(repo, gw, txRunner, logger)

	// Run two concurrent reconciliation passes. Due to Once() on the first
	// GetFailedForReconciliation mock, only one goroutine will get bindings.
	var wg sync.WaitGroup
	const goroutineCount = 2
	wg.Add(goroutineCount)
	for range goroutineCount {
		go func() {
			defer wg.Done()
			reconciler.ReconcileOnce(context.Background())
		}()
	}
	wg.Wait()

	// Each binding should be deleted exactly once across all goroutines.
	require.Equal(t, int32(1), deleteCountA.Load(), "binding-a should be deleted exactly once")
	require.Equal(t, int32(1), deleteCountB.Load(), "binding-b should be deleted exactly once")
}
