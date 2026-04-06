package nats

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	nc "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

// Dead-letter queue and retry constants.
const (
	// MaxMessageRetries is the maximum number of processing attempts before a
	// message is routed to the dead-letter queue.
	MaxMessageRetries = 3

	// DLQSubjectPrefix is prepended to the original subject when publishing
	// failed messages to the dead-letter stream.
	DLQSubjectPrefix = "dlq."

	// retryKeyPrefix is prepended to the idempotency key to form a separate
	// key that tracks retry attempts in the database. Using a separate key
	// prevents conflicts with the idempotency key lifecycle (acquire/release).
	retryKeyPrefix = "retry:"
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
	OnBindingTrafficExceeded(ctx context.Context, bindingID, subscriptionID string, usedBytes, limitBytes int64) error
	OnBindingTrafficReset(ctx context.Context, bindingID, subscriptionID string) error
	OnTrafficWarning(ctx context.Context, bindingID, subscriptionID string, usedBytes, limitBytes int64, thresholdPct int)
}

// CheckoutCompleter abstracts the billing context's CompleteCheckout operation.
// The BillingEventConsumer uses this to complete checkout asynchronously when
// it receives a payment.charge_completed event from the payment context.
type CheckoutCompleter interface {
	CompleteCheckout(ctx context.Context, invoiceID string) error
}

// NOTE: Subscription and plan lookup interfaces are defined in the multisub
// domain as multisub.PlanProvider and multisub.SubscriptionProvider. The NATS
// adapter (BillingSubscriptionLookup) implements those domain ports.

// IdempotencyChecker provides event-level deduplication and retry tracking.
// The adapter layer owns this interface; the postgres.IdempotencyRepository
// satisfies it.
//
// Keys are the domain event ID (UUIDv7), unique per event instance. This
// deduplicates at the event level rather than the transport level (Watermill
// message UUID). Events without an ID (backward compat) use
// "{event_type}:{entity_id}" as the key.
type IdempotencyChecker interface {
	// TryAcquire returns true if the key is new, false if it was already seen.
	TryAcquire(ctx context.Context, key string) (bool, error)

	// Release removes an idempotency key so that a redelivered message can be
	// processed again. This MUST be called when event processing fails,
	// otherwise the redelivered message will be silently skipped as a
	// duplicate.
	Release(ctx context.Context, key string) error

	// IncrementRetry atomically increments and returns the retry count for
	// the given key. The count is persisted in the database so it survives
	// NATS redeliveries (Watermill metadata is lost across Nack cycles).
	IncrementRetry(ctx context.Context, key string) (int, error)
}

// Consumer lifecycle constants.
const (
	// entityLockTTL is the duration after which an idle entity lock is eligible for
	// eviction. This prevents unbounded growth of the entityLocks map.
	entityLockTTL = 10 * time.Minute

	// ConsumerDrainTimeout is the maximum duration Drain waits for in-flight
	// messages to finish processing before returning. This bounds the shutdown
	// delay when handlers are blocked on slow external calls (e.g. Remnawave).
	ConsumerDrainTimeout = 30 * time.Second

	// DLQDepthPollInterval is how often the consumer queries JetStream for
	// the current DLQ stream message count and updates the DLQDepth gauge.
	DLQDepthPollInterval = 30 * time.Second

	// DefaultMessageProcessingTimeout bounds the time for processing a single
	// event. If a handler exceeds this timeout (e.g., due to a hung DB query
	// or unresponsive Remnawave API), the context is cancelled, the handler
	// returns an error, and the message is Nack'd for retry.
	//
	// Default: 60 seconds — covers DB lookup + Remnawave API call + retry.
	// Configure via BillingEventConsumer.messageTimeout for environments
	// with slower upstream APIs.
	DefaultMessageProcessingTimeout = 60 * time.Second
)

// entityLock serialises event processing for a single entity (e.g. one
// subscription). This prevents race conditions when events like
// subscription.activated and subscription.cancelled arrive on different NATS
// subjects concurrently for the same aggregate.
type entityLock struct {
	mu       sync.Mutex
	lastUsed atomic.Int64 // UnixNano timestamp; atomic to avoid data races with evictor
}

