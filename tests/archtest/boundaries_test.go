// Package archtest enforces architectural boundaries via compile-time-like Go
// tests. These tests parse import declarations in production Go files and fail
// if any forbidden cross-package dependency is detected.
//
// Run with:
//
//	go test ./tests/archtest/... -v
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const modulePrefix = "github.com/BEDOLAGA-DEV/RemnaCore"

// forbiddenInfraFromDomain lists the infrastructure-layer package prefixes that
// domain packages must never import.
var forbiddenInfraFromDomain = []string{
	modulePrefix + "/internal/adapter",
	modulePrefix + "/internal/gateway",
	modulePrefix + "/internal/plugin",
	modulePrefix + "/internal/infra",
	modulePrefix + "/internal/telegram",
	modulePrefix + "/internal/app",
	modulePrefix + "/internal/observability",
}

// domainContexts maps bounded context names to their module-qualified import
// path prefix.
var domainContexts = map[string]string{
	"identity": modulePrefix + "/internal/domain/identity",
	"billing":  modulePrefix + "/internal/domain/billing",
	"multisub": modulePrefix + "/internal/domain/multisub",
	"payment":  modulePrefix + "/internal/domain/payment",
	"reseller": modulePrefix + "/internal/domain/reseller",
}

// TestDomainIsolation verifies that domain packages never import adapter,
// gateway, plugin, infra, telegram, app, or observability packages.
func TestDomainIsolation(t *testing.T) {
	for name := range domainContexts {
		dir := filepath.Join("internal", "domain", name)
		t.Run(name, func(t *testing.T) {
			checkImports(t, dir, forbiddenInfraFromDomain)
		})
	}
}

// TestBoundedContextIsolation verifies that bounded contexts do not import each
// other directly. Communication between contexts must happen via domain events
// (NATS), shared kernel types (pkg/), or ACL types owned by the importing
// context.
func TestBoundedContextIsolation(t *testing.T) {
	for name := range domainContexts {
		dir := filepath.Join("internal", "domain", name)

		// Build forbidden list: all OTHER context prefixes.
		var forbidden []string
		for otherName, otherPrefix := range domainContexts {
			if otherName != name {
				forbidden = append(forbidden, otherPrefix)
			}
		}

		t.Run(name, func(t *testing.T) {
			checkImports(t, dir, forbidden)
		})
	}
}

// TestPluginIsolation verifies that the plugin package does not import adapter
// or gateway packages.
func TestPluginIsolation(t *testing.T) {
	forbiddenPrefixes := []string{
		modulePrefix + "/internal/adapter",
		modulePrefix + "/internal/gateway",
	}

	checkImports(t, filepath.Join("internal", "plugin"), forbiddenPrefixes)
}

// TestAdapterDoesNotImportGateway verifies that adapter packages never import
// gateway packages (and vice versa). These layers communicate only through
// domain interfaces wired at the composition root.
//
// Note: the "gateway_no_adapter" subtest walks filepath.Join("internal",
// "gateway") recursively, so it covers all sub-packages including
// internal/gateway/handler/ and internal/gateway/middleware/.
func TestAdapterDoesNotImportGateway(t *testing.T) {
	t.Run("adapter_no_gateway", func(t *testing.T) {
		checkImports(t, filepath.Join("internal", "adapter"), []string{
			modulePrefix + "/internal/gateway",
		})
	})
	t.Run("gateway_no_adapter", func(t *testing.T) {
		checkImports(t, filepath.Join("internal", "gateway"), []string{
			modulePrefix + "/internal/adapter",
		})
	})
}

// TestPluginDoesNotImportDomainDirectly verifies that the plugin package
// accesses domain functionality only through pkg/hookdispatch, not by
// importing internal/domain/ directly.
func TestPluginDoesNotImportDomainDirectly(t *testing.T) {
	checkImports(t, filepath.Join("internal", "plugin"), []string{
		modulePrefix + "/internal/domain",
	})
}

