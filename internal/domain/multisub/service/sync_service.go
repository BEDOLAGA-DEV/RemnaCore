package service

import (
	"context"
	"time"

	multisubdomain "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// DefaultSyncInterval is the default period between periodic sync runs.
const DefaultSyncInterval = 5 * time.Minute

// SyncService orchestrates both periodic and event-driven synchronisation of
// Remnawave bindings. For periodic sync it spawns a background goroutine; for
// webhook events it delegates to SyncSaga.
type SyncService struct {
	bindings     multisubdomain.BindingRepository
	syncSaga     *SyncSaga
	publisher    domainevent.Publisher
	syncInterval time.Duration
}

// NewSyncService creates a SyncService with defaults.
func NewSyncService(
	bindings multisubdomain.BindingRepository,
	syncSaga *SyncSaga,
	publisher domainevent.Publisher,
) *SyncService {
	return &SyncService{
		bindings:     bindings,
		syncSaga:     syncSaga,
		publisher:    publisher,
		syncInterval: DefaultSyncInterval,
	}
}

// RunPeriodicSync starts a blocking loop that syncs all active bindings at the
// configured interval. It returns when the context is cancelled.
func (s *SyncService) RunPeriodicSync(ctx context.Context) {
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAll(ctx)
		}
	}
}

// syncAll fetches all active bindings and re-syncs each one via SyncSaga.
// Errors on individual bindings are published as events but do not abort the
// overall run.
func (s *SyncService) syncAll(ctx context.Context) {
	bindings, err := s.bindings.GetAllActive(ctx)
	if err != nil {
		return
	}

	for _, binding := range bindings {
		_ = s.syncSaga.SyncBinding(ctx, binding.ID)
	}
}

// OnWebhookEvent is called when a Remnawave webhook is received. It delegates
// to SyncSaga.HandleWebhookEvent.
func (s *SyncService) OnWebhookEvent(ctx context.Context, remnawaveUUID string, eventType domainevent.EventType) error {
	return s.syncSaga.HandleWebhookEvent(ctx, remnawaveUUID, eventType)
}
