package app

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	nc "github.com/nats-io/nats.go"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
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
	fx.Provide(natsadapter.NewOutboxRelay),

	// NATS subscriber (shared by all consumers)
	fx.Provide(func(conn *nc.Conn) (*natsadapter.EventSubscriber, error) {
		return natsadapter.NewEventSubscriber(conn, "remnacore")
	}),

	// Idempotency repository for NATS message deduplication
	fx.Provide(postgres.NewIdempotencyRepository),
	fx.Provide(func(r *postgres.IdempotencyRepository) natsadapter.IdempotencyChecker {
		return r
	}),

	// Billing event consumer
	fx.Provide(natsadapter.NewBillingEventConsumer),

	// Async plugin consumer
	fx.Provide(natsadapter.NewPluginAsyncConsumer),

	// Webhook handler
	fx.Provide(provideWebhookHandler),

	// Lifecycle hooks
	fx.Invoke(startOutboxRelay),
	fx.Invoke(startBillingEventConsumer),
	fx.Invoke(startPluginAsyncConsumer),
)

// provideWebhookHandler creates a Remnawave WebhookHandler that translates
// incoming webhook payloads into domain events and publishes them to NATS.
func provideWebhookHandler(cfg *config.Config, pub *natsadapter.EventPublisher, logger *slog.Logger) *remnawave.WebhookHandler {
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

// startOutboxRelay spawns the transactional outbox relay as a background
// goroutine managed by the Fx lifecycle. The relay polls the outbox table for
// unpublished domain events and forwards them to NATS.
func startOutboxRelay(lc fx.Lifecycle, relay *natsadapter.OutboxRelay, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			relayCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("outbox relay started")
				relay.Run(relayCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("outbox relay stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startBillingEventConsumer starts the NATS consumer that routes billing
// subscription events to the MultiSubOrchestrator for provisioning/deprovisioning.
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
