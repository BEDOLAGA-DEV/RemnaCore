package multisub

import (
	"context"
	"time"
)

// VPNProvider is the domain port for VPN user management. Unlike
// RemnawaveGateway, this interface is provider-agnostic and can be backed by
// any VPN panel via plugins. The existing RemnawaveGateway is kept as-is for
// the hardcoded path; VPNProvider is used when the feature flag
// HooksVPNProviderEnabled is true.
type VPNProvider interface {
	CreateUser(ctx context.Context, req CreateVPNUserRequest) (*VPNUserResult, error)
	GetUser(ctx context.Context, vpnUUID string) (*VPNUserStatus, error)
	DeleteUser(ctx context.Context, vpnUUID string) error
	EnableUser(ctx context.Context, vpnUUID string) error
	DisableUser(ctx context.Context, vpnUUID string) error
}

// CreateVPNUserRequest contains parameters for creating a VPN user through a
// plugin-backed VPN provider. Fields mirror the subset of
// CreateRemnawaveUserRequest that is provider-agnostic.
type CreateVPNUserRequest struct {
	Username          string
	TrafficLimitBytes int64
	TrafficStrategy   string
	ExpireAt          *time.Time
	Tag               string
	BindingPurpose    string
}

// VPNUserResult is returned after a VPN user is successfully created.
type VPNUserResult struct {
	UUID      string
	ShortUUID string
}

// VPNUserStatus represents the current state of a VPN user as reported by the
// external VPN provider. Used during periodic sync to detect out-of-band
// status changes.
type VPNUserStatus struct {
	UUID      string
	Enabled   bool
	Expired   bool
	UsedBytes int64
}
