package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/app"
)

// TestContextMapNotEmpty verifies that the context map has entries. An empty
// map means someone deleted all entries without updating the tests.
func TestContextMapNotEmpty(t *testing.T) {
	if len(app.ContextMap) == 0 {
		t.Fatal("ContextMap is empty; it must declare all cross-context communication paths")
	}
}

// TestContextMapEntriesValid checks that every ContextMap entry has required
// fields populated and uses a recognised mechanism.
func TestContextMapEntriesValid(t *testing.T) {
	validMechanisms := map[app.Mechanism]bool{
		app.MechanismEvent:    true,
		app.MechanismPort:     true,
		app.MechanismACL:      true,
		app.MechanismGateway:  true,
		app.MechanismExternal: true,
		app.MechanismPlugin:   true,
	}

	for i, dep := range app.ContextMap {
		t.Run(dep.From+"->"+dep.To, func(t *testing.T) {
			if dep.From == "" {
				t.Errorf("ContextMap[%d]: From must not be empty", i)
			}
			if dep.To == "" {
				t.Errorf("ContextMap[%d]: To must not be empty", i)
			}
			if dep.From == dep.To {
				t.Errorf("ContextMap[%d]: From and To must be different (both are %q)", i, dep.From)
			}
			if !validMechanisms[dep.Mechanism] {
				t.Errorf("ContextMap[%d]: unrecognised mechanism %q", i, dep.Mechanism)
			}
			if dep.Description == "" {
				t.Errorf("ContextMap[%d]: Description must not be empty", i)
			}
		})
	}
}

// TestContextMapNoDuplicates checks that no two entries have the same
// (From, To, Mechanism) triple. Duplicate entries indicate copy-paste errors.
func TestContextMapNoDuplicates(t *testing.T) {
	type key struct {
		from, to  string
		mechanism app.Mechanism
	}
	seen := make(map[key]int) // key -> first index

	for i, dep := range app.ContextMap {
		k := key{dep.From, dep.To, dep.Mechanism}
		if firstIdx, ok := seen[k]; ok {
			t.Errorf("ContextMap[%d] duplicates entry [%d]: %s->%s via %s",
				i, firstIdx, dep.From, dep.To, dep.Mechanism)
		} else {
			seen[k] = i
		}
	}
}

// TestContextMapMatchesRealImports verifies that every cross-context domain
// import found in the codebase is declared in ContextMap. This test catches
// the case where a developer adds a new synchronous call between bounded
// contexts without updating the context map.
//
// Scope: production Go files under internal/domain/*/. Test files are excluded
// because test helpers may import other contexts for fixture construction.
//
// What counts as a cross-context import:
//   - internal/domain/X/** imports internal/domain/Y/** where X != Y
//
// The test verifies these imports against AllowedSyncImports(). If a new
// cross-context domain import is added, the developer must either:
//   - Add a corresponding entry to ContextMap (port or acl mechanism), or
//   - Remove the cross-context import (preferred: use events or ACL adapters).
func TestContextMapMatchesRealImports(t *testing.T) {
	root := findRepoRoot(t)
	allowed := app.AllowedSyncImports()

	for ctxName := range domainContexts {
		ctxDir := filepath.Join(root, "internal", "domain", ctxName)

		err := filepath.Walk(ctxDir, func(path string, info os.FileInfo, err error) error {
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
			if strings.Contains(path, string(filepath.Separator)+"gen"+string(filepath.Separator)) {
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

			for _, imp := range f.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)

				// Only interested in domain imports.
				if !strings.HasPrefix(importPath, domainServicePrefix) {
					continue
				}

				// Extract the target context name from the import path.
				suffix := strings.TrimPrefix(importPath, domainServicePrefix)
				parts := strings.SplitN(suffix, "/", 2)
				targetCtx := parts[0]

				// Skip self-imports (same bounded context).
				if targetCtx == ctxName {
					continue
				}

				// This is a cross-context domain import. Check if it's allowed.
				allowedTargets := allowed[ctxName]
				if !slices.Contains(allowedTargets, targetCtx) {
					t.Errorf("%s imports %s (cross-context: %s -> %s) but this dependency is not declared in ContextMap",
						relPath, importPath, ctxName, targetCtx)
				}
			}

			return nil
		})

		if err != nil {
			t.Fatalf("walking %s: %v", ctxDir, err)
		}
	}
}

// TestContextMapCoversEventCatalogConsumers verifies that every event producer
// -> consumer relationship in the event catalog has a corresponding event-
// mechanism entry in the ContextMap. This catches the case where an event
// consumer is registered in the catalog but the context map was not updated.
func TestContextMapCoversEventCatalogConsumers(t *testing.T) {
	catalog := app.BuildEventCatalog()

	// Build a set of declared event flows from ContextMap.
	type flow struct{ from, to string }
	declaredFlows := make(map[flow]bool)
	for _, dep := range app.ContextMap {
		if dep.Mechanism == app.MechanismEvent {
			declaredFlows[flow{dep.From, dep.To}] = true
		}
	}

	// Check every catalog entry with non-empty consumers.
	for eventType, entry := range catalog.All() {
		for _, consumer := range entry.Consumers {
			f := flow{entry.Producer, consumer}
			if !declaredFlows[f] {
				t.Errorf("event %q: producer %q -> consumer %q is declared in the event catalog but not in ContextMap",
					eventType, entry.Producer, consumer)
			}
		}
	}
}

