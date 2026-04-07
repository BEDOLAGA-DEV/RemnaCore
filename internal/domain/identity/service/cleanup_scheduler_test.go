package service_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
)

// mockCleanupRepo implements service.CleanupRepository for testing.
type mockCleanupRepo struct {
	mock.Mock
}

func (m *mockCleanupRepo) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockCleanupRepo) DeleteExpiredVerifications(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockCleanupRepo) DeleteExpiredPasswordResets(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func TestCleanupScheduler_Run_CancelsOnContext(t *testing.T) {
	repo := new(mockCleanupRepo)
	logger := slog.Default()

	scheduler := service.NewCleanupScheduler(repo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run should return promptly because the context is already cancelled.
	done := make(chan struct{})
	go func() {
		scheduler.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Success: Run returned after context cancellation.
	case <-time.After(2 * time.Second):
		t.Fatal("CleanupScheduler.Run did not return after context cancellation")
	}
}

func TestCleanupScheduler_DefaultInterval(t *testing.T) {
	// Verify the default interval constant is 1 hour.
	if service.DefaultCleanupInterval != 1*time.Hour {
		t.Errorf("expected DefaultCleanupInterval to be 1h, got %v", service.DefaultCleanupInterval)
	}
}

func TestCleanupScheduler_InterfaceCompliance(t *testing.T) {
	// Verify that mockCleanupRepo satisfies CleanupRepository at compile time.
	var _ service.CleanupRepository = (*mockCleanupRepo)(nil)
}

func TestNewCleanupScheduler(t *testing.T) {
	repo := new(mockCleanupRepo)
	logger := slog.Default()

	scheduler := service.NewCleanupScheduler(repo, logger)
	if scheduler == nil {
		t.Fatal("expected non-nil CleanupScheduler")
	}
}
