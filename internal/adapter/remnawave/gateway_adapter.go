package remnawave

import (
	"context"
	"fmt"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub"
)

// GatewayAdapter implements multisub.RemnawaveGateway, translating between
// domain port types and Remnawave client types. This is the Anti-Corruption
// Layer boundary; no remnawave client types leak into the domain.
type GatewayAdapter struct {
	client *ResilientClient
}

// NewGatewayAdapter creates a GatewayAdapter backed by the given resilient
// client.
func NewGatewayAdapter(client *ResilientClient) *GatewayAdapter {
	return &GatewayAdapter{client: client}
}

// CreateUser provisions a VPN user in Remnawave, translating the domain
// request into a Remnawave API call and mapping the result back.
func (a *GatewayAdapter) CreateUser(ctx context.Context, req multisub.CreateRemnawaveUserRequest) (*multisub.RemnawaveUserResult, error) {
	rwReq := CreateUserRequest{
		Username:       req.Username,
		TrafficLimitBytes: float64(req.TrafficLimitBytes),
	}
	if req.ExpireAt != nil {
		rwReq.ExpireAt = *req.ExpireAt
	} else {
		// Default: 30 days from now if no expiry specified.
		rwReq.ExpireAt = time.Now().AddDate(0, 1, 0)
	}

	user, err := a.client.CreateUser(ctx, rwReq)
	if err != nil {
		return nil, fmt.Errorf("remnawave create user: %w", err)
	}

	return &multisub.RemnawaveUserResult{
		UUID:      user.UUID,
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
		UUID:      user.UUID,
		Enabled:   user.Status == RemnawaveStatusActive,
		Expired:   user.Status == RemnawaveStatusExpired,
		UsedBytes: user.UsedTrafficBytes,
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

// AssignToSquad assigns a Remnawave user to a squad (server group). This maps
// to a future Remnawave API endpoint; currently a no-op placeholder that
// returns nil to indicate success.
func (a *GatewayAdapter) AssignToSquad(_ context.Context, _, _ string) error {
	// TODO: implement when Remnawave adds squad assignment API endpoint.
	return nil
}

// compile-time interface check
var _ multisub.RemnawaveGateway = (*GatewayAdapter)(nil)
