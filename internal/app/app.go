// Package app wires all Fx modules together into a single application.
package app

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"go.uber.org/fx"

	"github.com/jackc/pgx/v5/pgxpool"

	natsadapter "github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/nats"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/valkey"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing"
	billingservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	multisubservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/payment"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/reseller"

	nc "github.com/nats-io/nats.go"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/gateway"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/infra"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/observability"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/telegram"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/hookdispatch"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// httpShutdownTimeout is the maximum time allowed for the HTTP server to
// complete in-flight requests during graceful shutdown.
const httpShutdownTimeout = 10 * time.Second

// New constructs the Fx application with all modules wired together.
func New() *fx.App {
	return fx.New(
		// Config
		fx.Provide(config.Load),

		// Observability
		observability.Module,

		// Adapters
		postgres.Module,
		valkey.Module,
		natsadapter.Module,
		remnawave.Module,

		// Clock — shared wall-clock dependency injected into domain services
		fx.Provide(func() clock.Clock { return clock.NewReal() }),

		// JWT issuer
		fx.Provide(provideJWTIssuer),

		// Identity domain
		identity.Module,

		// Bindings: interface -> implementation (identity)
		fx.Provide(func(repo *postgres.IdentityRepository) identity.Repository { return repo }),
		fx.Provide(postgres.NewIdentityRepository),

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

		// Billing domain
		billingservice.Module,

		// Billing -> Payment ACL: billing.PaymentGateway wraps *payment.PaymentFacade
		// so that the billing domain never imports the payment domain directly.
		fx.Provide(newPaymentGatewayAdapter),

		// Billing repos -> interface bindings
		fx.Provide(postgres.NewPlanRepository),
		fx.Provide(func(repo *postgres.PlanRepository) billing.PlanRepository { return repo }),
		fx.Provide(postgres.NewSubscriptionRepository),
		fx.Provide(func(repo *postgres.SubscriptionRepository) billing.SubscriptionRepository { return repo }),
		fx.Provide(postgres.NewInvoiceRepository),
		fx.Provide(func(repo *postgres.InvoiceRepository) billing.InvoiceRepository { return repo }),
		fx.Provide(postgres.NewFamilyRepository),
		fx.Provide(func(repo *postgres.FamilyRepository) billing.FamilyRepository { return repo }),

		// MultiSub domain
		multisubservice.Module,

		// MultiSub repos -> interface bindings
		fx.Provide(postgres.NewBindingRepository),
		fx.Provide(func(repo *postgres.BindingRepository) multisub.BindingRepository { return repo }),

		// Remnawave gateway -> interface binding
		fx.Provide(remnawave.NewGatewayAdapter),
		fx.Provide(func(adapter *remnawave.GatewayAdapter) multisub.RemnawaveGateway { return adapter }),

		// MultiSub orchestrator -> billing event handler interface
		fx.Provide(func(o *multisubservice.MultiSubOrchestrator) natsadapter.SubscriptionEventHandler {
			return o
		}),

		// NATS subscriber (shared by all consumers)
		fx.Provide(func(conn *nc.Conn) (*natsadapter.EventSubscriber, error) {
			return natsadapter.NewEventSubscriber(conn, "remnacore")
		}),

		// Idempotency repository for NATS message deduplication
		fx.Provide(postgres.NewIdempotencyRepository),
		fx.Provide(func(r *postgres.IdempotencyRepository) natsadapter.IdempotencyChecker {
			return r
		}),

		// Billing event consumer dependencies
		fx.Provide(natsadapter.NewBillingSubscriptionLookup),
		fx.Provide(func(l *natsadapter.BillingSubscriptionLookup) natsadapter.SubscriptionLookup {
			return l
		}),
		fx.Provide(natsadapter.NewBillingEventConsumer),
		fx.Provide(natsadapter.NewPluginAsyncConsumer),

		// Webhook handler
		fx.Provide(provideWebhookHandler),

		// Reseller domain
		reseller.Module,

		// Reseller repos -> interface bindings
		fx.Provide(postgres.NewResellerRepository),
		fx.Provide(func(repo *postgres.ResellerRepository) reseller.TenantRepository { return repo }),
		fx.Provide(func(repo *postgres.ResellerRepository) reseller.CommissionRepository { return repo }),

		// Payment domain
		payment.Module,

		// Payment repos -> interface bindings
		fx.Provide(postgres.NewPaymentRepository),
		fx.Provide(func(repo *postgres.PaymentRepository) payment.PaymentRepository { return repo }),

		// Plugin domain
		plugin.Module,

		// Hook dispatcher -> interface binding (domain packages depend on the interface)
		fx.Provide(func(d *plugin.HookDispatcher) hookdispatch.Dispatcher { return d }),

		// Plugin repos -> interface bindings
		fx.Provide(postgres.NewPluginRepository),
		fx.Provide(func(repo *postgres.PluginRepository) plugin.PluginRepository { return repo }),
		fx.Provide(func(pool *pgxpool.Pool) plugin.StorageService {
			return postgres.NewPluginStorageRepository(pool, plugin.DefaultMaxStorageMB)
		}),

		// WASM runner factory — real Extism/wazero runtime.
		fx.Provide(provideExtismWASMFactory),

		// Infrastructure services (health monitor, smart router, speed test, subscription proxy)
		infra.Module,

		// Gateway
		gateway.Module,

		// Telegram bot
		telegram.Module,

		// OpenTelemetry tracing lifecycle
		fx.Invoke(startTracing),

		// HTTP server
		fx.Invoke(startHTTPServer),

		// Telegram bot lifecycle
		fx.Invoke(startTelegramBot),

		// Periodic sync
		fx.Invoke(startSyncService),

		// Binding reconciler (cleans up orphaned Remnawave users from failed compensations)
		fx.Invoke(startBindingReconciler),

		// Load enabled plugins on startup
		fx.Invoke(loadEnabledPlugins),

		// Outbox relay (polls outbox table, publishes to NATS)
		fx.Invoke(startOutboxRelay),

		// Start billing event consumer (routes to MultiSubOrchestrator)
		fx.Invoke(startBillingEventConsumer),

		// Start async plugin consumer
		fx.Invoke(startPluginAsyncConsumer),

		// Infrastructure service lifecycle hooks
		fx.Invoke(startHealthMonitor),
		fx.Invoke(startSpeedTest),
		fx.Invoke(startSubscriptionProxy),
	)
}

