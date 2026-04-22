package plugin

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// slugRe is compiled once and reused for every validation call.
var slugRe = regexp.MustCompile(PluginSlugPattern)

// pagePathRe validates page path segments: lowercase alphanumeric with hyphens,
// must start with a letter or digit.
var pagePathRe = regexp.MustCompile(PagePathPattern)

// routeFuncRe validates route function names: lowercase alphanumeric with underscores.
var routeFuncRe = regexp.MustCompile(RouteFunctionPattern)

// allowedRouteMethods is the set of HTTP methods a plugin route may declare.
var allowedRouteMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"DELETE": true,
	"PATCH":  true,
}

// reservedRoutePathPrefixes are path prefixes owned by the platform. Plugin
// routes must not start with any of these.
var reservedRoutePathPrefixes = []string{
	"/api/admin/",
	"/api/auth/",
	"/api/me",
	"/api/webhooks/",
}

// AllowedHookPrefixes defines the valid namespace prefixes for plugin hooks.
// Every hook name declared in a manifest must start with one of these prefixes.
var AllowedHookPrefixes = []string{
	"ab.",
	"balance.",
	"binding.",
	"checkout.",
	"cohort.",
	"invoice.",
	"notification.",
	"payment.",
	"plugin.",
	"pricing.",
	"promo.",
	"subscription.",
	"tariff.",
	"user.",
	"vpn.",
}

// validateHookName checks that a hook name starts with one of the allowed
// namespace prefixes. Returns ErrInvalidManifest if no prefix matches.
func validateHookName(name string) error {
	for _, prefix := range AllowedHookPrefixes {
		if strings.HasPrefix(name, prefix) {
			return nil
		}
	}
	return fmt.Errorf("%w: hook %q has unknown prefix (allowed: %v)",
		ErrInvalidManifest, name, AllowedHookPrefixes)
}

// Manifest is the strongly-typed representation of a plugin.toml file.
type Manifest struct {
	Plugin      ManifestPlugin                 `toml:"plugin"`
	Permissions ManifestPermissions            `toml:"permissions"`
	Hooks       ManifestHooks                  `toml:"hooks"`
	Config      map[string]ManifestConfigField `toml:"config"`
	Limits      ManifestLimits                 `toml:"limits"`
	Pages       []ManifestPage                 `toml:"pages"`
	Routes      []ManifestRoute                `toml:"routes"`
}

// ManifestRoute declares an HTTP route provided by the plugin. The platform
// registers these routes on startup and proxies incoming requests to the
// plugin's RPC function.
type ManifestRoute struct {
	Method    string `toml:"method"`     // HTTP method: GET, POST, PUT, DELETE, PATCH
	Path      string `toml:"path"`       // URL path, e.g. "/api/tariffs"
	Function  string `toml:"function"`   // RPC function name to call
	Public    bool   `toml:"public"`     // If true, no JWT auth required
	AdminOnly bool   `toml:"admin_only"` // If true, requires admin role (implies not Public)
}

// ManifestPage declares a UI page provided by the plugin. Plugins use this to
// register admin or cabinet pages that the frontend renders as dynamic menu
// items.
type ManifestPage struct {
	Path       string              `toml:"path"`       // URL path segment, e.g. "tariffs"
	Title      string              `toml:"title"`      // Display title, e.g. "Tariffs"
	Icon       string              `toml:"icon"`       // Lucide icon name, e.g. "CreditCard"
	Menu       string              `toml:"menu"`       // Where to show: "admin" or "cabinet"
	Collection string              `toml:"collection"` // Which collection this page manages
	Fields     []ManifestPageField `toml:"fields"`     // Form field definitions for the page
}

