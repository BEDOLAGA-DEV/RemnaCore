package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// MaxPluginAsyncRetries is the maximum number of processing attempts before a
// plugin async message is routed to the dead-letter queue.
const MaxPluginAsyncRetries = 3

// pluginAsyncDLQSubject is the DLQ subject for permanently failed plugin async
// hook messages.
const pluginAsyncDLQSubject = DLQSubjectPrefix + "plugin.hook"

// PluginAsyncConsumer subscribes to plugin.hook.* topics on NATS JetStream and
// dispatches each message to the appropriate plugin runtime handlers. Messages
// that fail processing are retried up to MaxPluginAsyncRetries times with
// exponential backoff before being sent to the dead-letter queue.
type PluginAsyncConsumer struct {
	subscriber  *EventSubscriber
	dispatcher  *plugin.HookDispatcher
	runtime     *plugin.RuntimePool
	publisher   *EventPublisher
	clock       clock.Clock
	logger      *slog.Logger
	retryCount  map[string]int // msg UUID -> retry attempts (in-memory; lost on restart)
}

// NewPluginAsyncConsumer creates a consumer that bridges NATS async hook events
// to the plugin runtime pool. The publisher is used to route permanently failed
// messages to the dead-letter queue.
func NewPluginAsyncConsumer(
	subscriber *EventSubscriber,
	dispatcher *plugin.HookDispatcher,
	runtime *plugin.RuntimePool,
	publisher *EventPublisher,
	clk clock.Clock,
	logger *slog.Logger,
) *PluginAsyncConsumer {
	return &PluginAsyncConsumer{
		subscriber: subscriber,
		dispatcher: dispatcher,
		runtime:    runtime,
		publisher:  publisher,
		clock:      clk,
		logger:     logger,
		retryCount: make(map[string]int),
	}
}

// asyncHookPayload is the envelope published by DispatchAsync.
type asyncHookPayload struct {
	HookName string `json:"hook_name"`
	Payload  string `json:"payload"`
}

// Start begins consuming async hook events from the "plugin.hook.>" subject.
// It blocks until the context is cancelled.
func (c *PluginAsyncConsumer) Start(ctx context.Context) error {
	ch, err := c.subscriber.Subscribe(ctx, "plugin.hook.>")
	if err != nil {
		return fmt.Errorf("subscribe plugin hook events: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				c.handleMessage(ctx, msg)
			}
		}
	}()

	return nil
}

// handleMessage parses an incoming NATS message, extracts the hook name from
// the subject, and dispatches the payload to all registered async handlers.
//
// On failure, the message is retried up to MaxPluginAsyncRetries times with
// exponential backoff (reusing retryBackoffDelay from the billing consumer).
// Messages that exhaust all retries are sent to the dead-letter queue and
// Ack'd to prevent infinite redelivery.
func (c *PluginAsyncConsumer) handleMessage(ctx context.Context, msg *message.Message) {
	var envelope asyncHookPayload
	if err := json.Unmarshal(msg.Payload, &envelope); err != nil {
		c.logger.Error("failed to unmarshal async hook payload, sending to DLQ",
			slog.String("msg_id", msg.UUID),
			slog.Any("error", err),
		)
		c.sendPluginToDLQ(msg, err)
		msg.Ack()
		return
	}

	hookName := envelope.HookName
	if hookName == "" {
		// Fallback: derive from the NATS subject (plugin.hook.<hookName>).
		subject := msg.Metadata.Get("subject")
		if subject != "" {
			hookName = strings.TrimPrefix(subject, "plugin.hook.")
		}
	}

	if hookName == "" {
		c.logger.Warn("async hook message missing hook name, skipping",
			slog.String("msg_id", msg.UUID),
		)
		msg.Ack()
		return
	}

	dispatchErr := c.dispatchToPlugins(ctx, hookName, envelope)
	if dispatchErr == nil {
		msg.Ack()
		return
	}

	// Processing failed — determine retry or DLQ.
	retryAttempt := c.retryCount[msg.UUID] + 1
	c.retryCount[msg.UUID] = retryAttempt

	if retryAttempt < MaxPluginAsyncRetries {
		delay := retryBackoffDelay(retryAttempt)
		c.logger.Warn("plugin async handler failed, retrying",
			slog.String("hook", hookName),
			slog.String("msg_id", msg.UUID),
			slog.Int("attempt", retryAttempt),
			slog.Int("max_retries", MaxPluginAsyncRetries),
			slog.Duration("backoff", delay),
			slog.Any("error", dispatchErr),
		)
		// Context-aware backoff: select with context so shutdown cancellation
		// interrupts the wait instead of blocking the goroutine.
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			msg.Nack()
			return
		}
		msg.Nack()
		return
	}

	// Max retries exceeded — send to DLQ and acknowledge to stop redelivery.
	c.logger.Error("plugin async handler permanently failed, sending to DLQ",
		slog.String("hook", hookName),
		slog.String("msg_id", msg.UUID),
		slog.Int("retries_exhausted", retryAttempt),
		slog.Any("error", dispatchErr),
	)
	c.sendPluginToDLQ(msg, dispatchErr)
	delete(c.retryCount, msg.UUID)
	msg.Ack()
}

