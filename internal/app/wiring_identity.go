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
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/rbac"
	identityservice "github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity/service"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/txmanager"
)

// identityWiring provides all identity-domain bindings: JWT issuer, identity
// repository, and the shared wall-clock dependency.
var identityWiring = fx.Options(
	// Clock — shared wall-clock dependency injected into domain services
	fx.Provide(func() clock.Clock { return clock.NewReal() }),

	// JWT issuer
	fx.Provide(provideJWTIssuer),

	// SessionIssuer — shared singleton consumed by both identity.Service and
	// IdentityAdminService; handles all token generation and session persistence.
	fx.Provide(provideSessionIssuer),

	// Identity domain service
	fx.Provide(func(repo identity.Repository, pub domainevent.Publisher, txRunner txmanager.Runner, jwt *authutil.JWTIssuer, clk clock.Clock, cfg *config.Config, sessions *identityservice.SessionIssuer) *identity.Service {
		return identity.NewService(repo, pub, txRunner, jwt, clk, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL, sessions)
	}),

	// Identity cleanup scheduler — uses concrete repo type which satisfies
	// the narrow CleanupRepository interface via structural typing.
	fx.Provide(func(repo *postgres.IdentityRepository, logger *slog.Logger) *identityservice.CleanupScheduler {
		return identityservice.NewCleanupScheduler(repo, logger)
	}),

	// Metrics collector — periodic time-series snapshots into metrics.samples.
	fx.Provide(func(pool *pgxpool.Pool, clk clock.Clock, logger *slog.Logger) *identityservice.MetricsCollector {
		return identityservice.NewMetricsCollector(pool, clk, logger)
	}),

	// RBAC repository (interface + impl)
	fx.Provide(postgres.NewRBACRepository),
	fx.Provide(func(repo *postgres.RBACRepository) rbac.Repository { return repo }),

	// Access service — per-request permission resolution + cache.
	fx.Provide(func(repo rbac.Repository) *identityservice.AccessService {
		return identityservice.NewAccessService(repo, time.Now, identityservice.AccessCacheTTL)
	}),

	// RBAC catalog boot sync — idempotent upsert + legacy-role backfill.
	fx.Provide(func(repo rbac.Repository, txRunner txmanager.Runner) *identityservice.RBACCatalogSync {
		return identityservice.NewRBACCatalogSync(repo, txRunner)
	}),

	// ShopProvisioner bridge — adapts ResellerService to the identity port.
	fx.Provide(newShopProvisioner),

	// Account-management orchestration service (invitations, roles, shops).
	fx.Provide(func(repo identity.Repository, rbacRepo rbac.Repository, access *identityservice.AccessService, shops identityservice.ShopProvisioner, sessions *identityservice.SessionIssuer, txRunner txmanager.Runner, pub domainevent.Publisher, clk clock.Clock) *identityservice.IdentityAdminService {
		return identityservice.NewIdentityAdminService(repo, rbacRepo, access, shops, sessions, txRunner, pub, clk)
	}),

	// Lifecycle hooks
	fx.Invoke(startIdentityCleanup),
	fx.Invoke(startMetricsCollector),
	fx.Invoke(startRBACSync),

	// Bindings: interface -> implementation (identity)
	fx.Provide(postgres.NewIdentityRepository),
	fx.Provide(func(repo *postgres.IdentityRepository) identity.Repository { return repo }),
)

// provideSessionIssuer constructs the shared SessionIssuer from the DI graph.
func provideSessionIssuer(repo identity.Repository, pub domainevent.Publisher, jwt *authutil.JWTIssuer, cfg *config.Config) *identityservice.SessionIssuer {
	return identityservice.NewSessionIssuer(repo, pub, jwt, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL)
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

// startIdentityCleanup spawns the identity cleanup scheduler as a background
// goroutine managed by the Fx lifecycle. It periodically removes expired
// sessions, email verifications, and password resets.
func startIdentityCleanup(lc fx.Lifecycle, scheduler *identityservice.CleanupScheduler, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			cleanupCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("identity cleanup scheduler started")
				scheduler.Run(cleanupCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("identity cleanup scheduler stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startMetricsCollector spawns the metrics collector as a background goroutine
// managed by the Fx lifecycle. It periodically snapshots platform metrics into
// metrics.samples for dashboard time-series trends.
func startMetricsCollector(lc fx.Lifecycle, collector *identityservice.MetricsCollector, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			collectorCtx, cancel := context.WithCancel(context.Background())
			go func() {
				logger.Info("metrics collector started")
				collector.Run(collectorCtx)
			}()
			lc.Append(fx.Hook{
				OnStop: func(_ context.Context) error {
					logger.Info("metrics collector stopping")
					cancel()
					return nil
				},
			})
			return nil
		},
	})
}

// startRBACSync runs the idempotent RBAC catalog sync once at startup, before
// the server accepts traffic, so permissions/roles exist for authorization.
// This is a one-shot synchronous OnStart operation — it completes before the
// server begins accepting traffic and requires no OnStop hook (unlike
// startIdentityCleanup and startMetricsCollector, which spawn long-running
// goroutines that must be cancelled on shutdown).
func startRBACSync(lc fx.Lifecycle, sync *identityservice.RBACCatalogSync, logger *slog.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("running RBAC catalog sync")
			return sync.Run(ctx)
		},
	})
}