// ManifestPageField declares a single form field for a plugin page. The
// frontend renders these fields when creating or editing documents in the
// page's collection. Pages without fields fall back to auto-detection.
type ManifestPageField struct {
	Key             string   `toml:"key"`               // JSON key in the document
	Label           string   `toml:"label"`             // Display label
	Type            string   `toml:"type"`              // text, number, boolean, select, multiselect, textarea
	Required        bool     `toml:"required"`
	Default         string   `toml:"default"`           // Default value (string representation)
	Options         []string `toml:"options"`           // Static options for select/multiselect
	OptionsURL      string   `toml:"options_url"`       // Dynamic options from URL
	OptionsValueKey string   `toml:"options_value_key"` // Key in response objects for option value
	OptionsLabelKey string   `toml:"options_label_key"` // Key in response objects for option label
	Group           string   `toml:"group"`             // Logical group for section-based layouts
	Span            int      `toml:"span"`              // Grid column span (1 or 2, default 1)
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
	if err := validatePluginVersion(m.Plugin.Version); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if m.Plugin.SDKVersion == "" {
		return fmt.Errorf("%w: sdk_version is required", ErrInvalidManifest)
	}
	if len(m.Hooks.Sync) == 0 && len(m.Hooks.Async) == 0 {
		return fmt.Errorf("%w: at least one hook (sync or async) is required", ErrInvalidManifest)
	}

	for _, hook := range m.Hooks.Sync {
		if err := validateHookName(hook); err != nil {
			return err
		}
	}
	for _, hook := range m.Hooks.Async {
		if err := validateHookName(hook); err != nil {
			return err
		}
	}

	if len(m.Pages) > MaxPagesPerPlugin {
		return fmt.Errorf("%w: too many pages: %d (max %d)",
			ErrInvalidManifest, len(m.Pages), MaxPagesPerPlugin)
	}

	pagePaths := make(map[string]bool, len(m.Pages))
	for i, page := range m.Pages {
		if err := validateManifestPage(i, page); err != nil {
			return err
		}
		if pagePaths[page.Path] {
			return fmt.Errorf("%w: pages[%d]: duplicate path %q",
				ErrInvalidManifest, i, page.Path)
		}
		pagePaths[page.Path] = true
	}

	if len(m.Routes) > MaxRoutesPerPlugin {
		return fmt.Errorf("%w: too many routes: %d (max %d)",
			ErrInvalidManifest, len(m.Routes), MaxRoutesPerPlugin)
	}

	routeKeys := make(map[string]bool, len(m.Routes))
	for i, route := range m.Routes {
		if err := validateManifestRoute(i, route); err != nil {
			return err
		}
		key := route.Method + " " + route.Path
		if routeKeys[key] {
			return fmt.Errorf("%w: routes[%d]: duplicate %s %s",
				ErrInvalidManifest, i, route.Method, route.Path)
		}
		routeKeys[key] = true
	}

	return nil
}

// validateManifestPage checks that a single page declaration has all required
// fields and that the menu value is one of the allowed page placements.
func validateManifestPage(index int, page ManifestPage) error {
	if page.Path == "" {
		return fmt.Errorf("%w: pages[%d].path is required", ErrInvalidManifest, index)
	}
	if len(page.Path) > MaxPagePathLen {
		return fmt.Errorf("%w: pages[%d].path too long: %d (max %d)",
			ErrInvalidManifest, index, len(page.Path), MaxPagePathLen)
	}
	if !pagePathRe.MatchString(page.Path) {
		return fmt.Errorf("%w: pages[%d].path must match %s, got %q",
			ErrInvalidManifest, index, PagePathPattern, page.Path)
	}
	if page.Title == "" {
		return fmt.Errorf("%w: pages[%d].title is required", ErrInvalidManifest, index)
	}
	if page.Menu == "" {
		return fmt.Errorf("%w: pages[%d].menu is required", ErrInvalidManifest, index)
	}
	if page.Menu != PageMenuAdmin && page.Menu != PageMenuCabinet {
		return fmt.Errorf("%w: pages[%d].menu must be %q or %q, got %q",
			ErrInvalidManifest, index, PageMenuAdmin, PageMenuCabinet, page.Menu)
	}
	return nil
}

// validateManifestRoute checks that a single route declaration has all
// required fields, a valid HTTP method, and does not use a reserved path.
func validateManifestRoute(index int, route ManifestRoute) error {
	if route.Method == "" {
		return fmt.Errorf("%w: routes[%d].method is required", ErrInvalidManifest, index)
	}
	if !allowedRouteMethods[route.Method] {
		return fmt.Errorf("%w: routes[%d].method must be one of GET, POST, PUT, DELETE, PATCH, got %q",
			ErrInvalidManifest, index, route.Method)
	}
	if route.Path == "" {
		return fmt.Errorf("%w: routes[%d].path is required", ErrInvalidManifest, index)
	}
	if !strings.HasPrefix(route.Path, RoutePathPrefix) {
		return fmt.Errorf("%w: routes[%d].path must start with %q, got %q",
			ErrInvalidManifest, index, RoutePathPrefix, route.Path)
	}
	for _, reserved := range reservedRoutePathPrefixes {
		if strings.HasPrefix(route.Path, reserved) {
			return fmt.Errorf("%w: routes[%d].path %q uses reserved prefix %q",
				ErrInvalidManifest, index, route.Path, reserved)
		}
	}
	if route.Function == "" {
		return fmt.Errorf("%w: routes[%d].function is required", ErrInvalidManifest, index)
	}
	if !routeFuncRe.MatchString(route.Function) {
		return fmt.Errorf("%w: routes[%d].function must match %s, got %q",
			ErrInvalidManifest, index, RouteFunctionPattern, route.Function)
	}
	return nil
}

