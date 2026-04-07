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

// domainStreams lists the JetStream streams that carry domain events. DLQ is
// excluded because it contains reprocessed/failed messages that would skew
// the reconciliation delta.
var domainStreams = []string{
	StreamIdentity,
	StreamBilling,
	StreamRemnawave,
	StreamPayment,
	StreamInfra,
	StreamReseller,
	StreamPlugins,
}

// ReconciliationCheck compares outbox and JetStream deltas over the last
// reconciliation interval. Unlike the previous all-time-total comparison,
// this delta-based approach avoids comparing monotonic outbox sequences
// (which never reset) against JetStream message counts (which can shrink
// due to retention/limits). Only a sustained positive gap warrants
// investigation.
//
// On the first invocation both stored values are zero, so the full current
// state becomes the "previous baseline" and the gap is reported as zero.
// This prevents false alarms at startup.
//
// This is observability only — it does not correct or republish events.
// The gap metric (platform_outbox_reconciliation_sequence_gap) can be
// used to trigger manual investigation when it remains non-zero for an
// extended period.
func (r *OutboxRelay) ReconciliationCheck(ctx context.Context) error {
	currentSeq, err := r.outbox.GetLastPublishedSequence(ctx)
	if err != nil {
		return fmt.Errorf("get last published sequence: %w", err)
	}

	jsTotal, err := r.jetStreamLastSeqSum(ctx)
	if err != nil {
		return fmt.Errorf("get jetstream last sequence sum: %w", err)
	}

	// Compare deltas since last check.
	prevSeq := r.lastReconciledSeq.Load()
	prevJS := r.lastJSSeq.Load()

	outboxDelta := currentSeq - prevSeq
	jsDelta := jsTotal - prevJS

	// Update stored baselines for the next check.
	r.lastReconciledSeq.Store(currentSeq)
	r.lastJSSeq.Store(jsTotal)

	// Negative gap means JetStream received more than the outbox published
	// (possible during catch-up or redelivery). Clamp to zero.
	gap := outboxDelta - jsDelta
	if gap < 0 {
		gap = 0
	}

	if r.metrics != nil {
		r.metrics.OutboxReconciliationSeqGap.Set(float64(gap))
	}

	if gap > 0 {
		r.logger.Warn("outbox reconciliation: delivery gap detected",
			slog.Int64("outbox_delta", outboxDelta),
			slog.Int64("jetstream_delta", jsDelta),
			slog.Int64("gap", gap),
		)
	} else {
		r.logger.Info("outbox reconciliation check",
			slog.Int64("outbox_delta", outboxDelta),
			slog.Int64("jetstream_delta", jsDelta),
		)
	}

	return nil
}

// jetStreamLastSeqSum returns the sum of LastSeq across all domain event
// JetStream streams (excluding DLQ). LastSeq is a monotonically increasing
// sequence number per stream, making it suitable for delta-based comparison
// with the outbox sequence. Returns 0 if the JetStream context cannot be
// initialised.
func (r *OutboxRelay) jetStreamLastSeqSum(ctx context.Context) (int64, error) {
	if r.natsConn == nil {
		return 0, fmt.Errorf("nats connection is nil")
	}

	js, err := jetstream.New(r.natsConn)
	if err != nil {
		return 0, fmt.Errorf("initialising jetstream context: %w", err)
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
		total += int64(info.State.LastSeq)
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
