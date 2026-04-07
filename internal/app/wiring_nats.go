package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.uber.org/fx"

	"github.com/jackc/pgx/v5/pgxpool"
	nc "github.com/nats-io/nats.go"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	billingaggregate "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/aggregate"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/health"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// natsWiring provides NATS-related bindings: publisher, subscriber, outbox
// relay, event consumers, idempotency checker, transaction manager, and the
// Remnawave webhook handler.
var natsWiring = fx.Options(
	// Transaction manager: wraps business writes + outbox inserts in a
	// single database transaction, preventing event loss on crashes.
	fx.Provide(postgres.NewTxManager),
	fx.Provide(func(tm *postgres.TxManager) txmanager.Runner { return tm }),

	// Transactional outbox: domain events are written to the outbox table
	// (same DB transaction as business logic) and relayed to NATS asynchronously.
	fx.Provide(postgres.NewOutboxRepository),
	fx.Provide(postgres.NewOutboxPublisher),
	fx.Provide(func(pub *postgres.OutboxPublisher, metrics *observability.Metrics) domainevent.Publisher {
		return observability.NewMeteredPublisher(pub, metrics)
	}),
	fx.Provide(func(
		outbox *postgres.OutboxRepository,
		publisher *natsadapter.EventPublisher,
		txRunner txmanager.Runner,
		clk clock.Clock,
		logger *slog.Logger,
		cfg *config.Config,
		metrics *observability.Metrics,
		conn *nc.Conn,
	) *natsadapter.OutboxRelay {
		return natsadapter.NewOutboxRelay(outbox, publisher, txRunner, clk, logger, cfg.Outbox.RelayWorkers, cfg.CircuitBreaker.OutboxNATS, metrics, conn)
	}),

	// Outbox health checker: reports degraded when backlog exceeds threshold.
	fx.Provide(
		fx.Annotate(
			func(outbox *postgres.OutboxRepository) health.Checker {
				return natsadapter.NewOutboxHealthChecker(outbox)
			},
			fx.ResultTags(`group:"health.checkers"`),
		),
	),

	// NATS subscriber (shared by all consumers)
	fx.Provide(func(conn *nc.Conn) (*natsadapter.EventSubscriber, error) {
		return natsadapter.NewEventSubscriber(conn, "remnacore")
	}),

	// Idempotency repository for NATS message deduplication
	fx.Provide(postgres.NewIdempotencyRepository),
	fx.Provide(func(r *postgres.IdempotencyRepository) natsadapter.IdempotencyChecker {
		return r
	}),

	// Schema registry: upcasts old event payloads to the latest version
	// before consumers process them. Register upcasters here as event
	// schemas evolve.
	fx.Provide(func() *domainevent.SchemaRegistry {
		r := domainevent.NewSchemaRegistry()
		// Reference upcasters: subscription.activated v1->v2
		r.Register(billingaggregate.EventSubActivated, billingaggregate.SubActivatedV1ToV2{})
		return r
	}),

	// CheckoutCompleter: billing's CheckoutService satisfies the adapter's
	// CheckoutCompleter interface, used by the billing event consumer to
	// handle payment.charge_completed events asynchronously.
	fx.Provide(func(cs *billingservice.CheckoutService) natsadapter.CheckoutCompleter { return cs }),

	// Billing event consumer
	fx.Provide(natsadapter.NewBillingEventConsumer),

	// Async plugin consumer
	fx.Provide(natsadapter.NewPluginAsyncConsumer),

	// Webhook handler — provided as named http.Handler to avoid gateway→adapter import.
	fx.Provide(fx.Annotate(provideWebhookHandler, fx.ResultTags(`name:"remnawave_webhook"`))),

	// Partition manager: ensures future partitions + cleans up expired ones.
	fx.Provide(func(
		outbox *postgres.OutboxRepository,
		pool *pgxpool.Pool,
		clk clock.Clock,
		logger *slog.Logger,
		cfg *config.Config,
	) *postgres.PartitionManager {
		var retention time.Duration
		if cfg.Outbox.RetentionDays > 0 {
			retention = time.Duration(cfg.Outbox.RetentionDays) * 24 * time.Hour
		}
		return postgres.NewPartitionManager(outbox, pool, clk, logger, cfg.Outbox.PartitionLookahead, retention)
	}),

	// Lifecycle hooks
	fx.Invoke(startPartitionManager),
	fx.Invoke(startOutboxRelay),
	fx.Invoke(startBillingEventConsumer),
	fx.Invoke(startPluginAsyncConsumer),
)