// BillingEventConsumer subscribes to billing domain events on NATS and routes
// them to the SubscriptionEventHandler (MultiSubOrchestrator) for Remnawave
// provisioning and deprovisioning.
//
// Correctness guarantees:
//   - Per-event-instance idempotency: events are deduplicated by the domain
//     event ID (UUIDv7), not Watermill message UUID. This allows legitimate
//     repeated operations on the same aggregate to be processed independently.
//   - Per-entity ordering: events for the same entity are processed serially
//     via entityLocks, while different entities run concurrently.
//   - Retry + DLQ: failed messages are retried up to MaxMessageRetries times;
//     permanently failing messages are sent to the dead-letter queue.
type BillingEventConsumer struct {
	subscriber     *EventSubscriber
	handler        SubscriptionEventHandler
	checkout       CheckoutCompleter
	plans          multisub.PlanProvider
	subs           multisub.SubscriptionProvider
	idempotency    IdempotencyChecker
	publisher      *EventPublisher
	schemaRegistry *domainevent.SchemaRegistry
	logger         *slog.Logger
	clock          clock.Clock
	metrics        *observability.Metrics
	natsConn       *nc.Conn
	messageTimeout time.Duration
	entityLocks    sync.Map // map[string]*entityLock — per-entity serialisation
	inflightWg     sync.WaitGroup
}

// NewBillingEventConsumer creates a BillingEventConsumer with the given
// dependencies. The publisher is used to route permanently failed messages to
// the dead-letter queue. Plan and subscription data are resolved through
// multisub domain ports (PlanProvider + SubscriptionProvider). The checkout
// completer handles payment.charge_completed events by completing the billing
// checkout flow. The schema registry upcasts old event payloads to the latest
// version before processing. The NATS connection is used for DLQ depth polling
// via JetStream API.
func NewBillingEventConsumer(
	subscriber *EventSubscriber,
	handler SubscriptionEventHandler,
	checkout CheckoutCompleter,
	plans multisub.PlanProvider,
	subs multisub.SubscriptionProvider,
	idempotency IdempotencyChecker,
	publisher *EventPublisher,
	schemaRegistry *domainevent.SchemaRegistry,
	logger *slog.Logger,
	clk clock.Clock,
	metrics *observability.Metrics,
	conn *nc.Conn,
) *BillingEventConsumer {
	return &BillingEventConsumer{
		subscriber:     subscriber,
		handler:        handler,
		checkout:       checkout,
		plans:          plans,
		subs:           subs,
		idempotency:    idempotency,
		publisher:      publisher,
		schemaRegistry: schemaRegistry,
		logger:         logger,
		clock:          clk,
		metrics:        metrics,
		natsConn:       conn,
		messageTimeout: DefaultMessageProcessingTimeout,
	}
}

// getEntityLock returns (or creates) the mutex for the given entity ID. This
// ensures that concurrent events targeting the same aggregate are serialised.
func (c *BillingEventConsumer) getEntityLock(entityID string) *entityLock {
	val, _ := c.entityLocks.LoadOrStore(entityID, &entityLock{})
	lock := val.(*entityLock)
	lock.lastUsed.Store(c.clock.Now().UnixNano())
	return lock
}

// evictStaleLocks periodically removes entity locks that have not been used
// within entityLockTTL. This prevents unbounded memory growth in long-running
// processes. After each eviction pass, the EntityLocksActive gauge is updated
// with the number of remaining locks.
func (c *BillingEventConsumer) evictStaleLocks(ctx context.Context) {
	ticker := time.NewTicker(entityLockTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := c.clock.Now().Add(-entityLockTTL).UnixNano()
			c.entityLocks.Range(func(key, value any) bool {
				lock := value.(*entityLock)
				if lock.lastUsed.Load() < cutoff {
					c.entityLocks.Delete(key)
				}
				return true
			})

			var remaining int
			c.entityLocks.Range(func(_, _ any) bool {
				remaining++
				return true
			})
			c.recordMetric(func(m *observability.Metrics) {
				m.EntityLocksActive.Set(float64(remaining))
			})
		}
	}
}

