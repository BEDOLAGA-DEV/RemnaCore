package plugin

import "context"

// StorageService provides isolated key-value storage for plugins using a
// shared table namespaced by plugin slug.
type StorageService interface {
	// Get retrieves the value for a key. Returns ErrPluginNotFound if the key
	// does not exist or has expired.
	Get(ctx context.Context, pluginSlug, key string) ([]byte, error)

	// Set stores a value under a key. If ttlSeconds > 0 the entry will expire
	// after that many seconds. Quota is checked before writing; if the plugin
	// exceeds its storage limit ErrStorageQuotaExceeded is returned.
	Set(ctx context.Context, pluginSlug, key string, value []byte, ttlSeconds int64) error

	// Delete removes a single key.
	Delete(ctx context.Context, pluginSlug, key string) error

	// DeleteAll removes all keys for the given plugin slug.
	DeleteAll(ctx context.Context, pluginSlug string) error

	// GetUsedBytes returns the total storage bytes consumed by the plugin.
	GetUsedBytes(ctx context.Context, pluginSlug string) (int64, error)
}
