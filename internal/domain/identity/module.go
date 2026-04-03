package identity

import (
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/config"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/authutil"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
	"go.uber.org/fx"
)

// Module provides the identity domain Service to the Fx dependency graph.
var Module = fx.Module("identity",
	fx.Provide(func(repo Repository, pub domainevent.Publisher, jwt *authutil.JWTIssuer, clk clock.Clock, cfg *config.Config) *Service {
		return NewService(repo, pub, jwt, clk, cfg.JWT.AccessTokenTTL, cfg.JWT.RefreshTokenTTL)
	}),
)