// Version component count bounds for plugin version validation.
const (
	minVersionComponents = 2 // major.minor
	maxVersionComponents = 3 // major.minor.patch
)

// validatePluginVersion checks that a version string follows the
// major.minor or major.minor.patch format with numeric-only components.
func validatePluginVersion(version string) error {
	parts := strings.Split(version, ".")
	if len(parts) < minVersionComponents || len(parts) > maxVersionComponents {
		return fmt.Errorf("version must be major.minor or major.minor.patch, got %q", version)
	}
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return fmt.Errorf("version component %q is not a number", p)
		}
	}
	return nil
}

// LimitWarning describes a plugin resource limit that was adjusted by the
// platform to fit within allowed bounds.
type LimitWarning struct {
	Field     string // manifest field name (e.g. "max_memory_mb")
	Requested int    // value the plugin author specified
	Applied   int    // value actually used after adjustment
	Reason    string // "capped at platform maximum" or "set to platform default"
}

// Limit adjustment reason constants.
const (
	limitReasonCapped  = "capped at platform maximum"
	limitReasonDefault = "set to platform default"
)

// EffectiveLimits returns a ManifestLimits with platform defaults applied for
// any field that was left at zero or negative by the plugin author. It also
// returns warnings for every field that was adjusted, so callers can log them
// during plugin loading.
func (m *Manifest) EffectiveLimits() (ManifestLimits, []LimitWarning) {
	l := m.Limits
	var warnings []LimitWarning

	if l.MaxMemoryMB <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "max_memory_mb", Requested: l.MaxMemoryMB,
			Applied: DefaultMaxMemoryMB, Reason: limitReasonDefault,
		})
		l.MaxMemoryMB = DefaultMaxMemoryMB
	} else if l.MaxMemoryMB > MaxMemoryMB {
		warnings = append(warnings, LimitWarning{
			Field: "max_memory_mb", Requested: l.MaxMemoryMB,
			Applied: MaxMemoryMB, Reason: limitReasonCapped,
		})
		l.MaxMemoryMB = MaxMemoryMB
	}

	if l.MaxFuel <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "max_fuel", Requested: l.MaxFuel,
			Applied: DefaultMaxFuel, Reason: limitReasonDefault,
		})
		l.MaxFuel = DefaultMaxFuel
	}

	if l.MaxStorageMB <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "max_storage_mb", Requested: l.MaxStorageMB,
			Applied: DefaultMaxStorageMB, Reason: limitReasonDefault,
		})
		l.MaxStorageMB = DefaultMaxStorageMB
	}

	if l.MaxHTTPCallsPerMin <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "max_http_calls_per_minute", Requested: l.MaxHTTPCallsPerMin,
			Applied: DefaultMaxHTTPCallsPerMin, Reason: limitReasonDefault,
		})
		l.MaxHTTPCallsPerMin = DefaultMaxHTTPCallsPerMin
	}

	if l.TimeoutSyncMs <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "timeout_sync_ms", Requested: l.TimeoutSyncMs,
			Applied: DefaultSyncTimeoutMs, Reason: limitReasonDefault,
		})
		l.TimeoutSyncMs = DefaultSyncTimeoutMs
	}

	if l.TimeoutAsyncMs <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "timeout_async_ms", Requested: l.TimeoutAsyncMs,
			Applied: DefaultAsyncTimeoutMs, Reason: limitReasonDefault,
		})
		l.TimeoutAsyncMs = DefaultAsyncTimeoutMs
	}

	if l.PoolSize <= 0 {
		warnings = append(warnings, LimitWarning{
			Field: "pool_size", Requested: l.PoolSize,
			Applied: DefaultPoolSize, Reason: limitReasonDefault,
		})
		l.PoolSize = DefaultPoolSize
	} else if l.PoolSize > MaxPoolSize {
		warnings = append(warnings, LimitWarning{
			Field: "pool_size", Requested: l.PoolSize,
			Applied: MaxPoolSize, Reason: limitReasonCapped,
		})
		l.PoolSize = MaxPoolSize
	}

	return l, warnings
}

// configFieldTypeSecret is the manifest config field type that marks a value
// as sensitive. Values for secret fields are redacted in admin API responses
// and structured logs.
const configFieldTypeSecret = "secret"

// IsSecretField reports whether the config field identified by key is declared
// as type="secret" in the manifest. Returns false if the manifest or its
// config map is nil, or if the key is not declared.
func (m *Manifest) IsSecretField(key string) bool {
	if m == nil || m.Config == nil {
		return false
	}
	field, ok := m.Config[key]
	return ok && field.Type == configFieldTypeSecret
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
