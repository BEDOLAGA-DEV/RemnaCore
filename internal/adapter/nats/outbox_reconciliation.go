package nats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// ReconciliationCheckInterval is how often the reconciliation check runs.
// Set to 5 minutes to balance observability with query overhead.
//
// Suggested Prometheus alert:
//
//	- alert: OutboxReconciliationGap
//	  expr: platform_outbox_reconciliation_sequence_gap > 100
//	  for: 15m
const ReconciliationCheckInterval = 5 * time.Minute

// ReconciliationCheck compares the last published outbox sequence with
// JetStream stream state. Logs a warning if there is a significant gap,
// indicating potential missed events from the partial-commit edge case
// (see OutboxRelay type doc for details).
//
// This is observability only — it does not correct or republish events.
// The gap metric (platform_outbox_reconciliation_sequence_gap) can be
// used to trigger manual investigation when it remains non-zero for an
// extended period.
//
// The comparison is approximate by design: the outbox sequence is a
// monotonic bigint, while JetStream message count includes all streams.
// A transient gap is normal during active relaying; only a sustained gap
// after the relay has drained the backlog warrants investigation.
func (r *OutboxRelay) ReconciliationCheck(ctx context.Context) error {
	lastPublished, err := r.outbox.GetLastPublishedSequence(ctx)
	if err != nil {
		return fmt.Errorf("get last published sequence: %w", err)
	}

	// Sum message counts across all domain event streams. The outbox
	// publishes to all of them, so the aggregate count is the closest
	// comparable metric to the outbox sequence.
	totalMessages, err := r.jetStreamMessageCount(ctx)
	if err != nil {
		return fmt.Errorf("get jetstream message count: %w", err)
	}

	gap := lastPublished - totalMessages
	r.logger.Info("outbox reconciliation check",
		slog.Int64("last_published_sequence", lastPublished),
		slog.Int64("jetstream_total_messages", totalMessages),
		slog.Int64("gap", gap),
	)

	if r.metrics != nil {
		r.metrics.OutboxReconciliationSeqGap.Set(float64(gap))
	}

	return nil
}

// jetStreamMessageCount returns the total number of messages across all
// domain event JetStream streams (excluding DLQ). Returns 0 if the
// JetStream context cannot be initialised.
func (r *OutboxRelay) jetStreamMessageCount(ctx context.Context) (int64, error) {
	if r.natsConn == nil {
		return 0, fmt.Errorf("nats connection is nil")
	}

	js, err := jetstream.New(r.natsConn)
	if err != nil {
		return 0, fmt.Errorf("initialising jetstream context: %w", err)
	}

	// Domain event streams — DLQ is excluded because it contains
	// reprocessed/failed messages that would skew the count.
	domainStreams := []string{
		StreamIdentity,
		StreamBilling,
		StreamRemnawave,
		StreamPayment,
		StreamInfra,
		StreamReseller,
		StreamPlugins,
	}

	var total int64
	for _, name := range domainStreams {
		stream, streamErr := js.Stream(ctx, name)
		if streamErr != nil {
			r.logger.Debug("outbox reconciliation: failed to get stream",
				slog.String("stream", name),
				slog.Any("error", streamErr),
			)
			continue
		}
		info, infoErr := stream.Info(ctx)
		if infoErr != nil {
			r.logger.Debug("outbox reconciliation: failed to get stream info",
				slog.String("stream", name),
				slog.Any("error", infoErr),
			)
			continue
		}
		total += int64(info.State.Msgs)
	}

	return total, nil
}

// runReconciliation periodically runs the reconciliation check. It runs as
// a single background goroutine started by Run and exits when the context
// is cancelled. If the NATS connection or metrics dependency is nil, the
// goroutine returns immediately to avoid panics in test environments.
func (r *OutboxRelay) runReconciliation(ctx context.Context) {
	if r.natsConn == nil || r.metrics == nil {
		return
	}

	ticker := time.NewTicker(ReconciliationCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.ReconciliationCheck(ctx); err != nil {
				r.logger.Warn("outbox reconciliation check failed",
					slog.Any("error", err),
				)
			}
		}
	}
}
