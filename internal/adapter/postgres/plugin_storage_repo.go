package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/postgres/gen"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/plugin"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/pgutil"
)

// BytesPerMB is the number of bytes in one megabyte, used for storage quota conversion.
const BytesPerMB = 1024 * 1024

// PluginStorageRepository implements plugin.StorageService using a shared
// plugins.plugin_storage table with composite key (plugin_slug, key).
type PluginStorageRepository struct {
	pool          *pgxpool.Pool
	queries       *gen.Queries
	maxStorageMB  int // default quota per plugin in MB
}

// NewPluginStorageRepository returns a new PluginStorageRepository.
// maxStorageMB sets the default storage quota per plugin in megabytes.
func NewPluginStorageRepository(pool *pgxpool.Pool, maxStorageMB int) *PluginStorageRepository {
	if maxStorageMB <= 0 {
		maxStorageMB = plugin.DefaultMaxStorageMB
	}
	return &PluginStorageRepository{
		pool:         pool,
		queries:      gen.New(pool),
		maxStorageMB: maxStorageMB,
	}
}

func (r *PluginStorageRepository) Get(ctx context.Context, pluginSlug, key string) ([]byte, error) {
	row, err := r.queries.StorageGet(ctx, gen.StorageGetParams{
		PluginSlug: pluginSlug,
		Key:        key,
	})
	if err != nil {
		return nil, pgutil.MapErr(err, "storage get", plugin.ErrPluginNotFound)
	}

	// Check expiry: if the row has an expiration in the past, treat it as
	// not found and clean it up asynchronously.
	if row.ExpiresAt.Valid && row.ExpiresAt.Time.Before(time.Now()) {
		_ = r.queries.StorageDelete(ctx, gen.StorageDeleteParams{
			PluginSlug: pluginSlug,
			Key:        key,
		})
		return nil, plugin.ErrPluginNotFound
	}

	return row.Value, nil
}

func (r *PluginStorageRepository) Set(ctx context.Context, pluginSlug, key string, value []byte, ttlSeconds int64) error {
	// Quota enforcement: check current usage before writing.
	usedBytes, err := r.queries.StorageGetSize(ctx, pluginSlug)
	if err != nil {
		return pgutil.MapErr(err, "storage get size", plugin.ErrPluginNotFound)
	}

	maxBytes := int64(r.maxStorageMB) * BytesPerMB
	if usedBytes+int64(len(value)) > maxBytes {
		return plugin.ErrStorageQuotaExceeded
	}

	var expiresAt pgtype.Timestamptz
	if ttlSeconds > 0 {
		t := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
		expiresAt = pgutil.TimeToPgtype(t)
	}

	err = r.queries.StorageSet(ctx, gen.StorageSetParams{
		PluginSlug: pluginSlug,
		Key:        key,
		Value:      value,
		ExpiresAt:  expiresAt,
	})
	return pgutil.MapErr(err, "storage set", plugin.ErrPluginNotFound)
}

func (r *PluginStorageRepository) Delete(ctx context.Context, pluginSlug, key string) error {
	err := r.queries.StorageDelete(ctx, gen.StorageDeleteParams{
		PluginSlug: pluginSlug,
		Key:        key,
	})
	return pgutil.MapErr(err, "storage delete", plugin.ErrPluginNotFound)
}

func (r *PluginStorageRepository) DeleteAll(ctx context.Context, pluginSlug string) error {
	err := r.queries.StorageDeleteAll(ctx, pluginSlug)
	return pgutil.MapErr(err, "storage delete all", plugin.ErrPluginNotFound)
}

func (r *PluginStorageRepository) GetUsedBytes(ctx context.Context, pluginSlug string) (int64, error) {
	bytes, err := r.queries.StorageGetSize(ctx, pluginSlug)
	if err != nil {
		return 0, pgutil.MapErr(err, "storage get size", plugin.ErrPluginNotFound)
	}
	return bytes, nil
}

// compile-time interface check
var _ plugin.StorageService = (*PluginStorageRepository)(nil)
