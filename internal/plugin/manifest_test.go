package plugin

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fullManifestTOML = `
[plugin]
id          = "loyalty-points"
name        = "Loyalty Points"
version     = "1.0.0"
description = "Awards loyalty points on payment"
author      = "ACME Corp"
license     = "MIT"
sdk_version = "0.1.0"
lang        = "rust"

[permissions]
billing       = "read"
payment       = "write"
users         = "read"
notifications = "emit"
analytics     = "write"
storage       = "readwrite"
vpn           = "read"
api           = "routes"
http          = ["https://api.example.com/*"]

[hooks]
sync  = ["invoice.created", "payment.completed"]
async = ["subscription.renewed"]

[hooks.priority]
"invoice.created"      = 10
"payment.completed"    = 20

[config.webhook_url]
type     = "string"
label    = "Webhook URL"
required = true

[config.api_key]
type     = "secret"
label    = "API Key"
required = true

[config.mode]
type     = "select"
label    = "Mode"
required = false
default  = "sandbox"
options  = ["sandbox", "production"]

[limits]
max_memory_mb          = 128
max_fuel               = 2000000
max_storage_mb         = 200
max_http_calls_per_minute = 50
timeout_sync_ms        = 3000
timeout_async_ms       = 15000

[[pages]]
path  = "tariffs"
title = "Tariffs"
icon  = "CreditCard"
menu  = "admin"

[[pages]]
path  = "my-points"
title = "My Points"
icon  = "Star"
menu  = "cabinet"

[[routes]]
method   = "GET"
path     = "/api/tariffs"
function = "handle_list_tariffs"
public   = true

[[routes]]
method   = "POST"
path     = "/api/tariffs"
function = "handle_create_tariff"
public   = false
`

const minimalManifestTOML = `
[plugin]
id          = "minimal-plugin"
name        = "Minimal"
version     = "0.0.1"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]
`

func TestParseManifest_Full(t *testing.T) {
	m, err := ParseManifest([]byte(fullManifestTOML))
	require.NoError(t, err)

	assert.Equal(t, "loyalty-points", m.Plugin.ID)
	assert.Equal(t, "Loyalty Points", m.Plugin.Name)
	assert.Equal(t, "1.0.0", m.Plugin.Version)
	assert.Equal(t, "Awards loyalty points on payment", m.Plugin.Description)
	assert.Equal(t, "ACME Corp", m.Plugin.Author)
	assert.Equal(t, "MIT", m.Plugin.License)
	assert.Equal(t, "0.1.0", m.Plugin.SDKVersion)
	assert.Equal(t, "rust", m.Plugin.Lang)

	// Hooks
	assert.Equal(t, []string{"invoice.created", "payment.completed"}, m.Hooks.Sync)
	assert.Equal(t, []string{"subscription.renewed"}, m.Hooks.Async)
	assert.Equal(t, 10, m.Hooks.Priority["invoice.created"])
	assert.Equal(t, 20, m.Hooks.Priority["payment.completed"])

	// Permissions
	assert.Equal(t, "read", m.Permissions.Billing)
	assert.Equal(t, "write", m.Permissions.Payment)
	assert.Equal(t, "read", m.Permissions.Users)
	assert.Equal(t, "emit", m.Permissions.Notifications)
	assert.Equal(t, "write", m.Permissions.Analytics)
	assert.Equal(t, "readwrite", m.Permissions.Storage)
	assert.Equal(t, "read", m.Permissions.VPN)
	assert.Equal(t, "routes", m.Permissions.API)
	assert.Equal(t, []string{"https://api.example.com/*"}, m.Permissions.HTTP)

	// Config fields
	require.Contains(t, m.Config, "webhook_url")
	assert.Equal(t, "string", m.Config["webhook_url"].Type)
	assert.True(t, m.Config["webhook_url"].Required)

	require.Contains(t, m.Config, "api_key")
	assert.Equal(t, "secret", m.Config["api_key"].Type)

	require.Contains(t, m.Config, "mode")
	assert.Equal(t, "select", m.Config["mode"].Type)
	assert.Equal(t, "sandbox", m.Config["mode"].Default)
	assert.Equal(t, []string{"sandbox", "production"}, m.Config["mode"].Options)

	// Limits (explicitly set)
	assert.Equal(t, 128, m.Limits.MaxMemoryMB)
	assert.Equal(t, 2_000_000, m.Limits.MaxFuel)
	assert.Equal(t, 200, m.Limits.MaxStorageMB)
	assert.Equal(t, 50, m.Limits.MaxHTTPCallsPerMin)
	assert.Equal(t, 3000, m.Limits.TimeoutSyncMs)
	assert.Equal(t, 15000, m.Limits.TimeoutAsyncMs)

	// Pages
	require.Len(t, m.Pages, 2)
	assert.Equal(t, "tariffs", m.Pages[0].Path)
	assert.Equal(t, "Tariffs", m.Pages[0].Title)
	assert.Equal(t, "CreditCard", m.Pages[0].Icon)
	assert.Equal(t, PageMenuAdmin, m.Pages[0].Menu)
	assert.Equal(t, "my-points", m.Pages[1].Path)
	assert.Equal(t, "My Points", m.Pages[1].Title)
	assert.Equal(t, "Star", m.Pages[1].Icon)
	assert.Equal(t, PageMenuCabinet, m.Pages[1].Menu)

	// Routes
	require.Len(t, m.Routes, 2)
	assert.Equal(t, "GET", m.Routes[0].Method)
	assert.Equal(t, "/api/tariffs", m.Routes[0].Path)
	assert.Equal(t, "handle_list_tariffs", m.Routes[0].Function)
	assert.True(t, m.Routes[0].Public)
	assert.Equal(t, "POST", m.Routes[1].Method)
	assert.Equal(t, "/api/tariffs", m.Routes[1].Path)
	assert.Equal(t, "handle_create_tariff", m.Routes[1].Function)
	assert.False(t, m.Routes[1].Public)
}

