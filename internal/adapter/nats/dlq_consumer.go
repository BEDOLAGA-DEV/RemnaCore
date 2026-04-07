package nats

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"
)

// DLQ consumer constants.
const (
	// dlqSubscribeSubject is the wildcard subject pattern for DLQ messages.
	dlqSubscribeSubject = "dlq.>"
)

// DLQLogConsumer subscribes to the DLQ stream and logs every message at ERROR
// level with structured fields for centralized logging and alerting visibility.
// It is a read-only observer -- it acknowledges messages after logging but does
// not replay or modify them. The consumer runs as a background goroutine
// started via Start and exits when the context is cancelled.
//
// This consumer is complementary to the DLQ depth gauge (polled by
// BillingEventConsumer.pollDLQDepth) and the DLQ replay tool
// (ReplayDLQMessages). Together they provide three layers of DLQ observability:
//   - Depth gauge: how many messages are pending
//   - Log consumer: structured ERROR log per message for alerting pipelines
//   - Replay tool: manual or automated re-processing of DLQ messages
type DLQLogConsumer struct {
	subscriber *EventSubscriber
	logger     *slog.Logger
}

// NewDLQLogConsumer creates a DLQLogConsumer with the given dependencies.
func NewDLQLogConsumer(
	subscriber *EventSubscriber,
	logger *slog.Logger,
) *DLQLogConsumer {
	return &DLQLogConsumer{
		subscriber: subscriber,
		logger:     logger,
	}
}

// Start subscribes to the DLQ stream and processes messages in a background
// goroutine. It returns immediately; the goroutine runs until the context is
// cancelled or the subscription channel is closed.
func (c *DLQLogConsumer) Start(ctx context.Context) error {
	ch, err := c.subscriber.Subscribe(ctx, dlqSubscribeSubject)
	if err != nil {
		return err
	}

	go c.consumeLoop(ctx, ch)

	c.logger.Info("dlq log consumer started")
	return nil
}

// consumeLoop reads DLQ messages and logs them until the context is cancelled.
func (c *DLQLogConsumer) consumeLoop(ctx context.Context, ch <-chan *message.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.handleMessage(msg)
		}
	}
}

// handleMessage parses a DLQ message and logs its contents at ERROR level.
// Malformed payloads are logged with the raw payload bytes and acknowledged
// to prevent poison pills from blocking the consumer.
func (c *DLQLogConsumer) handleMessage(msg *message.Message) {
	var payload DLQPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		c.logger.Error("dlq message with unparseable payload",
			slog.String("msg_id", msg.UUID),
			slog.String("raw_payload", string(msg.Payload)),
			slog.Any("error", err),
		)
		msg.Ack()
		return
	}

	c.logger.Error("dead letter event",
		slog.String("original_subject", payload.OriginalSubject),
		slog.String("event_type", payload.EventType),
		slog.String("entity_id", payload.EntityID),
		slog.String("error", payload.Error),
		slog.Int("retry_count", payload.RetryCount),
		slog.String("failed_at", payload.FailedAt),
		slog.String("msg_id", payload.MsgID),
	)

	msg.Ack()
}
