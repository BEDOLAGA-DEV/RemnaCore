package settings

import (
	"context"
	"encoding/json"
)

// OverrideRepository reads and writes runtime configuration overrides.
type OverrideRepository interface {
	// GetOverrides returns the persisted JSON overrides blob.
	GetOverrides(ctx context.Context) (json.RawMessage, error)

	// SaveOverrides persists the JSON overrides blob, replacing any existing value.
	SaveOverrides(ctx context.Context, overrides json.RawMessage) error
}