func TestParseManifest_Minimal(t *testing.T) {
	m, err := ParseManifest([]byte(minimalManifestTOML))
	require.NoError(t, err)

	assert.Equal(t, "minimal-plugin", m.Plugin.ID)
	assert.Equal(t, "Minimal", m.Plugin.Name)
	assert.Equal(t, "0.0.1", m.Plugin.Version)
	assert.Equal(t, []string{"invoice.created"}, m.Hooks.Sync)
	assert.Empty(t, m.Hooks.Async)
	assert.Empty(t, m.Config)
	assert.Empty(t, m.Pages)
	assert.Empty(t, m.Routes)
}

func TestParseManifest_InvalidSlug(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"starts with hyphen", "-bad-slug"},
		{"starts with underscore", "_bad-slug"},
		{"contains uppercase", "Bad-Slug"},
		{"contains spaces", "bad slug"},
		{"contains special chars", "bad$slug"},
		{"empty", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tomlStr := `
[plugin]
id      = "` + tc.id + `"
name    = "Test"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]
`
			_, err := ParseManifest([]byte(tomlStr))
			require.Error(t, err)
		})
	}
}

func TestParseManifest_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{
			"missing id",
			`[plugin]
name    = "Test"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]`,
		},
		{
			"missing name",
			`[plugin]
id      = "test-plugin"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]`,
		},
		{
			"missing version",
			`[plugin]
id   = "test-plugin"
name = "Test"

[hooks]
sync = ["invoice.created"]`,
		},
		{
			"missing sdk_version",
			`[plugin]
id      = "test-plugin"
name    = "Test"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]`,
		},
		{
			"no hooks",
			`[plugin]
id          = "test-plugin"
name        = "Test"
version     = "1.0.0"
sdk_version = "1.0.0"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tc.toml))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidManifest)
		})
	}
}

func TestEffectiveLimits_FillsDefaults(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			ID: "test", Name: "Test", Version: "1.0.0",
		},
		Hooks: ManifestHooks{Sync: []string{"hook.a"}},
		// Limits left at zero values
	}

	eff, warnings := m.EffectiveLimits()

	assert.Equal(t, DefaultMaxMemoryMB, eff.MaxMemoryMB)
	assert.Equal(t, DefaultMaxFuel, eff.MaxFuel)
	assert.Equal(t, DefaultMaxStorageMB, eff.MaxStorageMB)
	assert.Equal(t, DefaultMaxHTTPCallsPerMin, eff.MaxHTTPCallsPerMin)
	assert.Equal(t, DefaultSyncTimeoutMs, eff.TimeoutSyncMs)
	assert.Equal(t, DefaultAsyncTimeoutMs, eff.TimeoutAsyncMs)

	// All fields were zero, so every field should produce a default warning.
	expectedFields := map[string]bool{
		"max_memory_mb":            false,
		"max_fuel":                 false,
		"max_storage_mb":           false,
		"max_http_calls_per_minute": false,
		"timeout_sync_ms":          false,
		"timeout_async_ms":         false,
		"pool_size":                false,
	}
	for _, w := range warnings {
		expectedFields[w.Field] = true
		assert.Equal(t, limitReasonDefault, w.Reason, "field %s", w.Field)
	}
	for field, found := range expectedFields {
		assert.True(t, found, "expected default warning for field %s", field)
	}
}

func TestEffectiveLimits_PreservesExplicitValues(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			ID: "test", Name: "Test", Version: "1.0.0",
		},
		Hooks: ManifestHooks{Sync: []string{"hook.a"}},
		Limits: ManifestLimits{
			MaxMemoryMB:        256,
			MaxFuel:            5_000_000,
			MaxStorageMB:       500,
			MaxHTTPCallsPerMin: 200,
			TimeoutSyncMs:      10000,
			TimeoutAsyncMs:     60000,
			PoolSize:           8,
		},
	}

	eff, warnings := m.EffectiveLimits()

	assert.Equal(t, 256, eff.MaxMemoryMB)
	assert.Equal(t, 5_000_000, eff.MaxFuel)
	assert.Equal(t, 500, eff.MaxStorageMB)
	assert.Equal(t, 200, eff.MaxHTTPCallsPerMin)
	assert.Equal(t, 10000, eff.TimeoutSyncMs)
	assert.Equal(t, 60000, eff.TimeoutAsyncMs)
	assert.Equal(t, 8, eff.PoolSize)
	assert.Empty(t, warnings, "explicit valid values should produce no warnings")
}

func TestEffectiveLimits_CappedAtMaximum(t *testing.T) {
	m := &Manifest{
		Plugin: ManifestPlugin{
			ID: "test", Name: "Test", Version: "1.0.0",
		},
		Hooks: ManifestHooks{Sync: []string{"hook.a"}},
		Limits: ManifestLimits{
			MaxMemoryMB:        MaxMemoryMB + 1024,
			MaxFuel:            5_000_000,
			MaxStorageMB:       500,
			MaxHTTPCallsPerMin: 200,
			TimeoutSyncMs:      10000,
			TimeoutAsyncMs:     60000,
			PoolSize:           MaxPoolSize + 8,
		},
	}

	eff, warnings := m.EffectiveLimits()

	assert.Equal(t, MaxMemoryMB, eff.MaxMemoryMB, "memory should be capped")
	assert.Equal(t, MaxPoolSize, eff.PoolSize, "pool size should be capped")

	// Exactly two capped warnings expected.
	cappedWarnings := make(map[string]LimitWarning)
	for _, w := range warnings {
		if w.Reason == limitReasonCapped {
			cappedWarnings[w.Field] = w
		}
	}
	assert.Len(t, cappedWarnings, 2)

	memWarn := cappedWarnings["max_memory_mb"]
	assert.Equal(t, MaxMemoryMB+1024, memWarn.Requested)
	assert.Equal(t, MaxMemoryMB, memWarn.Applied)

	poolWarn := cappedWarnings["pool_size"]
	assert.Equal(t, MaxPoolSize+8, poolWarn.Requested)
	assert.Equal(t, MaxPoolSize, poolWarn.Applied)
}

func TestParsePermissions(t *testing.T) {
	m, err := ParseManifest([]byte(fullManifestTOML))
	require.NoError(t, err)

	perms := m.ParsePermissions()

	assert.Contains(t, perms, PermBillingRead)
	assert.Contains(t, perms, PermPaymentWrite)
	assert.Contains(t, perms, PermUsersRead)
	assert.Contains(t, perms, PermNotificationsEmit)
	assert.Contains(t, perms, PermAnalyticsWrite)
	assert.Contains(t, perms, PermStorageRead)
	assert.Contains(t, perms, PermStorageWrite)
	assert.Contains(t, perms, PermVPNRead)
	assert.Contains(t, perms, PermAPIRoutes)

	// "billing: read" should NOT grant write
	assert.NotContains(t, perms, PermBillingWrite)
	// "users: read" should NOT grant write
	assert.NotContains(t, perms, PermUsersWrite)
	// "vpn: read" should NOT grant write
	assert.NotContains(t, perms, PermVPNWrite)
}

func TestHookRegistrations(t *testing.T) {
	m, err := ParseManifest([]byte(fullManifestTOML))
	require.NoError(t, err)

	pluginID := "test-uuid-1234"
	regs := m.HookRegistrations(pluginID)

	// 2 sync + 1 async = 3 total
	require.Len(t, regs, 3)

	// Check sync hooks
	invoiceReg := regs[0]
	assert.Equal(t, pluginID, invoiceReg.PluginID)
	assert.Equal(t, "loyalty-points", invoiceReg.PluginSlug)
	assert.Equal(t, "invoice.created", invoiceReg.HookName)
	assert.Equal(t, HookSync, invoiceReg.HookType)
	assert.Equal(t, 10, invoiceReg.Priority) // explicitly set

	paymentReg := regs[1]
	assert.Equal(t, "payment.completed", paymentReg.HookName)
	assert.Equal(t, HookSync, paymentReg.HookType)
	assert.Equal(t, 20, paymentReg.Priority) // explicitly set

	// Check async hook — should get default priority
	renewedReg := regs[2]
	assert.Equal(t, "subscription.renewed", renewedReg.HookName)
	assert.Equal(t, HookAsync, renewedReg.HookType)
	assert.Equal(t, DefaultPluginPriority, renewedReg.Priority)
}

func TestParseManifest_InvalidTOML(t *testing.T) {
	_, err := ParseManifest([]byte("this is not valid toml {{{}}}"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestParseManifest_SlugTooLong(t *testing.T) {
	longSlug := "a"
	for len(longSlug) <= MaxPluginSlugLen {
		longSlug += "a"
	}

	tomlStr := `
[plugin]
id      = "` + longSlug + `"
name    = "Test"
version = "1.0.0"

[hooks]
sync = ["invoice.created"]
`
	_, err := ParseManifest([]byte(tomlStr))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidPluginSlug)
}

func TestValidateHookName(t *testing.T) {
	tests := []struct {
		name    string
		hook    string
		wantErr bool
	}{
		// Allowed prefixes
		{"ab prefix", "ab.variant_assigned", false},
		{"binding prefix", "binding.created", false},
		{"checkout prefix", "checkout.started", false},
		{"cohort prefix", "cohort.recalculated", false},
		{"invoice prefix", "invoice.created", false},
		{"notification prefix", "notification.sent", false},
		{"payment prefix", "payment.completed", false},
		{"plugin prefix", "plugin.installed", false},
		{"pricing prefix", "pricing.calculated", false},
		{"promo prefix", "promo.code_used", false},
		{"subscription prefix", "subscription.renewed", false},
		{"tariff prefix", "tariff.purchased", false},
		{"user prefix", "user.resolve_groups", false},
		{"vpn prefix", "vpn.provisioned", false},

		// Rejected prefixes
		{"admin prefix rejected", "admin.reset", true},
		{"system prefix rejected", "system.shutdown", true},
		{"empty string rejected", "", true},
		{"bare word rejected", "shutdown", true},
		{"internal prefix rejected", "internal.migrate", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHookName(tt.hook)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrInvalidManifest)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseManifest_RejectsUnknownHookPrefix(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{
			"unknown sync hook prefix",
			`[plugin]
id          = "bad-hooks"
name        = "Bad Hooks"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["admin.reset"]`,
		},
		{
			"unknown async hook prefix",
			`[plugin]
id          = "bad-hooks"
name        = "Bad Hooks"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
async = ["system.shutdown"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tt.toml))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidManifest)
			assert.Contains(t, err.Error(), "unknown prefix")
		})
	}
}

func TestValidatePluginVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"major.minor.patch", "1.0.0", false},
		{"major.minor", "1.0", false},
		{"zero version", "0.0.1", false},
		{"large numbers", "10.20.30", false},
		{"single number", "1", true},
		{"four components", "1.0.0.0", true},
		{"non-numeric major", "abc.0.0", true},
		{"non-numeric minor", "1.x.0", true},
		{"non-numeric patch", "1.0.beta", true},
		{"empty string", "", true},
		{"v prefix", "v1.0.0", true},
		{"spaces", "1. 0.0", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePluginVersion(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseManifest_RejectsInvalidVersion(t *testing.T) {
	tomlStr := `
[plugin]
id          = "bad-version"
name        = "Bad Version"
version     = "not-semver"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]
`
	_, err := ParseManifest([]byte(tomlStr))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
}

func TestParseManifest_AcceptsAllowedHookPrefixes(t *testing.T) {
	for _, prefix := range AllowedHookPrefixes {
		hookName := prefix + "test_event"
		t.Run(hookName, func(t *testing.T) {
			tomlStr := `
[plugin]
id          = "prefix-test"
name        = "Prefix Test"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["` + hookName + `"]
`
			_, err := ParseManifest([]byte(tomlStr))
			assert.NoError(t, err)
		})
	}
}

func TestParseManifest_InvalidPages(t *testing.T) {
	baseManifest := `
[plugin]
id          = "page-test"
name        = "Page Test"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]

`

	tests := []struct {
		name     string
		pageTOML string
		errMsg   string
	}{
		{
			"missing path",
			`[[pages]]
title = "Settings"
menu  = "admin"
`,
			"pages[0].path is required",
		},
		{
			"missing title",
			`[[pages]]
path = "settings"
menu = "admin"
`,
			"pages[0].title is required",
		},
		{
			"missing menu",
			`[[pages]]
path  = "settings"
title = "Settings"
`,
			"pages[0].menu is required",
		},
		{
			"invalid menu value",
			`[[pages]]
path  = "settings"
title = "Settings"
menu  = "sidebar"
`,
			`must be "admin" or "cabinet"`,
		},
		{
			"path starts with hyphen",
			`[[pages]]
path  = "-bad"
title = "Bad"
menu  = "admin"
`,
			"pages[0].path must match",
		},
		{
			"path contains uppercase",
			`[[pages]]
path  = "Bad"
title = "Bad"
menu  = "admin"
`,
			"pages[0].path must match",
		},
		{
			"path contains spaces",
			`[[pages]]
path  = "my page"
title = "My Page"
menu  = "admin"
`,
			"pages[0].path must match",
		},
		{
			"second page invalid",
			`[[pages]]
path  = "good"
title = "Good"
menu  = "admin"

[[pages]]
path  = "also-good"
title = ""
menu  = "cabinet"
`,
			"pages[1].title is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(baseManifest + tc.pageTOML))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidManifest)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestParseManifest_ValidPages(t *testing.T) {
	tests := []struct {
		name     string
		pageTOML string
		expected []ManifestPage
	}{
		{
			"single admin page",
			`[[pages]]
path  = "dashboard"
title = "Dashboard"
icon  = "LayoutDashboard"
menu  = "admin"
`,
			[]ManifestPage{
				{Path: "dashboard", Title: "Dashboard", Icon: "LayoutDashboard", Menu: PageMenuAdmin},
			},
		},
		{
			"cabinet page without icon",
			`[[pages]]
path  = "my-vpn"
title = "My VPN"
menu  = "cabinet"
`,
			[]ManifestPage{
				{Path: "my-vpn", Title: "My VPN", Icon: "", Menu: PageMenuCabinet},
			},
		},
		{
			"path with digits",
			`[[pages]]
path  = "v2-settings"
title = "V2 Settings"
menu  = "admin"
`,
			[]ManifestPage{
				{Path: "v2-settings", Title: "V2 Settings", Icon: "", Menu: PageMenuAdmin},
			},
		},
		{
			"path starting with digit",
			`[[pages]]
path  = "2fa"
title = "2FA"
menu  = "cabinet"
`,
			[]ManifestPage{
				{Path: "2fa", Title: "2FA", Icon: "", Menu: PageMenuCabinet},
			},
		},
	}

	baseManifest := `
[plugin]
id          = "page-test"
name        = "Page Test"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]

`
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseManifest([]byte(baseManifest + tc.pageTOML))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, m.Pages)
		})
	}
}