// provideWebhookHandler creates a Remnawave WebhookHandler that translates
// incoming webhook payloads into domain events and publishes them to NATS.
func provideWebhookHandler(cfg *config.Config, pub *natsadapter.EventPublisher, logger *slog.Logger) http.Handler {
	return remnawave.NewWebhookHandler(cfg.Remnawave.WebhookSecret.Expose(), func(payload remnawave.WebhookPayload) {
		domainEvent := remnawave.MapWebhookEvent(payload.Scope, payload.Event)
		logger.Info("remnawave webhook received",
			slog.String("scope", payload.Scope),
			slog.String("event", payload.Event),
			slog.String("domain_event", domainEvent),
		)
		if err := pub.Publish(context.Background(), domainEvent, payload); err != nil {
			logger.Error("failed to publish webhook event",
				slog.String("domain_event", domainEvent),
				slog.Any("error", err),
			)
		}
	})
}

// startPartitionManager spawns the PartitionManager as a background goroutine
// managed by the Fx lifecycle. It pre-creates future quarterly partitions and
// cleans up expired ones on a daily schedule.
//
// On shutdown, the context is cancelled and the hook waits for Run to return
// before proceeding. See shutdown.go Phase 5.
func startPartitionManager(lc fx.Lifecycle, pm *postgres.PartitionManager, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			pmCtx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				logger.Info("partition manager started")
				pm.Run(pmCtx)
				close(done)
			}()
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					logger.Info("partition manager stopping")
					cancel()

					select {
					case <-done:
						logger.Info("partition manager stopped")
					case <-ctx.Done():
						logger.Warn("partition manager shutdown timeout")
					}
					return nil
				},
			})
			return nil
		},
	})
}

// startOutboxRelay spawns the transactional outbox relay as a background
// goroutine managed by the Fx lifecycle. The relay polls the outbox table for
// unpublished domain events and forwards them to NATS.
//
// On shutdown, the context is cancelled and the hook WAITS for Run to return
// (all workers + cleanup goroutines finished) before proceeding. This ensures
// no in-flight batches are interrupted when Fx closes the NATS connection in
// the next phase. See shutdown.go Phase 4.
func startOutboxRelay(lc fx.Lifecycle, relay *natsadapter.OutboxRelay, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			relayCtx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				logger.Info("outbox relay started")
				relay.Run(relayCtx)
				close(done)
			}()
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					logger.Info("outbox relay stopping")
					cancel()

					// Wait for all relay workers to finish, bounded by
					// ShutdownPhaseRelay timeout.
					shutdownCtx, shutdownCancel := context.WithTimeout(ctx, ShutdownPhaseRelay)
					defer shutdownCancel()

					select {
					case <-done:
						logger.Info("outbox relay stopped")
					case <-shutdownCtx.Done():
						logger.Warn("outbox relay shutdown timeout, some workers may still be running")
					}
					return nil
				},
			})
			return nil
		},
	})
}

// startBillingEventConsumer starts the NATS consumer that routes billing
// subscription events to the MultiSubOrchestrator for provisioning/deprovisioning.
// On shutdown, the context is cancelled (stopping new message reads) and Drain
// is called to wait for in-flight handlers to complete before the process exits.
func startBillingEventConsumer(lc fx.Lifecycle, consumer *natsadapter.BillingEventConsumer, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			consumerCtx, cancel := context.WithCancel(context.Background())
			if err := consumer.Start(consumerCtx); err != nil {
				cancel()
				return fmt.Errorf("failed to start billing event consumer: %w", err)
			}
			logger.Info("billing event consumer started")
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("billing event consumer stopping")
					cancel()
					consumer.Drain()
					return nil
				},
			})
			return nil
		},
	})
}

// startPluginAsyncConsumer starts the NATS consumer that processes async plugin
// hook events. It manages the consumer lifecycle via the Fx lifecycle hooks.
func startPluginAsyncConsumer(lc fx.Lifecycle, consumer *natsadapter.PluginAsyncConsumer, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			consumerCtx, cancel := context.WithCancel(context.Background())
			if err := consumer.Start(consumerCtx); err != nil {
				cancel()
				return fmt.Errorf("failed to start async plugin consumer: %w", err)
			}
			logger.Info("async plugin consumer started")
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("async plugin consumer stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}
