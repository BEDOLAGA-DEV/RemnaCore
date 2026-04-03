package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// SubscriptionEventHandler defines the contract for handling billing
// subscription lifecycle events. The MultiSubOrchestrator satisfies this
// interface, keeping the NATS adapter decoupled from the multisub domain.
//
// Plan data is passed as multisub.PlanSnapshot (an Anti-Corruption Layer type)
// so that the handler never depends on billing/aggregate types.
type SubscriptionEventHandler interface {
	OnSubscriptionActivated(
		ctx context.Context,
		subscriptionID string,
		platformUserID string,
		plan multisub.PlanSnapshot,
		addonIDs []string,
		familyMemberIDs []string,
	) error
	OnSubscriptionCancelled(ctx context.Context, subscriptionID string) error
	OnSubscriptionPaused(ctx context.Context, subscriptionID string) error
	OnSubscriptionResumed(ctx context.Context, subscriptionID string) error
}

// SubscriptionLookup provides read access to billing data so the consumer can
// enrich sparse domain events with the full context the orchestrator requires.
// Plan data is returned as multisub.PlanSnapshot, translated at the adapter
// boundary (Anti-Corruption Layer).
type SubscriptionLookup interface {
	GetSubscriptionByID(ctx context.Context, id string) (SubscriptionInfo, error)
	GetPlanSnapshot(ctx context.Context, id string) (multisub.PlanSnapshot, error)
	GetFamilyMemberIDs(ctx context.Context, ownerID string) ([]string, error)
}

// SubscriptionInfo holds the minimal subscription data the consumer needs.
type SubscriptionInfo struct {
	ID       string
	UserID   string
	PlanID   string
	AddonIDs []string
}

// IdempotencyChecker provides message-level deduplication. The adapter layer
// owns this interface; the postgres.IdempotencyRepository satisfies it.
type IdempotencyChecker interface {
	// TryAcquire returns true if the key is new, false if it was already seen.
	TryAcquire(ctx context.Context, key string) (bool, error)
}

// BillingEventConsumer subscribes to billing domain events on NATS and routes
// them to the SubscriptionEventHandler (MultiSubOrchestrator) for Remnawave
// provisioning and deprovisioning.
type BillingEventConsumer struct {
	subscriber  *EventSubscriber
	handler     SubscriptionEventHandler
	lookup      SubscriptionLookup
	idempotency IdempotencyChecker
	logger      *slog.Logger
}

// NewBillingEventConsumer creates a BillingEventConsumer with the given
// dependencies.
func NewBillingEventConsumer(
	subscriber *EventSubscriber,
	handler SubscriptionEventHandler,
	lookup SubscriptionLookup,
	idempotency IdempotencyChecker,
	logger *slog.Logger,
) *BillingEventConsumer {
	return &BillingEventConsumer{
		subscriber:  subscriber,
		handler:     handler,
		lookup:      lookup,
		idempotency: idempotency,
		logger:      logger,
	}
}

// billingSubscriptionSubjects returns the NATS subjects this consumer listens to.
func billingSubscriptionSubjects() []string {
	return []string{
		"subscription.activated",
		"subscription.cancelled",
		"subscription.paused",
		"subscription.resumed",
	}
}

// Start subscribes to billing subscription events and processes them in
// background goroutines. It returns immediately; the goroutines run until the
// context is cancelled.
func (c *BillingEventConsumer) Start(ctx context.Context) error {
	subscribed := 0
	for _, subject := range billingSubscriptionSubjects() {
		ch, err := c.subscriber.Subscribe(ctx, subject)
		if err != nil {
			c.logger.Warn("failed to subscribe to billing event, will retry on next restart",
				slog.String("subject", subject),
				slog.String("error", err.Error()),
			)
			continue
		}

		go c.consumeLoop(ctx, subject, ch)
		subscribed++
	}

	c.logger.Info("billing event consumer started",
		slog.Int("subscribed", subscribed),
		slog.Int("total", len(billingSubscriptionSubjects())),
	)
	return nil
}

