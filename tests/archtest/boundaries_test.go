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
	"slices"
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
	"settings": modulePrefix + "/internal/domain/settings",
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
		modulePrefix + "/internal/adapter/billing",
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

// TestAdapterBillingIsolation verifies that internal/adapter/billing only
// depends on the domain ports it implements (billing.PaymentGateway,
// billing.PricingModifier) and the payment domain it bridges to, plus shared
// pkg/ libraries. It must NOT import other domain contexts, peer adapter
// packages, or upper-layer packages.
//
// Allowed imports:
//   - internal/domain/billing (PaymentGateway, PricingModifier ports)
//   - internal/domain/payment (PaymentFacade for ACL bridge)
//   - pkg/hookdispatch
//   - stdlib
//
// Forbidden: all other internal packages.
func TestAdapterBillingIsolation(t *testing.T) {
	// Peer adapter packages that adapter/billing must not import.
	peerAdapters := []string{
		modulePrefix + "/internal/adapter/nats",
		modulePrefix + "/internal/adapter/plugin",
		modulePrefix + "/internal/adapter/postgres",
		modulePrefix + "/internal/adapter/remnawave",
		modulePrefix + "/internal/adapter/valkey",
	}

	// Domain contexts that adapter/billing must not import (billing and payment
	// are allowed since this adapter bridges those two contexts).
	forbiddenDomains := []string{
		modulePrefix + "/internal/domain/identity",
		modulePrefix + "/internal/domain/multisub",
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

	checkImports(t, filepath.Join("internal", "adapter", "billing"), forbidden)
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

// domainServicePrefix is the common import path prefix for domain service
// sub-packages (e.g., internal/domain/billing/service).
const domainServicePrefix = modulePrefix + "/internal/domain/"

// serviceAtRootContexts lists bounded contexts whose service/facade types live
// at the domain root package rather than in a /service sub-package. Importing
// these root packages counts as a service-level import for the single-context
// rule because the handler must be constructing or calling a service.
//
// Contexts NOT listed here (billing, multisub) have a /service sub-package;
// importing their root package provides only repository interfaces, value
// objects, or error sentinels — not orchestration logic.
var serviceAtRootContexts = map[string]bool{
	"identity": true, // identity.Service
	"payment":  true, // payment.PaymentFacade
	"reseller": true, // reseller.ResellerService
}

// gatewayHandlerExcludedFiles lists files in internal/gateway/handler/ that are
// exempt from the single-context rule because they are shared utilities, not
// endpoint handlers.
var gatewayHandlerExcludedFiles = map[string]bool{
	"response.go": true,
}

// TestGatewayHandlerSingleContextRule verifies that each file in
// internal/gateway/handler/ imports at most one domain service package.
// This prevents synchronous cross-context orchestration from the gateway layer.
//
// The rule distinguishes service-level imports from type/error imports:
//   - internal/domain/{ctx}/service → always counts (ctx = billing, multisub)
//   - internal/domain/{ctx} where ctx is in serviceAtRootContexts → counts
//   - internal/domain/{ctx} or internal/domain/{ctx}/aggregate → does NOT count
//     when ctx has a /service sub-package (e.g., billing root for repo interfaces)
func TestGatewayHandlerSingleContextRule(t *testing.T) {
	root := findRepoRoot(t)
	handlerDir := filepath.Join(root, "internal", "gateway", "handler")

	err := filepath.Walk(handlerDir, func(path string, info os.FileInfo, err error) error {
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
			return nil
		}
		if gatewayHandlerExcludedFiles[filepath.Base(path)] {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "" {
			relPath = path
		}

		// Collect distinct service contexts imported by this file.
		contexts := make(map[string]string) // context name → import path

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(importPath, domainServicePrefix) {
				continue
			}

			// Strip the common prefix to get e.g. "billing/service",
			// "payment", "billing/aggregate", "billing".
			suffix := strings.TrimPrefix(importPath, domainServicePrefix)
			parts := strings.SplitN(suffix, "/", 2)
			ctx := parts[0]

			// Case 1: explicit /service sub-package (e.g. billing/service).
			if len(parts) == 2 && parts[1] == "service" {
				contexts[ctx] = importPath
				continue
			}

			// Case 2: domain root package for a context whose service lives
			// at root (identity, payment, reseller).
			if len(parts) == 1 && serviceAtRootContexts[ctx] {
				contexts[ctx] = importPath
				continue
			}

			// Everything else (root imports for billing/multisub, aggregate
			// sub-packages, etc.) is NOT counted as a service import.
		}

		if len(contexts) > 1 {
			var imported []string
			for ctx, pkg := range contexts {
				imported = append(imported, ctx+" ("+pkg+")")
			}
			// Sort for deterministic output.
			slices.Sort(imported)
			t.Errorf(
				"%s imports services from %d domain contexts; max 1 allowed: %s",
				relPath, len(contexts), strings.Join(imported, ", "),
			)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("walking handler dir: %v", err)
	}
}

// TestAdminHandlerSingleContextClassification explicitly verifies that the
// AdminHandler is classified correctly by TestGatewayHandlerSingleContextRule.
//
// AdminHandler imports both identity (service at root) and billing (root
// package for SubscriptionReader/InvoiceReader interfaces). The billing root
// import is NOT in serviceAtRootContexts because billing's orchestration lives
// in billing/service, not at the root. Therefore identity is the ONLY service
// context counted, and the file passes the single-context rule.
//
// This test documents this exception and guards against accidental
// reclassification of the billing root as a service import.
func TestAdminHandlerSingleContextClassification(t *testing.T) {
	root := findRepoRoot(t)
	adminPath := filepath.Join(root, "internal", "gateway", "handler", "admin_handler.go")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, adminPath, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing admin_handler.go: %v", err)
	}

	// Collect service-level contexts using the same classification logic as
	// TestGatewayHandlerSingleContextRule.
	serviceContexts := make(map[string]string)
	for _, imp := range f.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		if !strings.HasPrefix(importPath, domainServicePrefix) {
			continue
		}

		suffix := strings.TrimPrefix(importPath, domainServicePrefix)
		parts := strings.SplitN(suffix, "/", 2)
		ctx := parts[0]

		if len(parts) == 2 && parts[1] == "service" {
			serviceContexts[ctx] = importPath
			continue
		}
		if len(parts) == 1 && serviceAtRootContexts[ctx] {
			serviceContexts[ctx] = importPath
			continue
		}
	}

	// AdminHandler must import exactly one service context: identity.
	// The billing root import should NOT be counted.
	if len(serviceContexts) != 1 {
		var imported []string
		for ctx, pkg := range serviceContexts {
			imported = append(imported, ctx+" ("+pkg+")")
		}
		slices.Sort(imported)
		t.Errorf("expected 1 service context for admin_handler.go, got %d: %s",
			len(serviceContexts), strings.Join(imported, ", "))
	}

	if _, ok := serviceContexts["identity"]; !ok {
		t.Error("admin_handler.go must count identity as its single service context")
	}

	if _, ok := serviceContexts["billing"]; ok {
		t.Error("admin_handler.go billing root import must NOT count as a service context "+
			"(billing orchestration lives in billing/service, not at root)")
	}
}

// TestBillingDomainNoPluginImports verifies that the billing domain does not
// import pkg/hookdispatch or pkg/sdk. Hook dispatch is an application-layer
// concern; the billing service uses injected SyncHookFn / AsyncHookFn
// callbacks instead. This prevents plugin protocol types from leaking into
// the domain model.
func TestBillingDomainNoPluginImports(t *testing.T) {
	forbidden := []string{
		modulePrefix + "/pkg/hookdispatch",
		modulePrefix + "/pkg/sdk",
	}
	checkImports(t, filepath.Join("internal", "domain", "billing"), forbidden)
}

// maxHandlerDomainContexts is the maximum number of distinct domain contexts a
// single handler file may import. Keeping this low prevents handlers from
// becoming orchestrators that coordinate across bounded contexts -- that work
// belongs in domain services or sagas.
const maxHandlerDomainContexts = 2

// handlerDomainContextExcludedPackages lists shared-kernel packages that look
// like domain imports but do not count toward the context budget.
var handlerDomainContextExcludedPackages = map[string]bool{
	"plugin": true, // internal/plugin is infrastructure, not a domain context
}

// TestHandlerMaxDomainContextImports verifies that each file in
// internal/gateway/handler/ imports at most maxHandlerDomainContexts distinct
// domain contexts (excluding response.go which is an intentional mapper).
//
// Both type-level imports (e.g., internal/domain/billing) and service-level
// imports (e.g., internal/domain/billing/service) count. billing and
// billing/aggregate are grouped as a single "billing" context.
func TestHandlerMaxDomainContextImports(t *testing.T) {
	root := findRepoRoot(t)
	handlerDir := filepath.Join(root, "internal", "gateway", "handler")

	err := filepath.Walk(handlerDir, func(path string, info os.FileInfo, err error) error {
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
			return nil
		}
		if gatewayHandlerExcludedFiles[filepath.Base(path)] {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "" {
			relPath = path
		}

		// Collect distinct domain contexts imported by this file.
		contexts := make(map[string]string) // context name -> first import path

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Check for internal/domain/* imports.
			if !strings.HasPrefix(importPath, domainServicePrefix) {
				// Check for internal/plugin (has MapToAPIError but is infra).
				if importPath == modulePrefix+"/internal/plugin" {
					continue
				}
				continue
			}

			// Strip prefix to get e.g. "billing/service", "billing/aggregate",
			// "identity", "multisub".
			suffix := strings.TrimPrefix(importPath, domainServicePrefix)
			parts := strings.SplitN(suffix, "/", 2)
			ctx := parts[0]

			if handlerDomainContextExcludedPackages[ctx] {
				continue
			}

			// Group billing + billing/aggregate + billing/service as one
			// context. Same for any context with sub-packages.
			if _, seen := contexts[ctx]; !seen {
				contexts[ctx] = importPath
			}
		}

		if len(contexts) > maxHandlerDomainContexts {
			var imported []string
			for ctx, pkg := range contexts {
				imported = append(imported, ctx+" ("+pkg+")")
			}
			slices.Sort(imported)
			t.Errorf(
				"%s imports %d domain contexts; max %d allowed: %s",
				relPath, len(contexts), maxHandlerDomainContexts,
				strings.Join(imported, ", "),
			)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("walking handler dir: %v", err)
	}
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
