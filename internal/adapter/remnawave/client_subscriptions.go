package remnawave

import (
	"context"
	"net/http"
)

// Remnawave API path constant for subscription endpoints.
const APIPathSubscriptions = "/api/subscriptions/"

// Subscription lookup sub-path constants.
const (
	subPathByUsername  = "by-username/"
	subPathByUUID      = "by-uuid/"
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

// GetSubscriptionByUUID retrieves a subscription by its UUID.
func (c *Client) GetSubscriptionByUUID(ctx context.Context, uuid string) (*RemnawaveSubscription, error) {
	var resp APIResponse[RemnawaveSubscription]
	if err := c.do(ctx, http.MethodGet, APIPathSubscriptions+subPathByUUID+uuid, nil, &resp); err != nil {
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
