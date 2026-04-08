package remnawave

import (
	"context"
	"net/http"
)

// Remnawave API path constants for bulk user operations.
const (
	APIPathUsersBulk    = "/api/users/bulk/"
	APIPathUsersBulkAll = "/api/users/bulk/all/"
)

// Bulk action path suffixes.
const (
	BulkActionDeleteByStatus = "delete-by-status"
	BulkActionUpdate         = "update"
	BulkActionResetTraffic   = "reset-traffic"
	BulkActionRevokeSub      = "revoke-subscription"
	BulkActionDelete         = "delete"
	BulkActionUpdateSquads   = "update-squads"
	BulkActionExtendExpiry   = "extend-expiration-date"
)

// BulkDeleteUsersByStatus deletes all users matching the given status.
func (c *Client) BulkDeleteUsersByStatus(ctx context.Context, status string) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionDeleteByStatus, StatusRequest{Status: status}, nil)
}

// BulkUpdateUsers applies bulk updates to the specified users.
func (c *Client) BulkUpdateUsers(ctx context.Context, req BulkUpdateUsersRequest) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionUpdate, req, nil)
}

// BulkResetTraffic resets traffic counters for the specified users.
func (c *Client) BulkResetTraffic(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionResetTraffic, UUIDsRequest{UUIDs: uuids}, nil)
}

// BulkRevokeSubscription revokes subscriptions for the specified users.
func (c *Client) BulkRevokeSubscription(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionRevokeSub, UUIDsRequest{UUIDs: uuids}, nil)
}

// BulkDeleteUsers deletes the specified users by UUID.
func (c *Client) BulkDeleteUsers(ctx context.Context, uuids []string) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionDelete, UUIDsRequest{UUIDs: uuids}, nil)
}

// BulkUpdateSquads updates squad assignments for the specified users.
func (c *Client) BulkUpdateSquads(ctx context.Context, req BulkUpdateSquadsRequest) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionUpdateSquads, req, nil)
}

// BulkExtendExpiration extends the expiration date for the specified users.
func (c *Client) BulkExtendExpiration(ctx context.Context, req BulkExtendExpirationRequest) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulk+BulkActionExtendExpiry, req, nil)
}

// BulkUpdateAllUsers applies bulk updates to all users.
func (c *Client) BulkUpdateAllUsers(ctx context.Context, req BulkUpdateAllRequest) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulkAll+BulkActionUpdate, req, nil)
}

// BulkResetAllTraffic resets traffic counters for all users.
func (c *Client) BulkResetAllTraffic(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulkAll+BulkActionResetTraffic, nil, nil)
}

// BulkExtendAllExpiration extends the expiration date for all users.
func (c *Client) BulkExtendAllExpiration(ctx context.Context, req BulkExtendExpirationRequest) error {
	return c.do(ctx, http.MethodPost, APIPathUsersBulkAll+BulkActionExtendExpiry, req, nil)
}
