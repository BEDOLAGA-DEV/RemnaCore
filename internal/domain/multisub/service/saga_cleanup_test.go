package service_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/multisubtest"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestSagaCleanup_ReportStaleOnStartup(t *testing.T) {
	tests := []struct {
		name         string
		running      []*multisub.SagaInstance
		runningErr   error
		expectFail   int
	}{
		{
			name:       "no stale sagas",
			running:    []*multisub.SagaInstance{},
			expectFail: 0,
		},
		{
			name: "marks stale sagas as failed",
			running: []*multisub.SagaInstance{
				{
					ID:            "saga-stale-1",
					SagaType:      multisub.SagaTypeProvisioning,
					CorrelationID: "sub-1",
					Status:        multisub.SagaStatusRunning,
					CurrentStep:   1,
					TotalSteps:    3,
					CreatedAt:     time.Now().Add(-1 * time.Hour),
				},
				{
					ID:            "saga-stale-2",
					SagaType:      multisub.SagaTypeDeprovisioning,
					CorrelationID: "sub-2",
					Status:        multisub.SagaStatusRunning,
					CurrentStep:   0,
					TotalSteps:    2,
					CreatedAt:     time.Now().Add(-2 * time.Hour),
				},
			},
			expectFail: 2,
		},
		{
			name:       "query error handled gracefully",
			runningErr: errors.New("db unavailable"),
			expectFail: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sagaRepo := new(multisubtest.MockSagaRepo)

			if tt.runningErr != nil {
				sagaRepo.On("GetRunning", mock.Anything).Return(nil, tt.runningErr)
			} else {
				sagaRepo.On("GetRunning", mock.Anything).Return(tt.running, nil)
			}

			for _, saga := range tt.running {
				sagaRepo.On("Fail", mock.Anything, saga.ID, "stale: process restarted before completion").Return(nil)
			}

			cleanup := service.NewSagaCleanupService(sagaRepo, quietLogger())
			cleanup.ReportStaleOnStartup(context.Background())

			sagaRepo.AssertExpectations(t)
		})
	}
}
