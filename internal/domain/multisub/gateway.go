package multisub

import (
	"context"
	"time"
)

// RemnawaveGateway is a port that abstracts Remnawave API operations.
// The adapter (internal/adapter/remnawave) implements this interface.
type RemnawaveGateway interface {
	CreateUser(ctx context.Context, req CreateRemnawaveUserRequest) (*RemnawaveUserResult, error)
	GetUser(ctx context.Context, remnawaveUUID string) (*RemnawaveUserStatus, error)
	DeleteUser(ctx context.Context, remnawaveUUID string) error
	EnableUser(ctx context.Context, remnawaveUUID string) error
	DisableUser(ctx context.Context, remnawaveUUID string) error
	AssignToSquad(ctx context.Context, remnawaveUUID, squadUUID string) error
}

// CreateRemnawaveUserRequest holds the data needed to create a Remnawave VPN user.
type CreateRemnawaveUserRequest struct {
	Username          string
	TrafficLimitBytes int64
	TrafficStrategy   string
	ExpireAt          *time.Time
	Tag               string
}

// RemnawaveUserResult is the data returned after successfully creating a Remnawave user.
type RemnawaveUserResult struct {
	UUID      string
	ShortUUID string
}

// RemnawaveUserStatus holds the current state of a Remnawave VPN user as
// reported by the Remnawave panel. Used during periodic sync to detect
// out-of-band status changes.
type RemnawaveUserStatus struct {
	UUID      string
	Enabled   bool
	Expired   bool
	UsedBytes int64
}
