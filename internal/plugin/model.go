package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// pluginTransitions defines valid status transitions. A transition not in
// this map is rejected by canTransitionTo.
var pluginTransitions = map[PluginStatus][]PluginStatus{
	StatusInstalled: {StatusEnabled, StatusError},
	StatusEnabled:   {StatusDisabled, StatusError},
	StatusDisabled:  {StatusEnabled, StatusError},
	StatusError:     {StatusEnabled, StatusDisabled}, // recovery paths
}

// canTransitionTo reports whether the plugin's current status allows a
// transition to the target status.
func (p *Plugin) canTransitionTo(target PluginStatus) bool {
	allowed, ok := pluginTransitions[p.Status]
	if !ok {
		return false
	}
	return slices.Contains(allowed, target)
}

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
	WASMHash    string            // SHA-256 hex digest of WASMBytes for content-addressable storage
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

	var wasmHash string
	if len(wasmBytes) > 0 {
		h := sha256.Sum256(wasmBytes)
		wasmHash = hex.EncodeToString(h[:])
	}

	return &Plugin{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Slug:        manifest.Plugin.ID,
		Name:        manifest.Plugin.Name,
		Version:     manifest.Plugin.Version,
		Description: manifest.Plugin.Description,
		Author:      manifest.Plugin.Author,
		License:     manifest.Plugin.License,
		SDKVersion:  manifest.Plugin.SDKVersion,
		Lang:        manifest.Plugin.Lang,
		WASMBytes:   wasmBytes,
		WASMHash:    wasmHash,
		Manifest:    manifest,
		Status:      StatusInstalled,
		Config:      make(map[string]string),
		Permissions: manifest.ParsePermissions(),
		InstalledAt: now,
		UpdatedAt:   now,
	}, nil
}

// secretRedactionMarker replaces secret config values in API responses and logs.
const secretRedactionMarker = "***REDACTED***"

// GetConfigValue returns the config value for the given key. Values for fields
// declared as type="secret" in the manifest are returned verbatim here; use
// RedactedConfig for admin API responses and logging.
func (p *Plugin) GetConfigValue(key string) string {
	return p.Config[key]
}

// RedactedConfig returns a copy of the config map with secret values replaced
// by a redaction marker. Fields are considered secret when declared as
// type="secret" in the plugin manifest. This is used for admin API responses
// and structured logging to prevent accidental secret leakage.
func (p *Plugin) RedactedConfig() map[string]string {
	redacted := make(map[string]string, len(p.Config))
	for k, v := range p.Config {
		if p.Manifest != nil && p.Manifest.IsSecretField(k) {
			redacted[k] = secretRedactionMarker
			continue
		}
		redacted[k] = v
	}
	return redacted
}

// Enable transitions the plugin to StatusEnabled.
func (p *Plugin) Enable(now time.Time) error {
	if !p.canTransitionTo(StatusEnabled) {
		return fmt.Errorf("%w: cannot transition from %s to enabled", ErrInvalidTransition, p.Status)
	}
	p.Status = StatusEnabled
	p.EnabledAt = &now
	p.ErrorLog = "" // clear error on recovery
	p.UpdatedAt = now
	return nil
}

// Disable transitions the plugin to StatusDisabled.
func (p *Plugin) Disable(now time.Time) error {
	if !p.canTransitionTo(StatusDisabled) {
		return fmt.Errorf("%w: cannot transition from %s to disabled", ErrInvalidTransition, p.Status)
	}
	p.Status = StatusDisabled
	p.UpdatedAt = now
	return nil
}

// SetError transitions the plugin to StatusError and records the reason.
func (p *Plugin) SetError(reason string, now time.Time) error {
	if !p.canTransitionTo(StatusError) {
		return fmt.Errorf("%w: cannot transition from %s to error", ErrInvalidTransition, p.Status)
	}
	p.Status = StatusError
	p.ErrorLog = reason
	p.UpdatedAt = now
	return nil
}