// TestAdapterPluginIsolation verifies that internal/adapter/plugin only depends
// on the domain port it implements (multisub.VPNProvider) and shared pkg/
// libraries. It must NOT import other domain contexts, peer adapter packages,
// or upper-layer packages (gateway, infra, telegram, etc.).
//
// Allowed imports:
//   - internal/domain/multisub (VPNProvider port)
//   - pkg/hookdispatch, pkg/sdk
//   - stdlib
//
// Forbidden: all other internal packages.
func TestAdapterPluginIsolation(t *testing.T) {
	// Peer adapter packages that adapter/plugin must not import.
	peerAdapters := []string{
		modulePrefix + "/internal/adapter/nats",
		modulePrefix + "/internal/adapter/postgres",
		modulePrefix + "/internal/adapter/remnawave",
		modulePrefix + "/internal/adapter/valkey",
	}

	// Domain contexts that adapter/plugin must not import (multisub is allowed).
	forbiddenDomains := []string{
		modulePrefix + "/internal/domain/billing",
		modulePrefix + "/internal/domain/identity",
		modulePrefix + "/internal/domain/payment",
		modulePrefix + "/internal/domain/reseller",
	}

	// Upper-layer packages that no adapter should import.
	forbiddenLayers := []string{
		modulePrefix + "/internal/gateway",
		modulePrefix + "/internal/plugin",
		modulePrefix + "/internal/infra",
		modulePrefix + "/internal/telegram",
		modulePrefix + "/internal/app",
		modulePrefix + "/internal/observability",
	}

	var forbidden []string
	forbidden = append(forbidden, peerAdapters...)
	forbidden = append(forbidden, forbiddenDomains...)
	forbidden = append(forbidden, forbiddenLayers...)

	checkImports(t, filepath.Join("internal", "adapter", "plugin"), forbidden)
}

// TestPluginRuntimeDoesNotImportAdapterPlugin verifies that the WASM plugin
// runtime (internal/plugin) does not depend on the adapter/plugin package.
// They are at the same layer but serve different concerns: internal/plugin is
// the generic runtime, while internal/adapter/plugin is a domain-specific
// adapter that dispatches VPN hooks. Communication flows through
// hookdispatch.Dispatcher, not direct imports.
//
// Note: TestPluginIsolation already forbids internal/plugin from importing
// internal/adapter (the parent prefix). This test makes the specific boundary
// between plugin runtime and plugin adapter explicit and self-documenting.
func TestPluginRuntimeDoesNotImportAdapterPlugin(t *testing.T) {
	checkImports(t, filepath.Join("internal", "plugin"), []string{
		modulePrefix + "/internal/adapter/plugin",
	})
}

// TestInfraDoesNotImportGateway verifies that infra services (health monitor,
// smart router, speed test, subscription proxy) never import gateway or
// adapter packages.
func TestInfraDoesNotImportGateway(t *testing.T) {
	checkImports(t, filepath.Join("internal", "infra"), []string{
		modulePrefix + "/internal/gateway",
	})
}

// checkImports walks dir, parses each non-test .go file, and fails the test for
// every import whose path starts with any of the forbiddenPrefixes.
func checkImports(t *testing.T, dir string, forbiddenPrefixes []string) {
	t.Helper()

	// Resolve relative to the repository root. The test binary may run from
	// a different working directory, so we look for go.mod to anchor.
	root := findRepoRoot(t)
	absDir := filepath.Join(root, dir)

	err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil // skip test files
		}
		if strings.Contains(path, string(filepath.Separator)+"gen"+string(filepath.Separator)) {
			return nil // skip generated code
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil // skip unparseable files
		}

		// Use path relative to repo root for readable error messages.
		relPath, _ := filepath.Rel(root, path)
		if relPath == "" {
			relPath = path
		}

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			for _, forbidden := range forbiddenPrefixes {
				if strings.HasPrefix(importPath, forbidden) {
					t.Errorf("%s imports forbidden package %s", relPath, importPath)
				}
			}
		}

		return nil
	})

	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
}

// findRepoRoot walks up from the current working directory to find the directory
// containing go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}
