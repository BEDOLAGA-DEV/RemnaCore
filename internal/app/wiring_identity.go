package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"go.uber.org/fx"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/identity"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// identityWiring provides all identity-domain bindings: JWT issuer, identity
// repository, and the shared wall-clock dependency.
var identityWiring = fx.Options(
	// Clock — shared wall-clock dependency injected into domain services
	fx.Provide(func() clock.Clock { return clock.NewReal() }),

	// JWT issuer
	fx.Provide(provideJWTIssuer),

	// Identity domain module
	identity.Module,

	// Bindings: interface -> implementation (identity)
	fx.Provide(postgres.NewIdentityRepository),
	fx.Provide(func(repo *postgres.IdentityRepository) identity.Repository { return repo }),
)

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
