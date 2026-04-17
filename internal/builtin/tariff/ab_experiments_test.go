package tariff

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssignVariant_Deterministic(t *testing.T) {
	variants := []ABVariant{
		{Key: "control", PriceCents: 999, TrafficPct: 5000},
		{Key: "v1_high", PriceCents: 1299, TrafficPct: 5000},
	}

	// Same user+experiment should always get the same variant.
	v1 := assignVariant("user-123", "exp-abc", variants)
	v2 := assignVariant("user-123", "exp-abc", variants)
	v3 := assignVariant("user-123", "exp-abc", variants)

	assert.Equal(t, v1.Key, v2.Key)
	assert.Equal(t, v2.Key, v3.Key)
}

func TestAssignVariant_DifferentUsersDifferentResults(t *testing.T) {
	variants := []ABVariant{
		{Key: "control", PriceCents: 999, TrafficPct: 5000},
		{Key: "v1_high", PriceCents: 1299, TrafficPct: 5000},
	}

	// Different users should (statistically) get different variants.
	results := make(map[string]int)
	for i := 0; i < 1000; i++ {
		v := assignVariant(fmt.Sprintf("user-%d", i), "exp-test", variants)
		results[v.Key]++
	}

	// Both variants should have assignments.
	assert.Greater(t, results["control"], 0)
	assert.Greater(t, results["v1_high"], 0)
}

func TestAssignVariant_TrafficDistribution(t *testing.T) {
	variants := []ABVariant{
		{Key: "control", PriceCents: 999, TrafficPct: 5000},
		{Key: "v1_high", PriceCents: 1299, TrafficPct: 5000},
	}

	counts := make(map[string]int)
	n := 100000
	for i := 0; i < n; i++ {
		userID := fmt.Sprintf("user-%d", i)
		v := assignVariant(userID, "exp-distribution", variants)
		counts[v.Key]++
	}

	// With 50/50 split and 100k samples, each should be ~50k.
	// Allow 5% deviation.
	expected := float64(n) / 2
	tolerance := expected * 0.05

	assert.InDelta(t, expected, float64(counts["control"]), tolerance)
	assert.InDelta(t, expected, float64(counts["v1_high"]), tolerance)
}

func TestAssignVariant_UnevenSplit(t *testing.T) {
	variants := []ABVariant{
		{Key: "control", PriceCents: 999, TrafficPct: 9000}, // 90%
		{Key: "treatment", PriceCents: 1299, TrafficPct: 1000}, // 10%
	}

	counts := make(map[string]int)
	n := 100000
	for i := 0; i < n; i++ {
		userID := fmt.Sprintf("user-%d", i)
		v := assignVariant(userID, "exp-uneven", variants)
		counts[v.Key]++
	}

	controlPct := float64(counts["control"]) / float64(n)
	treatmentPct := float64(counts["treatment"]) / float64(n)

	assert.InDelta(t, 0.90, controlPct, 0.02)
	assert.InDelta(t, 0.10, treatmentPct, 0.02)
}

func TestAssignVariant_ThreeVariants(t *testing.T) {
	variants := []ABVariant{
		{Key: "a", PriceCents: 999, TrafficPct: 3334},
		{Key: "b", PriceCents: 1099, TrafficPct: 3333},
		{Key: "c", PriceCents: 1199, TrafficPct: 3333},
	}

	counts := make(map[string]int)
	n := 100000
	for i := 0; i < n; i++ {
		userID := fmt.Sprintf("user-%d", i)
		v := assignVariant(userID, "exp-three", variants)
		counts[v.Key]++
	}

	expected := float64(n) / 3
	tolerance := expected * 0.05

	assert.InDelta(t, expected, float64(counts["a"]), tolerance)
	assert.InDelta(t, expected, float64(counts["b"]), tolerance)
	assert.InDelta(t, expected, float64(counts["c"]), tolerance)
}

func TestAssignVariant_DifferentExperimentsDifferentAssignment(t *testing.T) {
	variants := []ABVariant{
		{Key: "control", PriceCents: 999, TrafficPct: 5000},
		{Key: "treatment", PriceCents: 1299, TrafficPct: 5000},
	}

	// Same user, different experiments — may get different assignments.
	// We just verify they're both valid.
	v1 := assignVariant("user-fixed", "exp-A", variants)
	v2 := assignVariant("user-fixed", "exp-B", variants)

	assert.Contains(t, []string{"control", "treatment"}, v1.Key)
	assert.Contains(t, []string{"control", "treatment"}, v2.Key)
}