func TestParseManifest_ValidRoutes(t *testing.T) {
	baseManifest := `
[plugin]
id          = "route-test"
name        = "Route Test"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]

`

	tests := []struct {
		name      string
		routeTOML string
		expected  []ManifestRoute
	}{
		{
			"single public GET route",
			`[[routes]]
method   = "GET"
path     = "/api/tariffs"
function = "handle_list"
public   = true
`,
			[]ManifestRoute{
				{Method: "GET", Path: "/api/tariffs", Function: "handle_list", Public: true},
			},
		},
		{
			"protected POST route",
			`[[routes]]
method   = "POST"
path     = "/api/orders"
function = "handle_create_order"
public   = false
`,
			[]ManifestRoute{
				{Method: "POST", Path: "/api/orders", Function: "handle_create_order", Public: false},
			},
		},
		{
			"all HTTP methods",
			`[[routes]]
method   = "GET"
path     = "/api/items"
function = "list_items"
public   = true

[[routes]]
method   = "POST"
path     = "/api/items"
function = "create_item"

[[routes]]
method   = "PUT"
path     = "/api/items"
function = "update_item"

[[routes]]
method   = "DELETE"
path     = "/api/items"
function = "delete_item"

[[routes]]
method   = "PATCH"
path     = "/api/items"
function = "patch_item"
`,
			[]ManifestRoute{
				{Method: "GET", Path: "/api/items", Function: "list_items", Public: true},
				{Method: "POST", Path: "/api/items", Function: "create_item"},
				{Method: "PUT", Path: "/api/items", Function: "update_item"},
				{Method: "DELETE", Path: "/api/items", Function: "delete_item"},
				{Method: "PATCH", Path: "/api/items", Function: "patch_item"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseManifest([]byte(baseManifest + tc.routeTOML))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, m.Routes)
		})
	}
}

