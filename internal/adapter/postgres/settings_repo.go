package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// settingsRowID is the fixed primary key for the singleton runtime settings row.
const settingsRowID = "runtime"

// SettingsRepository reads and writes runtime configuration overrides
// stored in platform.system_settings as a single JSONB blob.
type SettingsRepository struct {
	pool *pgxpool.Pool
}

// NewSettingsRepository returns a new SettingsRepository backed by the given pool.
func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

// GetOverrides returns the JSON overrides blob from the database.
// Returns an empty JSON object ({}) when no overrides have been saved.
func (r *SettingsRepository) GetOverrides(ctx context.Context) (json.RawMessage, error) {
	const query = `SELECT overrides FROM platform.system_settings WHERE id = $1`

	var overrides json.RawMessage
	if err := r.pool.QueryRow(ctx, query, settingsRowID).Scan(&overrides); err != nil {
		return nil, fmt.Errorf("get settings overrides: %w", err)
	}

	return overrides, nil
}

// SaveOverrides writes the JSON overrides blob to the database, replacing
// any existing value and updating the timestamp.
func (r *SettingsRepository) SaveOverrides(ctx context.Context, overrides json.RawMessage) error {
	const query = `UPDATE platform.system_settings SET overrides = $1, updated_at = now() WHERE id = $2`

	tag, err := r.pool.Exec(ctx, query, overrides, settingsRowID)
	if err != nil {
		return fmt.Errorf("save settings overrides: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("save settings overrides: no rows updated (missing seed row)")
	}

	return nil
}
