package infra_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/tenantctx"
)

// sentinelRecordingRunner captures the tenant ID present in the context handed
// to each RunInTx body, so the test can assert the platform sentinel is set.
type sentinelRecordingRunner struct {
	seenTenants []string
}

func (r *sentinelRecordingRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	r.seenTenants = append(r.seenTenants, tenantctx.TenantIDFromContext(ctx))
	return fn(ctx)
}

type stubCleaner struct{}

func (stubCleaner) CleanupOldKeys(_ context.Context, _ time.Time) (int64, error)    { return 0, nil }
func (stubCleaner) CleanupOldSyncLog(_ context.Context, _ time.Time) (int64, error) { return 0, nil }

func TestCleanup_RunsUnderPlatformSentinel(t *testing.T) {
	runner := &sentinelRecordingRunner{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := infra.NewDataCleanupScheduler(stubCleaner{}, stubCleaner{}, runner, logger)

	s.CleanupForTest(context.Background())

	for _, tenant := range runner.seenTenants {
		assert.Equal(t, tenantctx.PlatformScopeSentinel, tenant,
			"cleanup must run under the platform sentinel so it can touch FORCE-RLS tables")
	}
	assert.Len(t, runner.seenTenants, 2, "both cleaners must run under RunInTx")
}
