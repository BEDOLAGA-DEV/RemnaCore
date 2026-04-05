package service_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return([]*aggregate.RemnawaveBinding{orphan}, nil)
				gw.On("DeleteUser", mock.Anything, "rw-orphan-1").Return(nil)
				repo.On("Update", mock.Anything, mock.MatchedBy(func(b *aggregate.RemnawaveBinding) bool {
					return b.ID == "binding-1" && b.RemnawaveUUID == "" && b.RemnawaveShortUUID == ""
				})).Return(nil)
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertExpectations(t)
			},
		},
		{
			name: "delete fails - binding left for next reconciliation",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				orphan := &aggregate.RemnawaveBinding{
					ID:            "binding-2",
					RemnawaveUUID: "rw-orphan-2",
					Status:        aggregate.BindingFailed,
				}
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return([]*aggregate.RemnawaveBinding{orphan}, nil)
				gw.On("DeleteUser", mock.Anything, "rw-orphan-2").
					Return(errors.New("connection refused"))
			},
			assertMocks: func(t *testing.T, repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				t.Helper()
				repo.AssertExpectations(t)
				gw.AssertExpectations(t)
				// Update should not be called since delete failed
				repo.AssertNotCalled(t, "Update")
			},
		},
		{
			name: "query fails gracefully",
			setupMocks: func(repo *multisubtest.MockBindingRepo, gw *multisubtest.MockRemnawaveGateway) {
				repo.On("GetFailedForReconciliation", mock.Anything, service.ReconcilerBatchLimit).
					Return(nil, errors.New("database unavailable"))
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

			// Call the exported Reconcile method for testing.
			// We test via ReconcileOnce which is the single-pass variant.
			reconciler.ReconcileOnce(context.Background())

			tt.assertMocks(t, repo, gw)
			_ = assert.ObjectsAreEqual // keep assert import
		})
	}
}
