package remnawave

import (
	"context"
	"encoding/json"
	"net/http"
)

// APIPathMetadata is the base path for metadata endpoints.
const APIPathMetadata = "/api/metadata/"

// Metadata endpoint path segments.
const (
	metadataPathNode = "node/"
	metadataPathUser = "user/"
)

// GetNodeMetadata returns the raw JSON metadata for a specific node.
func (c *Client) GetNodeMetadata(ctx context.Context, uuid string) (json.RawMessage, error) {
	var resp APIResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, APIPathMetadata+metadataPathNode+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// UpsertNodeMetadata creates or updates the metadata for a specific node.
func (c *Client) UpsertNodeMetadata(ctx context.Context, uuid string, metadata json.RawMessage) error {
	return c.do(ctx, http.MethodPut, APIPathMetadata+metadataPathNode+uuid, metadata, nil)
}

// GetUserMetadata returns the raw JSON metadata for a specific user.
func (c *Client) GetUserMetadata(ctx context.Context, uuid string) (json.RawMessage, error) {
	var resp APIResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodGet, APIPathMetadata+metadataPathUser+uuid, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Response, nil
}

// UpsertUserMetadata creates or updates the metadata for a specific user.
func (c *Client) UpsertUserMetadata(ctx context.Context, uuid string, metadata json.RawMessage) error {
	return c.do(ctx, http.MethodPut, APIPathMetadata+metadataPathUser+uuid, metadata, nil)
}
