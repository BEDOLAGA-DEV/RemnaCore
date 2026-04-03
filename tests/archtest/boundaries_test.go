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
