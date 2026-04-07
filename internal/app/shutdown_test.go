package app

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownPhasesAreSequential(t *testing.T) {
	for i, phase := range ShutdownPhases {
		t.Run(phase.Name, func(t *testing.T) {
			assert.Equal(t, i+1, phase.Order,
				"phase %q at index %d has order %d, expected %d",
				phase.Name, i, phase.Order, i+1)
			assert.NotEmpty(t, phase.Reason,
				"phase %q must have a non-empty Reason", phase.Name)
		})
	}
}

func TestShutdownPhasesContainNoDuplicates(t *testing.T) {
	seen := make(map[string]int, len(ShutdownPhases))
	for i, phase := range ShutdownPhases {
		if prev, ok := seen[phase.Name]; ok {
			t.Errorf("duplicate shutdown phase %q at indices %d and %d", phase.Name, prev, i)
		}
		seen[phase.Name] = i
	}
}

func TestShutdownConstraintsAreSatisfied(t *testing.T) {
	phaseIndex := make(map[string]int, len(ShutdownPhases))
	for i, phase := range ShutdownPhases {
		phaseIndex[phase.Name] = i
	}

	for _, c := range ShutdownConstraints {
		t.Run(c.Before+"_before_"+c.After, func(t *testing.T) {
			beforeIdx, beforeOK := phaseIndex[c.Before]
			afterIdx, afterOK := phaseIndex[c.After]

			require.True(t, beforeOK,
				"shutdown constraint references unknown phase %q", c.Before)
			require.True(t, afterOK,
				"shutdown constraint references unknown phase %q", c.After)

			assert.Less(t, beforeIdx, afterIdx,
				"constraint violated: %s (phase %d) must complete before %s (phase %d): %s",
				c.Before, beforeIdx+1, c.After, afterIdx+1, c.Reason)
		})
	}
}

func TestShutdownConstraintsReferenceValidPhases(t *testing.T) {
	phaseNames := make([]string, 0, len(ShutdownPhases))
	for _, p := range ShutdownPhases {
		phaseNames = append(phaseNames, p.Name)
	}

	for _, c := range ShutdownConstraints {
		t.Run(c.Before+"_"+c.After, func(t *testing.T) {
			assert.True(t, slices.Contains(phaseNames, c.Before),
				"constraint references unknown Before phase %q", c.Before)
			assert.True(t, slices.Contains(phaseNames, c.After),
				"constraint references unknown After phase %q", c.After)
			assert.NotEmpty(t, c.Reason,
				"constraint %s -> %s must have a non-empty Reason", c.Before, c.After)
		})
	}
}