// provideJWTIssuer loads the ECDSA private key from the configured path. If the
// file does not exist it generates an ephemeral P-256 key pair suitable for
// development and logs a warning.
func provideJWTIssuer(cfg *config.Config, logger *slog.Logger) (*authutil.JWTIssuer, error) {
	privateKey, err := loadECDSAPrivateKey(cfg.JWT.PrivateKeyPath)
	if err == nil {
		publicKey := &privateKey.PublicKey
		// If a separate public key path is configured, prefer it.
		if cfg.JWT.PublicKeyPath != "" {
			pub, pubErr := loadECDSAPublicKey(cfg.JWT.PublicKeyPath)
			if pubErr != nil {
				return nil, fmt.Errorf("loading public key from %s: %w", cfg.JWT.PublicKeyPath, pubErr)
			}
			publicKey = pub
		}
		logger.Info("jwt issuer initialised with key file", slog.String("path", cfg.JWT.PrivateKeyPath))
		return authutil.NewJWTIssuer(privateKey, publicKey), nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("loading private key from %s: %w", cfg.JWT.PrivateKeyPath, err)
	}

	// File does not exist — generate an ephemeral key for development.
	logger.Warn("jwt private key file not found, generating ephemeral P-256 key (NOT FOR PRODUCTION)",
		slog.String("path", cfg.JWT.PrivateKeyPath),
	)

	ephemeral, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ephemeral ECDSA key: %w", err)
	}

	return authutil.NewJWTIssuer(ephemeral, &ephemeral.PublicKey), nil
}

// loadECDSAPrivateKey reads a PEM-encoded EC private key from disk.
func loadECDSAPrivateKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing EC private key: %w", err)
	}

	return key, nil
}

// loadECDSAPublicKey reads a PEM-encoded EC public key from disk.
func loadECDSAPublicKey(path string) (*ecdsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	ecPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key is not ECDSA")
	}

	return ecPub, nil
}