// TestNoUndeclaredCrossContextImports is the explicit bidirectional complement
// to TestContextMapMatchesRealImports. While TestContextMapMatchesRealImports
// checks "every cross-context domain import is declared in the map" (code ->
// map), this test verifies the reverse: every port/ACL entry in the context
// map that declares a synchronous dependency between two domain contexts has
// at least one corresponding Go file in either domain or adapter packages
// that materialises that dependency.
//
// A port/ACL entry without any corresponding import indicates a stale entry
// that should be removed, or an unfinished implementation.
func TestNoUndeclaredCrossContextImports(t *testing.T) {
	root := findRepoRoot(t)
	allowed := app.AllowedSyncImports()

	// For each declared sync dependency, scan the source context's domain
	// package AND the adapter packages for an import of the target context.
	for sourceCtx, targets := range allowed {
		for _, targetCtx := range targets {
			t.Run(sourceCtx+"->"+targetCtx, func(t *testing.T) {
				// Build search prefixes. Domain contexts live under
				// internal/domain/{ctx}, but some targets (infra, plugin)
				// are infrastructure packages at internal/{ctx} without a
				// domain sub-package. Search both locations so that
				// config-mutation ports (e.g., settings -> infra) are found
				// in app wiring without a direct domain-to-domain import.
				targetPrefixes := []string{
					modulePrefix + "/internal/domain/" + targetCtx,
					modulePrefix + "/internal/" + targetCtx,
				}

				// Search domain packages of the source context.
				sourceDomainDir := filepath.Join(root, "internal", "domain", sourceCtx)
				foundInDomain := hasImportOfAnyPrefix(t, sourceDomainDir, targetPrefixes)

				// Search adapter packages (adapters bridge contexts).
				adapterDir := filepath.Join(root, "internal", "adapter")
				foundInAdapter := hasImportOfAnyPrefix(t, adapterDir, targetPrefixes)

				// Search app wiring (wiring_*.go files may reference both contexts).
				appDir := filepath.Join(root, "internal", "app")
				foundInApp := hasImportOfAnyPrefix(t, appDir, targetPrefixes)

				if !foundInDomain && !foundInAdapter && !foundInApp {
					t.Errorf("ContextMap declares %s -> %s (port/acl) but no import of %s found in domain, adapter, or app packages",
						sourceCtx, targetCtx, strings.Join(targetPrefixes, " or "))
				}
			})
		}
	}
}

// hasImportOfPrefix walks a directory tree and returns true if any non-test Go
// file imports a path starting with the given prefix. Returns false if the
// directory does not exist.
func hasImportOfPrefix(t *testing.T, dir, prefix string) bool {
	t.Helper()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}

	found := false
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"gen"+string(filepath.Separator)) {
			return nil
		}

		fset := token.NewFileSet()
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return nil
		}

		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(importPath, prefix) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})

	return found
}

// hasImportOfAnyPrefix returns true if any non-test Go file in dir imports a
// path starting with any of the given prefixes. This extends hasImportOfPrefix
// for targets that may live at multiple locations (e.g., internal/domain/{ctx}
// or internal/{ctx} for infrastructure packages).
func hasImportOfAnyPrefix(t *testing.T, dir string, prefixes []string) bool {
	t.Helper()
	for _, prefix := range prefixes {
		if hasImportOfPrefix(t, dir, prefix) {
			return true
		}
	}
	return false
}

// TestAllowedSyncImportsConsistency verifies that AllowedSyncImports returns
// a non-nil map and that the entries match port/acl entries in ContextMap.
func TestAllowedSyncImportsConsistency(t *testing.T) {
	allowed := app.AllowedSyncImports()
	if allowed == nil {
		t.Fatal("AllowedSyncImports returned nil")
	}

	// Count expected port/acl entries from ContextMap.
	expected := make(map[string][]string)
	for _, dep := range app.ContextMap {
		if dep.Mechanism == app.MechanismPort || dep.Mechanism == app.MechanismACL {
			expected[dep.From] = append(expected[dep.From], dep.To)
		}
	}

	// Verify counts match.
	for from, targets := range expected {
		got := allowed[from]
		if len(got) != len(targets) {
			t.Errorf("AllowedSyncImports[%q]: expected %d targets, got %d", from, len(targets), len(got))
		}
	}

	for from := range allowed {
		if _, ok := expected[from]; !ok {
			t.Errorf("AllowedSyncImports has key %q but no port/acl entry exists in ContextMap", from)
		}
	}
}
