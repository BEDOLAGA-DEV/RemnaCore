package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// ReplayDLQMessages reads up to limit messages from the DLQ stream, re-publishes
// the original payload to the original subject, and acknowledges the DLQ message.
// Idempotency on the consumer side ensures correctness when replaying messages
// that may have been partially processed.
//
// Messages that cannot be unmarshalled are acknowledged and skipped to prevent
// poison pills from blocking the replay. If a re-publish fails, the DLQ message
// is Nack'd so it remains available for a subsequent replay attempt.
func ReplayDLQMessages(ctx context.Context, subscriber *EventSubscriber, publisher *EventPublisher, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	ch, err := subscriber.Subscribe(ctx, "dlq.>")
	if err != nil {
		return 0, fmt.Errorf("subscribe to DLQ: %w", err)
	}

	replayed := 0
	for replayed < limit {
		select {
		case msg, ok := <-ch:
			if !ok {
				return replayed, nil
			}

			var payload DLQPayload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				msg.Ack() // skip malformed
				continue
			}

			// Re-publish to original subject with a new message ID.
			reMsg := message.NewMessage(watermill.NewUUID(), []byte(payload.OriginalPayload))
			if err := publisher.PublishRaw(payload.OriginalSubject, reMsg); err != nil {
				msg.Nack() // keep in DLQ for retry
				return replayed, fmt.Errorf("re-publish to %s: %w", payload.OriginalSubject, err)
			}

			msg.Ack()
			replayed++

		case <-ctx.Done():
			return replayed, ctx.Err()
		}
	}
	return replayed, nil
}