// Drain waits for all in-flight message handlers to complete, bounded by
// ConsumerDrainTimeout. Call Drain after cancelling the context passed to
// Start so that consumeLoop stops reading new messages while existing handlers
// finish. If the timeout expires, a warning is logged — this is a best-effort
// mechanism that avoids blocking shutdown indefinitely.
func (c *BillingEventConsumer) Drain() {
	done := make(chan struct{})
	go func() {
		c.inflightWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		c.logger.Info("billing event consumer: drain complete")
	case <-time.After(ConsumerDrainTimeout):
		c.logger.Warn("billing event consumer: drain timeout, some messages may still be in-flight")
	}
}

// pollDLQDepth periodically queries the JetStream DLQ stream for its current
// message count and updates the DLQDepth Prometheus gauge. This runs as a
// background goroutine started by Start and exits when the context is cancelled.
// If the NATS connection or metrics dependency is nil, the goroutine returns
// immediately to avoid panics in test environments.
func (c *BillingEventConsumer) pollDLQDepth(ctx context.Context) {
	if c.natsConn == nil || c.metrics == nil {
		return
	}

	js, err := jetstream.New(c.natsConn)
	if err != nil {
		c.logger.Warn("billing event consumer: failed to init JetStream for DLQ polling",
			slog.Any("error", err),
		)
		return
	}

	ticker := time.NewTicker(DLQDepthPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stream, streamErr := js.Stream(ctx, StreamDLQ)
			if streamErr != nil {
				c.logger.Debug("billing event consumer: failed to get DLQ stream info",
					slog.Any("error", streamErr),
				)
				continue
			}
			info, infoErr := stream.Info(ctx)
			if infoErr != nil {
				c.logger.Debug("billing event consumer: failed to get DLQ stream info",
					slog.Any("error", infoErr),
				)
				continue
			}
			c.metrics.DLQDepth.Set(float64(info.State.Msgs))
		}
	}
}

// billingSubscriptionSubjects returns the NATS subjects this consumer listens to.
// Includes billing lifecycle events, traffic lifecycle events, and payment
// events that trigger billing-side effects (checkout completion).
func billingSubscriptionSubjects() []string {
	return []string{
		string(billing.EventSubActivated),
		string(billing.EventSubCancelled),
		string(billing.EventSubPaused),
		string(billing.EventSubResumed),
		string(payment.EventChargeCompleted),
		subjectBindingTrafficExceeded,
		subjectTrafficCycleReset,
		subjectTrafficWarning,
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

	// Start background goroutine to evict stale entity locks.
	go c.evictStaleLocks(ctx)

	// Start DLQ depth polling goroutine to keep the DLQDepth gauge current.
	go c.pollDLQDepth(ctx)

	c.logger.Info("billing event consumer started",
		slog.Int("subscribed", subscribed),
		slog.Int("total", len(billingSubscriptionSubjects())),
	)
	return nil
}

// consumeLoop reads messages from a single subscription channel until the
// context is cancelled or the channel is closed. Each in-flight message is
// tracked by inflightWg so that Drain can wait for completion on shutdown.
// When the context is cancelled, the loop stops reading new messages but any
// message already being processed continues with a detached (Background)
// context so that in-flight work is not aborted mid-processing.
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
			c.inflightWg.Add(1)
			// Use context.Background() so that a cancelled parent context does
			// not abort an in-flight handler mid-processing (e.g. during a
			// Remnawave provisioning call). The WaitGroup ensures Drain blocks
			// until this handler completes. The timeout prevents handlers from
			// blocking indefinitely on hung DB queries or unresponsive APIs.
			msgCtx, cancel := context.WithTimeout(context.Background(), c.messageTimeout)
			c.handleMessage(msgCtx, subject, msg)
			cancel()
			c.inflightWg.Done()
		}
	}
}


