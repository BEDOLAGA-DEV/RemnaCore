package nats

import (
	"context"
	"fmt"
	"log/slog"

	nc "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
)

// MaxReconnects is set to -1 so the client retries indefinitely.
const MaxReconnects = -1

// Module provides the NATS connection, event publisher, and stream
// provisioning to the Fx dependency graph.
var Module = fx.Module("nats",
	fx.Provide(NewConnection),
	fx.Provide(NewEventPublisher),
	fx.Invoke(EnsureStreams),
)

// NewConnection dials the NATS server described in cfg and registers lifecycle
// hooks to close the connection on shutdown.
func NewConnection(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) (*nc.Conn, error) {
	conn, err := nc.Connect(
		cfg.NATS.URL,
		nc.RetryOnFailedConnect(true),
		nc.MaxReconnects(MaxReconnects),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS at %s: %w", cfg.NATS.URL, err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			conn.Close()
			return nil
		},
	})

	logger.Info("nats connection established", slog.String("url", cfg.NATS.URL))

	return conn, nil
}

// EnsureStreams creates or updates every JetStream stream the platform needs.
// The operation is idempotent: existing streams whose configuration matches are
// left untouched, and those that differ are updated in place.
func EnsureStreams(conn *nc.Conn, logger *slog.Logger) error {
	js, err := jetstream.New(conn)
	if err != nil {
		return fmt.Errorf("initialising JetStream context: %w", err)
	}

	for _, cfg := range StreamConfigs() {
		if _, err := js.CreateOrUpdateStream(context.Background(), cfg); err != nil {
			return fmt.Errorf("ensuring stream %s: %w", cfg.Name, err)
		}
		logger.Info("jetstream stream ensured", slog.String("stream", cfg.Name))
	}

	return nil
}
