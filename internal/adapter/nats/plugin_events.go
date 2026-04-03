package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/sdk"
)

// PluginAsyncConsumer subscribes to plugin.hook.* topics on NATS JetStream and
// dispatches each message to the appropriate plugin runtime handlers.
type PluginAsyncConsumer struct {
	subscriber *EventSubscriber
	dispatcher *plugin.HookDispatcher
	runtime    *plugin.RuntimePool
	logger     *slog.Logger
}

// NewPluginAsyncConsumer creates a consumer that bridges NATS async hook events
// to the plugin runtime pool.
func NewPluginAsyncConsumer(
	subscriber *EventSubscriber,
	dispatcher *plugin.HookDispatcher,
	runtime *plugin.RuntimePool,
	logger *slog.Logger,
) *PluginAsyncConsumer {
	return &PluginAsyncConsumer{
		subscriber: subscriber,
		dispatcher: dispatcher,
		runtime:    runtime,
		logger:     logger,
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
func (c *PluginAsyncConsumer) handleMessage(ctx context.Context, msg *message.Message) {
	defer msg.Ack()

	var envelope asyncHookPayload
	if err := json.Unmarshal(msg.Payload, &envelope); err != nil {
		c.logger.Error("failed to unmarshal async hook payload", slog.Any("error", err))
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
		c.logger.Warn("async hook message missing hook name, skipping")
		return
	}

	// Look up all registered async handlers for this hook.
	regs := c.dispatcher.Registrations(hookName)
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
				"hook", hookName, "plugin", reg.PluginSlug, "error", err)
			continue
		}

		if _, err := c.runtime.CallHook(ctx, reg.PluginSlug, reg.FuncName, input); err != nil {
			c.logger.Error("async hook execution failed",
				"hook", hookName, "plugin", reg.PluginSlug, "error", err)
		}
	}
}