// dispatchToPlugins dispatches the async hook payload to all registered plugin
// handlers. Returns the first error encountered, or nil if all succeed.
func (c *PluginAsyncConsumer) dispatchToPlugins(
	ctx context.Context,
	hookName string,
	envelope asyncHookPayload,
) error {
	regs := c.dispatcher.Registrations(hookName)
	var firstErr error
	for _, reg := range regs {
		if reg.HookType != plugin.HookAsync {
			continue
		}

		hookCtx := sdk.HookContext{
			HookName: hookName,
			PluginID: reg.PluginSlug,
			Payload:  json.RawMessage(envelope.Payload),
		}

		input, err := json.Marshal(hookCtx)
		if err != nil {
			c.logger.Error("failed to marshal async hook context",
				slog.String("hook", hookName),
				slog.String("plugin", reg.PluginSlug),
				slog.Any("error", err),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("marshal hook context for %s: %w", reg.PluginSlug, err)
			}
			continue
		}

		if _, err := c.runtime.CallHook(ctx, reg.PluginSlug, reg.FuncName, input); err != nil {
			c.logger.Error("async hook execution failed",
				slog.String("hook", hookName),
				slog.String("plugin", reg.PluginSlug),
				slog.Any("error", err),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("hook %s plugin %s: %w", hookName, reg.PluginSlug, err)
			}
		}
	}
	return firstErr
}

// sendPluginToDLQ publishes a failed plugin async message to the dead-letter
// queue. The DLQ message preserves the original payload for debugging.
func (c *PluginAsyncConsumer) sendPluginToDLQ(msg *message.Message, processingErr error) {
	dlqPayload := DLQPayload{
		OriginalSubject: "plugin.hook",
		OriginalPayload: string(msg.Payload),
		Error:           processingErr.Error(),
		MsgID:           msg.UUID,
		FailedAt:        c.clock.Now().Format(time.RFC3339),
		RetryCount:      c.retryCount[msg.UUID],
	}

	data, err := json.Marshal(dlqPayload)
	if err != nil {
		c.logger.Error("failed to marshal plugin DLQ payload",
			slog.String("msg_id", msg.UUID),
			slog.Any("error", err),
		)
		return
	}

	if c.publisher == nil {
		c.logger.Error("DLQ publisher not configured, plugin async message dropped",
			slog.String("msg_id", msg.UUID),
		)
		return
	}

	dlqMsg := message.NewMessage(watermill.NewUUID(), data)
	if err := c.publisher.PublishRaw(pluginAsyncDLQSubject, dlqMsg); err != nil {
		c.logger.Error("failed to publish plugin async message to DLQ",
			slog.String("msg_id", msg.UUID),
			slog.Any("error", err),
		)
	}
}
