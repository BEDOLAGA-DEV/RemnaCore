package plugin

import (
	"fmt"
	"regexp"

	toml "github.com/pelletier/go-toml/v2"
)

// slugRe is compiled once and reused for every validation call.
var slugRe = regexp.MustCompile(PluginSlugPattern)

// Manifest is the strongly-typed representation of a plugin.toml file.
type Manifest struct {
	Plugin      ManifestPlugin                 `toml:"plugin"`
	Permissions ManifestPermissions            `toml:"permissions"`
	Hooks       ManifestHooks                  `toml:"hooks"`
	Config      map[string]ManifestConfigField `toml:"config"`
	Limits      ManifestLimits                 `toml:"limits"`
}

// ManifestPlugin holds the top-level metadata about the plugin.
type ManifestPlugin struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Version     string `toml:"version"`
	Description string `toml:"description"`
	Author      string `toml:"author"`
	License     string `toml:"license"`
	SDKVersion  string `toml:"sdk_version"`
	Lang        string `toml:"lang"`
}

// ManifestPermissions declares what platform capabilities the plugin needs.
type ManifestPermissions struct {
	Billing       string   `toml:"billing"`
	Payment       string   `toml:"payment"`
	Users         string   `toml:"users"`
	Notifications string   `toml:"notifications"`
	Analytics     string   `toml:"analytics"`
	Storage       string   `toml:"storage"`
	HTTP          []string `toml:"http"`
	VPN           string   `toml:"vpn"`
	API           string   `toml:"api"`
}

// ManifestHooks lists the hook points the plugin subscribes to.
type ManifestHooks struct {
	Sync     []string       `toml:"sync"`
	Async    []string       `toml:"async"`
	Priority map[string]int `toml:"priority"`
}

// ManifestConfigField describes a single admin-configurable field for the
// plugin.
type ManifestConfigField struct {
	Type     string   `toml:"type"`     // string, secret, select, number, boolean
	Label    string   `toml:"label"`
	Required bool     `toml:"required"`
	Default  string   `toml:"default"`
	Options  []string `toml:"options"` // for select type
}

// ManifestLimits lets a plugin request custom resource limits. Zero values
// are replaced with platform defaults by EffectiveLimits.
type ManifestLimits struct {
	MaxMemoryMB        int `toml:"max_memory_mb"`
	MaxFuel            int `toml:"max_fuel"`
	MaxStorageMB       int `toml:"max_storage_mb"`
	MaxHTTPCallsPerMin int `toml:"max_http_calls_per_minute"`
	TimeoutSyncMs      int `toml:"timeout_sync_ms"`
	TimeoutAsyncMs     int `toml:"timeout_async_ms"`
	PoolSize           int `toml:"pool_size"` // WASM instance pool size (default 4, max 16)
}

// ParseManifest deserialises TOML bytes into a Manifest and validates it.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks required fields, slug format, and hook presence.
func (m *Manifest) Validate() error {
	if m.Plugin.ID == "" {
		return fmt.Errorf("%w: plugin.id is required", ErrInvalidManifest)
	}
	if len(m.Plugin.ID) > MaxPluginSlugLen {
		return fmt.Errorf("%w: plugin.id exceeds max length %d", ErrInvalidPluginSlug, MaxPluginSlugLen)
	}
	if !slugRe.MatchString(m.Plugin.ID) {
		return fmt.Errorf("%w: plugin.id must match %s", ErrInvalidPluginSlug, PluginSlugPattern)
	}
	if m.Plugin.Name == "" {
		return fmt.Errorf("%w: plugin.name is required", ErrInvalidManifest)
	}
	if m.Plugin.Version == "" {
		return fmt.Errorf("%w: plugin.version is required", ErrInvalidManifest)
	}
	if m.Plugin.SDKVersion == "" {
		return fmt.Errorf("%w: sdk_version is required", ErrInvalidManifest)
	}
	if len(m.Hooks.Sync) == 0 && len(m.Hooks.Async) == 0 {
		return fmt.Errorf("%w: at least one hook (sync or async) is required", ErrInvalidManifest)
	}
	return nil
}

