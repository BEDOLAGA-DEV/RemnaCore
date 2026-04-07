package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// Scheduler interval and batch size constants.
const (
	// DefaultSchedulerInterval is the default period between scheduler ticks.
	DefaultSchedulerInterval = 1 * time.Minute

	// SchedulerBatchSize is the maximum number of subscriptions processed per tick.
	SchedulerBatchSize = 100
)

// SubscriptionScheduler periodically finds subscriptions that need
// expiration or renewal and triggers the appropriate lifecycle action.
//
// Active subscriptions whose billing period has elapsed are classified as:
//   - Renewable: the BillingService.RenewSubscription method is called, which
//     advances the period and creates a renewal invoice.
//   - Expirable: the BillingService.ExpireSubscription method is called, which
//     moves the subscription to the expired terminal state.
//
// The scheduler does not distinguish between renewable and expirable
// subscriptions at query time. Both are fetched via GetExpiredActive (active
// subs where period_end < now). The Renew method's own precondition check
// (period must be elapsed, status must be active) makes this safe: if Renew
// fails because the subscription should not be renewed, the scheduler falls
// through to ExpireSubscription.
type SubscriptionScheduler struct {
	subs    billing.SubscriptionRepository
	billing *BillingService
	clock   clock.Clock
	logger  *slog.Logger

	tickInterval time.Duration
}

// NewSubscriptionScheduler creates a SubscriptionScheduler with its
// dependencies and the default tick interval.
func NewSubscriptionScheduler(
	subs billing.SubscriptionRepository,
	billingSvc *BillingService,
	clk clock.Clock,
	logger *slog.Logger,
) *SubscriptionScheduler {
	return &SubscriptionScheduler{
		subs:         subs,
		billing:      billingSvc,
		clock:        clk,
		logger:       logger,
		tickInterval: DefaultSchedulerInterval,
	}
}

// Run starts the scheduler loop. It blocks until the context is cancelled.
func (s *SubscriptionScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processDueSubscriptions(ctx)
		}
	}
}

// processDueSubscriptions fetches active subscriptions whose billing period
// has elapsed and attempts renewal first, falling back to expiration.
func (s *SubscriptionScheduler) processDueSubscriptions(ctx context.Context) {
	now := s.clock.Now()
	subs, err := s.subs.GetExpiredActive(ctx, now, SchedulerBatchSize)
	if err != nil {
		s.logger.Error("subscription scheduler: failed to fetch due subscriptions",
			slog.Any("error", err),
		)
		return
	}

	if len(subs) == 0 {
		return
	}

	s.logger.Info("subscription scheduler: processing due subscriptions",
		slog.Int("count", len(subs)),
	)

	for _, sub := range subs {
		// Attempt renewal first. RenewSubscription will fail with
		// ErrSubscriptionNotActiveForRenewal or ErrPeriodNotElapsed
		// if the subscription is ineligible, in which case we expire it.
		if err := s.billing.RenewSubscription(ctx, sub.ID); err != nil {
			s.logger.Info("subscription scheduler: renewal ineligible, expiring",
				slog.String("subscription_id", sub.ID),
				slog.Any("renewal_error", err),
			)
			if expErr := s.billing.ExpireSubscription(ctx, sub.ID); expErr != nil {
				s.logger.Error("subscription scheduler: failed to expire subscription",
					slog.String("subscription_id", sub.ID),
					slog.Any("error", expErr),
				)
			}
		}
	}
}