// consumeLoop reads messages from a single subscription channel until the
// context is cancelled or the channel is closed.
func (c *BillingEventConsumer) consumeLoop(ctx context.Context, subject string, ch <-chan *message.Message) {
	c.logger.Info("billing event consumer started", slog.String("subject", subject))

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			c.handleMessage(ctx, subject, msg)
		}
	}
}

// handleMessage parses and routes a single billing event message. It performs
// message-level deduplication via the idempotency checker before dispatching
// to the handler.
func (c *BillingEventConsumer) handleMessage(ctx context.Context, subject string, msg *message.Message) {
	defer msg.Ack()

	// Dedup by Watermill message UUID. If the idempotency check fails (DB
	// error), we fail open — at-least-once delivery is safer than dropping.
	isNew, err := c.idempotency.TryAcquire(ctx, msg.UUID)
	if err != nil {
		c.logger.Warn("idempotency check failed, processing message anyway",
			slog.String("msg_id", msg.UUID),
			slog.String("subject", subject),
			slog.Any("error", err),
		)
	} else if !isNew {
		c.logger.Debug("duplicate message, skipping",
			slog.String("msg_id", msg.UUID),
			slog.String("subject", subject),
		)
		return
	}

	var event domainevent.Event
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		c.logger.Error("failed to unmarshal billing event",
			slog.String("subject", subject),
			slog.Any("error", err),
		)
		return
	}

	var handleErr error

	switch subject {
	case "subscription.activated":
		handleErr = c.handleActivated(ctx, event)
	case "subscription.cancelled":
		handleErr = c.handleSimple(ctx, event, c.handler.OnSubscriptionCancelled)
	case "subscription.paused":
		handleErr = c.handleSimple(ctx, event, c.handler.OnSubscriptionPaused)
	case "subscription.resumed":
		handleErr = c.handleSimple(ctx, event, c.handler.OnSubscriptionResumed)
	default:
		c.logger.Warn("unhandled billing event subject", slog.String("subject", subject))
		return
	}

	if handleErr != nil {
		c.logger.Error("failed to handle billing event",
			slog.String("subject", subject),
			slog.String("subscription_id", extractString(event.Data, "subscription_id")),
			slog.Any("error", handleErr),
		)
	}
}

// handleActivated enriches the sparse activated event with subscription, plan,
// and family data before dispatching to the orchestrator.
func (c *BillingEventConsumer) handleActivated(ctx context.Context, event domainevent.Event) error {
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return fmt.Errorf("subscription_id missing from event data")
	}

	subInfo, err := c.lookup.GetSubscriptionByID(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("lookup subscription %s: %w", subscriptionID, err)
	}

	plan, err := c.lookup.GetPlanSnapshot(ctx, subInfo.PlanID)
	if err != nil {
		return fmt.Errorf("lookup plan %s: %w", subInfo.PlanID, err)
	}

	familyMemberIDs, err := c.lookup.GetFamilyMemberIDs(ctx, subInfo.UserID)
	if err != nil {
		c.logger.Warn("failed to lookup family members, proceeding without",
			slog.String("user_id", subInfo.UserID),
			slog.Any("error", err),
		)
		familyMemberIDs = nil
	}

	return c.handler.OnSubscriptionActivated(
		ctx,
		subInfo.ID,
		subInfo.UserID,
		plan,
		subInfo.AddonIDs,
		familyMemberIDs,
	)
}

// handleSimple handles events that only require a subscription_id.
func (c *BillingEventConsumer) handleSimple(
	ctx context.Context,
	event domainevent.Event,
	fn func(ctx context.Context, subscriptionID string) error,
) error {
	subscriptionID := extractString(event.Data, "subscription_id")
	if subscriptionID == "" {
		return fmt.Errorf("subscription_id missing from event data")
	}

	return fn(ctx, subscriptionID)
}

// extractString safely extracts a string value from a map[string]any.
func extractString(data map[string]any, key string) string {
	if data == nil {
		return ""
	}
	v, ok := data[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