// provideWebhookHandler creates a Remnawave WebhookHandler that translates
// incoming webhook payloads into domain events and publishes them to NATS.
func provideWebhookHandler(cfg *config.Config, pub *natsadapter.EventPublisher, logger *slog.Logger) *remnawave.WebhookHandler {
	return remnawave.NewWebhookHandler(cfg.Remnawave.WebhookSecret, func(payload remnawave.WebhookPayload) {
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

// startTracing initialises the OpenTelemetry tracer provider on start and
// flushes pending spans on stop. When no TRACING_ENDPOINT is configured a noop
// provider is used and the shutdown function is a harmless no-op.
func startTracing(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) {
	var shutdown observability.TracerShutdownFunc

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			var err error
			shutdown, err = observability.InitTracer(ctx, cfg, logger)
			if err != nil {
				return fmt.Errorf("init tracer: %w", err)
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if shutdown == nil {
				return nil
			}
			logger.Info("tracer provider shutting down")
			shutdownCtx, cancel := context.WithTimeout(ctx, observability.TracerShutdownTimeout)
			defer cancel()
			return shutdown(shutdownCtx)
		},
	})
}

// startHTTPServer registers an HTTP server that starts listening on OnStart and
// shuts down gracefully on OnStop.
func startHTTPServer(lc fx.Lifecycle, router http.Handler, cfg *config.Config, logger *slog.Logger) {
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("binding to %s: %w", addr, err)
			}
			logger.Info("http server starting", slog.String("addr", addr))
			go func() {
				if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
					logger.Error("http server error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("http server shutting down")
			shutdownCtx, cancel := context.WithTimeout(ctx, httpShutdownTimeout)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	})
}

// provideExtismWASMFactory returns a WASMRunnerFactory backed by the Extism Go
// SDK (wazero runtime). Plugins are loaded with WASI support and receive their
// config via the Extism manifest config mechanism.
func provideExtismWASMFactory() plugin.WASMRunnerFactory {
	return plugin.ExtismRunnerFactory()
}

// loadEnabledPlugins bootstraps the plugin runtime by loading every plugin that
// was enabled before the last shutdown.
func loadEnabledPlugins(lc fx.Lifecycle, lm *plugin.LifecycleManager, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := lm.LoadAllEnabled(ctx); err != nil {
				logger.Error("failed to load enabled plugins on startup", slog.Any("error", err))
				// Non-fatal — the platform can still operate without plugins.
			}
			return nil
		},
	})
}

// startSyncService spawns the periodic Remnawave binding sync as a background
// goroutine managed by the Fx lifecycle.
func startSyncService(lc fx.Lifecycle, syncService *multisubservice.SyncService, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Create a long-lived context that is cancelled on shutdown.
			syncCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("periodic sync service started")
				syncService.RunPeriodicSync(syncCtx)
			}()
			// Store cancel for OnStop via closure.
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("periodic sync service stopping")
					cancel()
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

// startTelegramBot starts the Telegram bot as a background goroutine managed
// by the Fx lifecycle.
func startTelegramBot(lc fx.Lifecycle, bot *telegram.Bot, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			botCtx, cancel := context.WithCancel(context.Background())
			go func() {
				if err := bot.Start(botCtx); err != nil {
					logger.Error("telegram bot error", slog.Any("error", err))
				}
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("telegram bot stopping")
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

// startHealthMonitor runs the node health monitor as a background goroutine.
func startHealthMonitor(lc fx.Lifecycle, hm *infra.HealthMonitor, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			hmCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("health monitor started")
				hm.Run(hmCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("health monitor stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startSpeedTest runs the speed test server on its dedicated port.
func startSpeedTest(lc fx.Lifecycle, st *infra.SpeedTestServer, cfg *config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			stCtx, cancel := context.WithCancel(context.Background())
			go func() {
				port := cfg.Infra.SpeedTestPort
				if port == 0 {
					port = infra.SpeedTestPort
				}
				if err := st.Start(stCtx, port); err != nil {
					logger.Error("speed test server error", slog.Any("error", err))
				}
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("speed test server stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startBindingReconciler spawns the orphaned Remnawave user reconciler as a
// background goroutine managed by the Fx lifecycle. It periodically cleans up
// ghost Remnawave users left behind by failed saga compensations.
func startBindingReconciler(lc fx.Lifecycle, reconciler *multisubservice.BindingReconciler, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			recCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("binding reconciler started")
				reconciler.Run(recCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("binding reconciler stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startSubscriptionProxy runs the subscription proxy on its dedicated port.
func startSubscriptionProxy(lc fx.Lifecycle, sp *infra.SubscriptionProxy, cfg *config.Config, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			spCtx, cancel := context.WithCancel(context.Background())
			go func() {
				port := cfg.Infra.SubscriptionProxyPort
				if port == 0 {
					port = infra.SubscriptionProxyPort
				}
				if err := sp.Start(spCtx, port); err != nil {
					logger.Error("subscription proxy error", slog.Any("error", err))
				}
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("subscription proxy stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}