// --- Validation tests ---

func TestValidateABExperiment_Valid(t *testing.T) {
	input := &ABExperimentInput{
		Name:     "Test Experiment",
		TariffID: "t_premium",
		Variants: []ABVariant{
			{Key: "control", PriceCents: 999, TrafficPct: 5000},
			{Key: "treatment", PriceCents: 1299, TrafficPct: 5000},
		},
	}
	assert.NoError(t, validateABExperimentInput(input))
	assert.Equal(t, "draft", input.Status)
	assert.Equal(t, "hash_user_id", input.AssignmentStrategy)
}

func TestValidateABExperiment_MissingName(t *testing.T) {
	input := &ABExperimentInput{
		TariffID: "t_premium",
		Variants: []ABVariant{
			{Key: "a", PriceCents: 999, TrafficPct: 5000},
			{Key: "b", PriceCents: 1299, TrafficPct: 5000},
		},
	}
	assert.ErrorContains(t, validateABExperimentInput(input), "name is required")
}

func TestValidateABExperiment_MissingTariffID(t *testing.T) {
	input := &ABExperimentInput{
		Name: "Test",
		Variants: []ABVariant{
			{Key: "a", PriceCents: 999, TrafficPct: 5000},
			{Key: "b", PriceCents: 1299, TrafficPct: 5000},
		},
	}
	assert.ErrorContains(t, validateABExperimentInput(input), "tariff_id is required")
}

func TestValidateABExperiment_SingleVariant(t *testing.T) {
	input := &ABExperimentInput{
		Name:     "Test",
		TariffID: "t_premium",
		Variants: []ABVariant{
			{Key: "only", PriceCents: 999, TrafficPct: 10000},
		},
	}
	assert.ErrorContains(t, validateABExperimentInput(input), "at least 2 variants required")
}

func TestValidateABExperiment_TrafficDoesntSum(t *testing.T) {
	input := &ABExperimentInput{
		Name:     "Test",
		TariffID: "t_premium",
		Variants: []ABVariant{
			{Key: "a", PriceCents: 999, TrafficPct: 5000},
			{Key: "b", PriceCents: 1299, TrafficPct: 3000},
		},
	}
	assert.ErrorContains(t, validateABExperimentInput(input), "must sum to 10000")
}

func TestValidateABExperiment_DuplicateKeys(t *testing.T) {
	input := &ABExperimentInput{
		Name:     "Test",
		TariffID: "t_premium",
		Variants: []ABVariant{
			{Key: "same", PriceCents: 999, TrafficPct: 5000},
			{Key: "same", PriceCents: 1299, TrafficPct: 5000},
		},
	}
	assert.ErrorContains(t, validateABExperimentInput(input), "duplicate variant key")
}

func TestValidateABExperiment_InvalidStrategy(t *testing.T) {
	input := &ABExperimentInput{
		Name:               "Test",
		TariffID:           "t_premium",
		AssignmentStrategy: "round_robin",
		Variants: []ABVariant{
			{Key: "a", PriceCents: 999, TrafficPct: 5000},
			{Key: "b", PriceCents: 1299, TrafficPct: 5000},
		},
	}
	assert.ErrorContains(t, validateABExperimentInput(input), "unsupported assignment_strategy")
}

// Ensure chi-square goodness of fit is reasonable.
func TestAssignVariant_ChiSquare(t *testing.T) {
	variants := []ABVariant{
		{Key: "a", PriceCents: 999, TrafficPct: 5000},
		{Key: "b", PriceCents: 1299, TrafficPct: 5000},
	}

	n := 100000
	counts := make(map[string]float64)
	for i := 0; i < n; i++ {
		userID := "u" + string(rune(i/256)) + string(rune(i%256))
		v := assignVariant(userID, "exp-chi", variants)
		counts[v.Key]++
	}

	// Chi-square test: sum((observed - expected)^2 / expected)
	expected := float64(n) / 2
	chiSquare := 0.0
	for _, count := range counts {
		diff := count - expected
		chiSquare += (diff * diff) / expected
	}

	// Critical value for 1 df at p=0.05 is 3.84
	// Critical value for 1 df at p=0.01 is 6.63
	assert.Less(t, chiSquare, 6.63, "chi-square value %f exceeds critical threshold", chiSquare)
}
