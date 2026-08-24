package remnawave

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/clock"
)

// DefaultBindingExpiryMonths is the number of months added to the current time
// when no explicit expiry is provided for a Remnawave user.
const DefaultBindingExpiryMonths = 1

// GatewayAdapter implements multisub.RemnawaveGateway, translating between
// domain port types and Remnawave client types. This is the Anti-Corruption
// Layer boundary; no remnawave client types leak into the domain.
type GatewayAdapter struct {
	client                *ResilientClient
	clock                 clock.Clock
	defaultInternalSquads []string
	logger                *slog.Logger
}

// NewGatewayAdapter creates a GatewayAdapter backed by the given resilient
// client. defaultInternalSquads is applied to every CreateUser call that does
// not supply its own ActiveInternalSquads override.
func NewGatewayAdapter(client *ResilientClient, clk clock.Clock, defaultInternalSquads []string, logger *slog.Logger) *GatewayAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &GatewayAdapter{client: client, clock: clk, defaultInternalSquads: defaultInternalSquads, logger: logger}
}

// CreateUser provisions a VPN user in Remnawave, translating the domain
// request into a Remnawave API call and mapping the result back.
func (a *GatewayAdapter) CreateUser(ctx context.Context, req multisub.CreateRemnawaveUserRequest) (*multisub.RemnawaveUserResult, error) {
	rwReq := CreateUserRequest{
		Username:          req.Username,
		TrafficLimitBytes: float64(req.TrafficLimitBytes),
	}
	rwReq.ActiveInternalSquads = req.ActiveInternalSquads
	if len(rwReq.ActiveInternalSquads) == 0 {
		rwReq.ActiveInternalSquads = a.defaultInternalSquads
	}
	if len(rwReq.ActiveInternalSquads) == 0 {
		// A user with no internal squads gets an EMPTY, non-working subscription
		// (no inbounds). This is almost always a misconfiguration — warn loudly
		// per provision, not just once at boot, so it surfaces in operations.
		a.logger.WarnContext(ctx, "remnawave: provisioning VPN user with NO internal squads — subscription will not work; set REMNAWAVE_DEFAULT_INTERNAL_SQUADS or supply squads per request",
			slog.String("username", req.Username))
	}
	if req.ExpireAt != nil {
		rwReq.ExpireAt = *req.ExpireAt
	} else {
		// Default: 1 month from now if no expiry specified.
		rwReq.ExpireAt = a.clock.Now().AddDate(0, DefaultBindingExpiryMonths, 0)
	}

	user, err := a.client.CreateUser(ctx, rwReq)
	if err != nil {
		return nil, fmt.Errorf("remnawave create user: %w", err)
	}

	return &multisub.RemnawaveUserResult{
		UUID:      user.UserRef(),
		ShortUUID: user.ShortUUID,
	}, nil
}

// GetUser retrieves the current status of a Remnawave VPN user.
func (a *GatewayAdapter) GetUser(ctx context.Context, remnawaveUUID string) (*multisub.RemnawaveUserStatus, error) {
	user, err := a.client.GetUserByUUID(ctx, remnawaveUUID)
	if err != nil {
		return nil, fmt.Errorf("remnawave get user: %w", err)
	}

	return &multisub.RemnawaveUserStatus{
		UUID:      user.UserRef(),
		Enabled:   user.Status == RemnawaveStatusActive,
		Expired:   user.Status == RemnawaveStatusExpired || user.Status == RemnawaveStatusLimited,
		UsedBytes: user.UsedTrafficBytesInt(),
	}, nil
}

// DeleteUser removes a VPN user from Remnawave.
func (a *GatewayAdapter) DeleteUser(ctx context.Context, remnawaveUUID string) error {
	if err := a.client.DeleteUser(ctx, remnawaveUUID); err != nil {
		return fmt.Errorf("remnawave delete user: %w", err)
	}
	return nil
}

// EnableUser activates a VPN user in Remnawave.
func (a *GatewayAdapter) EnableUser(ctx context.Context, remnawaveUUID string) error {
	if err := a.client.EnableUser(ctx, remnawaveUUID); err != nil {
		return fmt.Errorf("remnawave enable user: %w", err)
	}
	return nil
}

// DisableUser deactivates a VPN user in Remnawave.
func (a *GatewayAdapter) DisableUser(ctx context.Context, remnawaveUUID string) error {
	if err := a.client.DisableUser(ctx, remnawaveUUID); err != nil {
		return fmt.Errorf("remnawave disable user: %w", err)
	}
	return nil
}

// AssignToSquad assigns a Remnawave user to an internal squad (server group)
// via the bulk-add-users endpoint.
func (a *GatewayAdapter) AssignToSquad(ctx context.Context, remnawaveUUID, squadUUID string) error {
	userID, err := parseUserRef(remnawaveUUID)
	if err != nil {
		return fmt.Errorf("remnawave assign to squad %s: %w", squadUUID, err)
	}
	if err := a.client.AddUsersToInternalSquad(ctx, squadUUID, []int64{userID}); err != nil {
		return fmt.Errorf("remnawave assign user %s to squad %s: %w", remnawaveUUID, squadUUID, err)
	}
	return nil
}

// parseUserRef turns the platform's text identifier for a bound Remnawave user
// back into the numeric id the panel expects. Bindings created before the
// Remnawave 3 migration hold a UUID string and cannot be addressed by the v3
// API at all, so the failure is reported rather than passed along.
func parseUserRef(ref string) (int64, error) {
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("remnawave user ref %q is not a numeric id (pre-v3 binding?): %w", ref, err)
	}
	return id, nil
}

// compile-time interface check
var _ multisub.RemnawaveGateway = (*GatewayAdapter)(nil)
