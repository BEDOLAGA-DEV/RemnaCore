package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// ReplayOptions configures how DLQ messages are replayed.
type ReplayOptions struct {
	// Limit caps the number of messages replayed. Zero means replay all available messages.
	Limit int

	// FilterSubject restricts replay to messages whose OriginalSubject contains
	// this substring. Empty means all subjects match.
	FilterSubject string

	// FilterEntity restricts replay to messages whose EntityID equals this value
	// exactly. Empty means all entities match.
	FilterEntity string

	// DryRun when true collects matching DLQ payloads without re-publishing them.
	DryRun bool

	// DelayBetween inserts a pause between consecutive re-publishes for rate
	// limiting. Zero means no delay.
	DelayBetween time.Duration
}

// ReplayResult summarises the outcome of a DLQ replay operation.
type ReplayResult struct {
	// Replayed is the number of messages successfully re-published (or matched in dry-run mode).
	Replayed int

	// Skipped is the number of messages that did not match the filter criteria.
	Skipped int

	// Errors is the number of messages that failed to re-publish. The replay
	// stops on the first publish error; this count will be 0 or 1.
	Errors int

	// Matched holds the DLQ payloads that were replayed (or would be replayed
	// in dry-run mode). This is always populated so callers can inspect what
	// was processed.
	Matched []DLQPayload
}

// unlimitedReplayBound is the internal sentinel used when ReplayOptions.Limit
// is zero (meaning "replay all"). We set an upper bound to prevent unbounded
// loops against misconfigured streams.
const unlimitedReplayBound = 1_000_000

// ReplayDLQMessages reads messages from the DLQ stream and re-publishes the
// original payload to the original subject, filtered and rate-limited according
// to the provided options.
//
// Messages that cannot be unmarshalled are acknowledged and skipped to prevent
// poison pills from blocking the replay. If a re-publish fails, the DLQ message
// is Nack'd so it remains available for a subsequent replay attempt.
//
// When DryRun is true, matching messages are collected and acknowledged without
// re-publishing. The original MsgID from the DLQ payload is preserved as the
// Watermill message UUID to enable JetStream server-side deduplication on replay.
func ReplayDLQMessages(ctx context.Context, subscriber *EventSubscriber, publisher *EventPublisher, opts ReplayOptions) (ReplayResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = unlimitedReplayBound
	}

	ch, err := subscriber.Subscribe(ctx, dlqSubscribeSubject)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("subscribe to DLQ: %w", err)
	}

	return replayFromChannel(ctx, &channelSubscription{ch: ch}, publisher, opts, limit)
}

// channelSubscription adapts a raw channel to the dlqSubscriber interface.
type channelSubscription struct {
	ch <-chan *message.Message
}

func (c *channelSubscription) Subscribe(_ context.Context, _ string) (<-chan *message.Message, error) {
	return c.ch, nil
}

// dlqSubscriber abstracts the Subscribe call so the core replay logic can be
// tested with stubs instead of a real NATS connection.
type dlqSubscriber interface {
	Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error)
}

// dlqPublisher abstracts the PublishRaw call so the core replay logic can be
// tested with stubs instead of a real NATS connection.
type dlqPublisher interface {
	PublishRaw(topic string, msg *message.Message) error
}

// replayFromChannel contains the core replay logic, accepting interfaces for
// testability. Both the production ReplayDLQMessages and the test helper
// replayDLQFromChannel delegate to this function.
func replayFromChannel(ctx context.Context, subscriber dlqSubscriber, publisher dlqPublisher, opts ReplayOptions, limit int) (ReplayResult, error) {
	ch, err := subscriber.Subscribe(ctx, dlqSubscribeSubject)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("subscribe to DLQ: %w", err)
	}

	var result ReplayResult
	processed := 0

	for processed < limit {
		select {
		case msg, ok := <-ch:
			if !ok {
				return result, nil
			}

			var payload DLQPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				msg.Ack() // skip malformed
				continue
			}

			if !matchesFilter(payload, opts) {
				result.Skipped++
				msg.Nack() // leave non-matching messages in the stream
				continue
			}

			result.Matched = append(result.Matched, payload)

			if opts.DryRun {
				msg.Ack()
				result.Replayed++
				processed++
				continue
			}

			// Preserve the original MsgID as the Watermill message UUID so that
			// JetStream's Nats-Msg-Id header enables server-side deduplication
			// when the same DLQ message is replayed more than once.
			msgID := payload.MsgID
			if msgID == "" {
				msgID = watermill.NewUUID()
			}

			reMsg := message.NewMessage(msgID, []byte(payload.OriginalPayload))
			if err := publisher.PublishRaw(payload.OriginalSubject, reMsg); err != nil {
				msg.Nack() // keep in DLQ for retry
				result.Errors++
				return result, fmt.Errorf("re-publish to %s: %w", payload.OriginalSubject, err)
			}

			msg.Ack()
			result.Replayed++
			processed++

			// Rate-limit between re-publishes if configured.
			if opts.DelayBetween > 0 && processed < limit {
				select {
				case <-ctx.Done():
					return result, ctx.Err()
				case <-time.After(opts.DelayBetween):
				}
			}

		case <-ctx.Done():
			return result, ctx.Err()
		}
	}
	return result, nil
}

// matchesFilter returns true if the payload passes all configured filter criteria.
func matchesFilter(payload DLQPayload, opts ReplayOptions) bool {
	if opts.FilterSubject != "" && !strings.Contains(payload.OriginalSubject, opts.FilterSubject) {
		return false
	}
	if opts.FilterEntity != "" && payload.EntityID != opts.FilterEntity {
		return false
	}
	return true
}
