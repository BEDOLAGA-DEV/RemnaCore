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
//	  expr: max(platform_outbox_reconciliation_sequence_gap) > 100
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

// ReconciliationCheck compares per-stream JetStream LastSeq deltas between
// reconciliation intervals. A stream whose LastSeq stops growing while the
// relay is actively publishing to its subjects indicates a delivery gap.
//
// The previous implementation summed LastSeq across all streams into a single
// number and compared it against the outbox sequence. This was an
// apples-to-oranges comparison: the outbox uses a single autoincrement
// sequence spanning all event types, while each JetStream stream has its
// own independent LastSeq counter. A quiet stream could mask a gap in a
// busy stream (and vice versa).
//
// Per-stream tracking eliminates this class of false negatives by reporting
// each stream's delta independently.
//
// On the first invocation, all previous values are zero, so the current
// state becomes the baseline and no gap is reported. This prevents false
// alarms at startup.
//
// This is observability only -- it does not correct or republish events.
// The per-stream gap metric (platform_outbox_reconciliation_sequence_gap
// with a "stream" label) can be used to trigger manual investigation when
// it remains non-zero for an extended period.
func (r *OutboxRelay) ReconciliationCheck(ctx context.Context) error {
	currentSeqs, err := r.jetStreamPerStreamSeq(ctx)
	if err != nil {
		return fmt.Errorf("get jetstream per-stream sequences: %w", err)
	}

	r.lastStreamSeqMu.Lock()
	defer r.lastStreamSeqMu.Unlock()

	for _, streamName := range domainStreams {
		currentLastSeq := currentSeqs[streamName]
		prevLastSeq := r.lastStreamSeq[streamName]

		delta := currentLastSeq - prevLastSeq

		// Update baseline for next check.
		r.lastStreamSeq[streamName] = currentLastSeq

		// On the first invocation prevLastSeq is zero; delta equals the
		// full current LastSeq. Skip reporting to avoid a startup spike.
		if prevLastSeq == 0 {
			continue
		}

		// Negative delta means LastSeq decreased, which should not happen
		// under normal operation (LastSeq is monotonic). Log it but clamp
		// the gap metric to zero.
		if delta < 0 {
			r.logger.Warn("outbox reconciliation: stream LastSeq decreased",
				slog.String("stream", streamName),
				slog.Int64("previous", prevLastSeq),
				slog.Int64("current", currentLastSeq),
			)
			if r.metrics != nil {
				r.metrics.OutboxReconciliationSeqGap.WithLabelValues(streamName).Set(0)
			}
			continue
		}

		if r.metrics != nil {
			r.metrics.OutboxReconciliationSeqGap.WithLabelValues(streamName).Set(float64(delta))
		}

		// A zero delta means the stream received no new messages during
		// this interval. This is normal for quiet streams but is logged
		// at debug level for troubleshooting.
		if delta == 0 {
			r.logger.Debug("outbox reconciliation: stream idle",
				slog.String("stream", streamName),
				slog.Int64("last_seq", currentLastSeq),
			)
		} else {
			r.logger.Info("outbox reconciliation: stream healthy",
				slog.String("stream", streamName),
				slog.Int64("delta", delta),
				slog.Int64("last_seq", currentLastSeq),
			)
		}
	}

	return nil
}

// jetStreamPerStreamSeq returns the current LastSeq for each domain event
// stream. Streams that cannot be queried are silently excluded from the
// result (logged at debug level). Returns an error only if the JetStream
// context itself cannot be initialised.
func (r *OutboxRelay) jetStreamPerStreamSeq(ctx context.Context) (map[string]int64, error) {
	if r.natsConn == nil {
		return nil, fmt.Errorf("nats connection is nil")
	}

	js, err := jetstream.New(r.natsConn)
	if err != nil {
		return nil, fmt.Errorf("initialising jetstream context: %w", err)
	}

	seqs := make(map[string]int64, len(domainStreams))
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
		seqs[name] = int64(info.State.LastSeq)
	}

	return seqs, nil
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