// EffectiveLimits returns a ManifestLimits with platform defaults applied for
// any field that was left at zero or negative by the plugin author.
func (m *Manifest) EffectiveLimits() ManifestLimits {
	l := m.Limits
	if l.MaxMemoryMB <= 0 {
		l.MaxMemoryMB = DefaultMaxMemoryMB
	}
	if l.MaxMemoryMB > MaxMemoryMB {
		l.MaxMemoryMB = MaxMemoryMB
	}
	if l.MaxFuel <= 0 {
		l.MaxFuel = DefaultMaxFuel
	}
	if l.MaxStorageMB <= 0 {
		l.MaxStorageMB = DefaultMaxStorageMB
	}
	if l.MaxHTTPCallsPerMin <= 0 {
		l.MaxHTTPCallsPerMin = DefaultMaxHTTPCallsPerMin
	}
	if l.TimeoutSyncMs <= 0 {
		l.TimeoutSyncMs = DefaultSyncTimeoutMs
	}
	if l.TimeoutAsyncMs <= 0 {
		l.TimeoutAsyncMs = DefaultAsyncTimeoutMs
	}
	if l.PoolSize <= 0 {
		l.PoolSize = DefaultPoolSize
	}
	if l.PoolSize > MaxPoolSize {
		l.PoolSize = MaxPoolSize
	}
	return l
}

// ParsePermissions converts the human-friendly manifest permission fields into
// a typed slice of PermissionScope values.
func (m *Manifest) ParsePermissions() []PermissionScope {
	var perms []PermissionScope

	switch m.Permissions.Billing {
	case PermValueRead:
		perms = append(perms, PermBillingRead)
	case PermValueWrite:
		perms = append(perms, PermBillingRead, PermBillingWrite)
	}

	if m.Permissions.Payment == PermValueWrite {
		perms = append(perms, PermPaymentWrite)
	}

	switch m.Permissions.Users {
	case PermValueRead:
		perms = append(perms, PermUsersRead)
	case PermValueWrite:
		perms = append(perms, PermUsersRead, PermUsersWrite)
	}

	if m.Permissions.Notifications == PermValueEmit {
		perms = append(perms, PermNotificationsEmit)
	}

	if m.Permissions.Analytics == PermValueWrite {
		perms = append(perms, PermAnalyticsWrite)
	}

	switch m.Permissions.Storage {
	case PermValueRead:
		perms = append(perms, PermStorageRead)
	case PermValueReadWrite:
		perms = append(perms, PermStorageRead, PermStorageWrite)
	}

	switch m.Permissions.VPN {
	case PermValueRead:
		perms = append(perms, PermVPNRead)
	case PermValueWrite:
		perms = append(perms, PermVPNRead, PermVPNWrite)
	}

	if m.Permissions.API == PermValueRoutes {
		perms = append(perms, PermAPIRoutes)
	}

	return perms
}

// HookRegistrations generates a HookRegistration entry for every sync and
// async hook declared in the manifest. Priority falls back to
// DefaultPluginPriority when not overridden.
func (m *Manifest) HookRegistrations(pluginID string) []HookRegistration {
	var regs []HookRegistration

	for _, hook := range m.Hooks.Sync {
		regs = append(regs, HookRegistration{
			PluginID:   pluginID,
			PluginSlug: m.Plugin.ID,
			HookName:   hook,
			HookType:   HookSync,
			Priority:   m.hookPriority(hook),
			FuncName:   hook,
		})
	}

	for _, hook := range m.Hooks.Async {
		regs = append(regs, HookRegistration{
			PluginID:   pluginID,
			PluginSlug: m.Plugin.ID,
			HookName:   hook,
			HookType:   HookAsync,
			Priority:   m.hookPriority(hook),
			FuncName:   hook,
		})
	}

	return regs
}

// hookPriority returns the configured priority for a hook name, or the default.
func (m *Manifest) hookPriority(hookName string) int {
	if m.Hooks.Priority != nil {
		if p, ok := m.Hooks.Priority[hookName]; ok {
			return p
		}
	}
	return DefaultPluginPriority
}