func TestParseManifest_InvalidRoutes(t *testing.T) {
	baseManifest := `
[plugin]
id          = "route-test"
name        = "Route Test"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]

`

	tests := []struct {
		name      string
		routeTOML string
		errMsg    string
	}{
		{
			"missing method",
			`[[routes]]
path     = "/api/items"
function = "handle_list"
`,
			"routes[0].method is required",
		},
		{
			"invalid method",
			`[[routes]]
method   = "OPTIONS"
path     = "/api/items"
function = "handle_list"
`,
			"routes[0].method must be one of",
		},
		{
			"lowercase method rejected",
			`[[routes]]
method   = "get"
path     = "/api/items"
function = "handle_list"
`,
			"routes[0].method must be one of",
		},
		{
			"missing path",
			`[[routes]]
method   = "GET"
function = "handle_list"
`,
			"routes[0].path is required",
		},
		{
			"path without /api/ prefix",
			`[[routes]]
method   = "GET"
path     = "/tariffs"
function = "handle_list"
`,
			`routes[0].path must start with "/api/"`,
		},
		{
			"reserved path /api/admin/",
			`[[routes]]
method   = "GET"
path     = "/api/admin/plugins"
function = "handle_list"
`,
			`routes[0].path "/api/admin/plugins" uses reserved prefix "/api/admin"`,
		},
		{
			"reserved path /api/auth/",
			`[[routes]]
method   = "POST"
path     = "/api/auth/login"
function = "handle_login"
`,
			`routes[0].path "/api/auth/login" uses reserved prefix "/api/auth"`,
		},
		{
			"reserved path /api/me",
			`[[routes]]
method   = "GET"
path     = "/api/me"
function = "handle_me"
`,
			`routes[0].path "/api/me" uses reserved prefix "/api/me"`,
		},
		{
			"reserved path /api/webhooks/",
			`[[routes]]
method   = "POST"
path     = "/api/webhooks/custom"
function = "handle_webhook"
`,
			`routes[0].path "/api/webhooks/custom" uses reserved prefix "/api/webhooks"`,
		},
		{
			"missing function",
			`[[routes]]
method   = "GET"
path     = "/api/items"
`,
			"routes[0].function is required",
		},
		{
			"invalid function name with uppercase",
			`[[routes]]
method   = "GET"
path     = "/api/items"
function = "HandleList"
`,
			"routes[0].function must match",
		},
		{
			"invalid function name with hyphen",
			`[[routes]]
method   = "GET"
path     = "/api/items"
function = "handle-list"
`,
			"routes[0].function must match",
		},
		{
			"duplicate method+path",
			`[[routes]]
method   = "GET"
path     = "/api/items"
function = "handle_list_v1"

[[routes]]
method   = "GET"
path     = "/api/items"
function = "handle_list_v2"
`,
			"routes[1]: duplicate GET /api/items",
		},
		{
			"second route invalid",
			`[[routes]]
method   = "GET"
path     = "/api/items"
function = "handle_list"

[[routes]]
method   = "POST"
path     = "/api/items"
function = ""
`,
			"routes[1].function is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(baseManifest + tc.routeTOML))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidManifest)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestParseManifest_ExceedMaxRoutes(t *testing.T) {
	baseManifest := `
[plugin]
id          = "route-test"
name        = "Route Test"
version     = "1.0.0"
sdk_version = "1.0.0"

[hooks]
sync = ["invoice.created"]

`
	// Build TOML with MaxRoutesPerPlugin + 1 routes.
	var routesTOML string
	for i := 0; i <= MaxRoutesPerPlugin; i++ {
		routesTOML += fmt.Sprintf(`
[[routes]]
method   = "GET"
path     = "/api/route%d"
function = "handle_%d"
`, i, i)
	}

	_, err := ParseManifest([]byte(baseManifest + routesTOML))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidManifest)
	assert.Contains(t, err.Error(), "too many routes")
}

func TestValidateManifestRoute_ReservedPrefixMatching(t *testing.T) {
	reject := []string{
		"/api/admin", "/api/admin/roles", "/api/Admin/roles",
		"/api/auth", "/api/AUTH/login",
		"/api/me", "/api/me/profile",
		"/api/webhooks", "/api/webhooks/stripe",
	}
	accept := []string{
		"/api/members", "/api/messages", "/api/metrics",
		"/api/administrative-notes", "/api/plugin-foo",
	}
	for _, p := range reject {
		err := validateManifestRoute(0, ManifestRoute{Method: "GET", Path: p, Function: "fn"})
		require.ErrorIs(t, err, ErrInvalidManifest, "expected %q rejected as reserved", p)
	}
	for _, p := range accept {
		err := validateManifestRoute(0, ManifestRoute{Method: "GET", Path: p, Function: "fn"})
		require.NoError(t, err, "expected %q accepted", p)
	}
}
