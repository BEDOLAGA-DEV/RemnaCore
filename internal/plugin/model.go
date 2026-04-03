package plugin

import (
	"time"

	"github.com/google/uuid"
)

// Plugin is the domain entity representing an installed plugin. It corresponds
// to a row in the plugins.plugin_registry table.
type Plugin struct {
	ID          string
	Slug        string
	Name        string
	Version     string
	Description string
	Author      string
	License     string
	SDKVersion  string
	Lang        string
	WASMBytes   []byte            // stored WASM binary
	Manifest    *Manifest         // parsed plugin.toml
	Status      PluginStatus
	Config      map[string]string // runtime config values (filled by admin)
	Permissions []PermissionScope
	ErrorLog    string
	InstalledAt time.Time
	EnabledAt   *time.Time
	UpdatedAt   time.Time
}

// HookRegistration binds a plugin to a specific hook point with a priority and
// an exported WASM function name.
type HookRegistration struct {
	PluginID   string
	PluginSlug string
	HookName   string
	HookType   HookType
	Priority   int
	FuncName   string // exported WASM function name
}

// NewPlugin validates the manifest and returns a new Plugin in StatusInstalled
// state with a generated UUID. wasmBytes may be nil when the binary is stored
// externally.
func NewPlugin(manifest *Manifest, wasmBytes []byte, now time.Time) (*Plugin, error) {
	if manifest == nil {
		return nil, ErrInvalidManifest
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return &Plugin{
		ID:          uuid.New().String(),
		Slug:        manifest.Plugin.ID,
		Name:        manifest.Plugin.Name,
		Version:     manifest.Plugin.Version,
		Description: manifest.Plugin.Description,
		Author:      manifest.Plugin.Author,
		License:     manifest.Plugin.License,
		SDKVersion:  manifest.Plugin.SDKVersion,
		Lang:        manifest.Plugin.Lang,
		WASMBytes:   wasmBytes,
		Manifest:    manifest,
		Status:      StatusInstalled,
		Config:      make(map[string]string),
		Permissions: manifest.ParsePermissions(),
		InstalledAt: now,
		UpdatedAt:   now,
	}, nil
}

// Enable transitions the plugin to StatusEnabled.
func (p *Plugin) Enable(now time.Time) error {
	if p.Status == StatusEnabled {
		return ErrPluginAlreadyEnabled
	}
	p.Status = StatusEnabled
	p.EnabledAt = &now
	p.UpdatedAt = now
	return nil
}

// Disable transitions the plugin to StatusDisabled.
func (p *Plugin) Disable(now time.Time) {
	p.Status = StatusDisabled
	p.UpdatedAt = now
}

// SetError transitions the plugin to StatusError and records the reason.
func (p *Plugin) SetError(reason string, now time.Time) {
	p.Status = StatusError
	p.ErrorLog = reason
	p.UpdatedAt = now
}
