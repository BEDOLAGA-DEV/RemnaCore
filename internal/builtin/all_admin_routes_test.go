// Package builtin_test provides a systemic regression guard asserting that
// every AdminOnly route across ALL built-in plugins declares an explicit
// RequiredPermission. This closes finding HIGH-2 from the security audit
// (spec §13): "every admin/admin-plugin route is bound to a permission, no
// ungated route".
//
// Approach: blank-import each builtin plugin package to trigger its init()
// registration, then call builtin.AllPlugins() to get the full list. For
// every AdminOnly route assert RequiredPermission != "". Failure messages
// include plugin slug + method + path so the gap is immediately actionable.
//
// Adding a new built-in plugin: register it here with a blank import so the
// exhaustive check covers it automatically.
package builtin_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin"

	// Blank imports trigger each plugin's init(), which calls
	// builtin.RegisterPlugin(). Order does not matter; the registry is
	// thread-safe.
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/balance"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/checkout"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/remnawave"
	_ "github.com/BEDOLAGA-DEV/RemnaCore/internal/builtin/tariff"
)

// TestAllBuiltinAdminRoutes_ExplicitPermissions is the exhaustive regression
// guard. It iterates every route across every registered built-in plugin and
// asserts that any route with AdminOnly == true carries a non-empty
// RequiredPermission.
//
// An empty RequiredPermission on an AdminOnly route causes the route
// registrar (internal/gateway/plugin_routes.go) to fall back to the broad
// "plugins.manage" default, which is the ungated behaviour HIGH-2 targets.
//
// If this test fails, the failure message names the exact plugin slug, HTTP
// method, and path that needs to be remediated.
func TestAllBuiltinAdminRoutes_ExplicitPermissions(t *testing.T) {
	plugins := builtin.AllPlugins()

	// Require at least the four known plugins so a misconfigured test
	// environment (e.g. init() not running) is detected immediately rather
	// than silently passing with zero assertions.
	const minExpectedPlugins = 4
	assert.GreaterOrEqual(t, len(plugins), minExpectedPlugins,
		"expected at least %d built-in plugins in registry; got %d — "+
			"check that all blank imports in this file are present and correct",
		minExpectedPlugins, len(plugins))

	totalAdminRoutes := 0

	for _, p := range plugins {
		p := p // capture for subtest closure
		t.Run(p.Slug, func(t *testing.T) {
			for _, route := range p.Routes {
				if !route.AdminOnly {
					continue
				}
				totalAdminRoutes++
				route := route // capture for subtest closure
				t.Run(fmt.Sprintf("%s %s", route.Method, route.Path), func(t *testing.T) {
					assert.NotEmpty(t, route.RequiredPermission,
						"plugin %q: AdminOnly route %s %s has empty RequiredPermission — "+
							"this route will fall back to the broad 'plugins.manage' default "+
							"(HIGH-2 / spec §13). Assign an explicit RBAC permission.",
						p.Slug, route.Method, route.Path)
				})
			}
		})
	}

	// Surface the total count so it is visible in verbose test output.
	t.Logf("total AdminOnly routes asserted across %d built-in plugins: %d",
		len(plugins), totalAdminRoutes)

	// Guard against a vacuous pass: if every route's AdminOnly flag were ever
	// removed in a refactor the loop above would run zero assertions and the
	// test would pass silently. Require at least one AdminOnly route to exist.
	assert.Greater(t, totalAdminRoutes, 0,
		"no AdminOnly routes found across %d plugins — was AdminOnly removed from the route definitions?",
		len(plugins))
}

// TestAllBuiltinAdminRoutes_PluginCoverage asserts that all four known
// built-in plugin slugs are present in the registry. This prevents the
// systemic test from silently passing if a plugin's blank import is removed
// or its init() registration is accidentally deleted.
func TestAllBuiltinAdminRoutes_PluginCoverage(t *testing.T) {
	plugins := builtin.AllPlugins()

	slugs := make(map[string]bool, len(plugins))
	for _, p := range plugins {
		slugs[p.Slug] = true
	}

	requiredSlugs := []string{
		"tariff-manager",
		"remnawave-provider",
		"user-balance",
		"checkout",
	}

	for _, slug := range requiredSlugs {
		slug := slug
		t.Run(slug, func(t *testing.T) {
			assert.True(t, slugs[slug],
				"built-in plugin %q not found in registry — "+
					"add/restore its blank import in all_admin_routes_test.go",
				slug)
		})
	}
}
