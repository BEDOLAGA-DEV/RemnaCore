package app

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWiringOrderSatisfiesConstraints(t *testing.T) {
	for _, c := range WiringConstraints {
		t.Run(c.Before+"_before_"+c.After, func(t *testing.T) {
			beforeIdx := slices.Index(ModuleOrder, c.Before)
			afterIdx := slices.Index(ModuleOrder, c.After)

			require.NotEqual(t, -1, beforeIdx,
				"module %q referenced in constraint but not found in ModuleOrder", c.Before)
			require.NotEqual(t, -1, afterIdx,
				"module %q referenced in constraint but not found in ModuleOrder", c.After)

			assert.Less(t, beforeIdx, afterIdx,
				"constraint violated: %s (index %d) must come before %s (index %d): %s",
				c.Before, beforeIdx, c.After, afterIdx, c.Reason)
		})
	}
}

func TestModuleOrderContainsNoDuplicates(t *testing.T) {
	seen := make(map[string]int, len(ModuleOrder))
	for i, name := range ModuleOrder {
		if prev, ok := seen[name]; ok {
			t.Errorf("duplicate module %q at indices %d and %d", name, prev, i)
		}
		seen[name] = i
	}
}

func TestModuleOrderMatchesKnownModules(t *testing.T) {
	// All module constants must appear in ModuleOrder.
	knownModules := []string{
		ModuleIdentity,
		ModuleBilling,
		ModuleMultisub,
		ModulePayment,
		ModuleReseller,
		ModulePlugin,
		ModuleNATS,
		ModuleInfra,
		ModuleHTTP,
		ModuleTelegram,
		ModuleTracing,
	}

	for _, m := range knownModules {
		t.Run(m, func(t *testing.T) {
			assert.True(t, slices.Contains(ModuleOrder, m),
				"module constant %q is not present in ModuleOrder", m)
		})
	}
}

func TestWiringConstraintsReferenceValidModules(t *testing.T) {
	for _, c := range WiringConstraints {
		t.Run(c.Before+"_"+c.After, func(t *testing.T) {
			assert.True(t, slices.Contains(ModuleOrder, c.Before),
				"constraint references unknown Before module %q", c.Before)
			assert.True(t, slices.Contains(ModuleOrder, c.After),
				"constraint references unknown After module %q", c.After)
			assert.NotEmpty(t, c.Reason,
				"constraint %s -> %s must have a non-empty Reason", c.Before, c.After)
		})
	}
}
