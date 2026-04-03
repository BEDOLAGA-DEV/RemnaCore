package plugin

import (
	"context"
	"time"
)

// PluginRepository persists plugin registry records.
type PluginRepository interface {
	Create(ctx context.Context, p *Plugin) error
	GetByID(ctx context.Context, id string) (*Plugin, error)
	GetBySlug(ctx context.Context, slug string) (*Plugin, error)
	GetAll(ctx context.Context) ([]*Plugin, error)
	GetEnabled(ctx context.Context) ([]*Plugin, error)
	UpdateStatus(ctx context.Context, id string, status PluginStatus, errorLog string, enabledAt *time.Time) error
	UpdateConfig(ctx context.Context, id string, config map[string]string) error
	UpdatePlugin(ctx context.Context, p *Plugin) error
	Delete(ctx context.Context, id string) error
}
