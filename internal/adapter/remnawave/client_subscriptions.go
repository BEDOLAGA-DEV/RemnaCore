package remnawave

import (
	"context"
	"net/http"
	"strconv"
)

// Remnawave API path constant for subscription endpoints.
const APIPathSubscriptions = "/api/subscriptions/"

// Subscription lookup sub-path constants.
const (
	subPathByUsername  = "by-username/"
	subPathByUserID    = "by-id/"
	subPathByShortUUID = "by-short-uuid/"
)

// GetAllSubscriptions returns all subscriptions from Remnawave.
func (c *Client) GetAllSubscriptions(ctx context.Context) ([]RemnawaveSubscription, error) {
	var resp APIResponse[[]RemnawaveSubscription]
	if err := c.do(ctx, http.MethodGet, APIPathSubscriptions, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// GetSubscriptionByUsername retrieves a subscription by its username.
func (c *Client) GetSubscriptionByUsername(ctx context.Context, username string) (*RemnawaveSubscription, error) {
	var resp APIResponse[RemnawaveSubscription]
	if err := c.do(ctx, http.MethodGet, APIPathSubscriptions+subPathByUsername+username, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetSubscriptionByUserID retrieves a subscription by its owner's user id.
// Remnawave 3 dropped by-uuid along with the user UUID and addresses the
// subscription through the numeric user id instead.
func (c *Client) GetSubscriptionByUserID(ctx context.Context, userID int64) (*RemnawaveSubscription, error) {
	var resp APIResponse[RemnawaveSubscription]
	path := APIPathSubscriptions + subPathByUserID + strconv.FormatInt(userID, 10)
	if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}

// GetSubscriptionPageConfigs returns all subscription page configurations.
func (c *Client) GetSubscriptionPageConfigs(ctx context.Context) ([]map[string]any, error) {
	var resp APIResponse[[]map[string]any]
	if err := c.do(ctx, http.MethodGet, "/api/subscription-page-configs/", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// GetSubscriptionByShortUUID retrieves a subscription by its short UUID.
func (c *Client) GetSubscriptionByShortUUID(ctx context.Context, shortUUID string) (*RemnawaveSubscription, error) {
	var resp APIResponse[RemnawaveSubscription]
	if err := c.do(ctx, http.MethodGet, APIPathSubscriptions+subPathByShortUUID+shortUUID, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Response, nil
}
