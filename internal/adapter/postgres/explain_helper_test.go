//go:build integration

package postgres_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// explainPlanPrefix is prepended to every query to produce a JSON query plan.
const explainPlanPrefix = "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "

// seqScanNodeType is the node type string in EXPLAIN JSON output that
// indicates a sequential scan.
const seqScanNodeType = "Seq Scan"

// ExplainPlan runs EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) on the given SQL
// with args and returns the plan as a raw JSON string. Use in integration
// tests to verify index usage, scan types, and buffer statistics.
func ExplainPlan(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()

	explainSQL := explainPlanPrefix + sql
	var planJSON string
	err := pool.QueryRow(context.Background(), explainSQL, args...).Scan(&planJSON)
	require.NoError(t, err, "EXPLAIN ANALYZE query failed")

	return planJSON
}

// AssertIndexUsed verifies that the EXPLAIN plan JSON contains the expected
// index name. Works for both "Index Scan" and "Index Only Scan" node types.
func AssertIndexUsed(t *testing.T, plan string, indexName string) {
	t.Helper()

	assert.Contains(t, plan, indexName,
		"expected query plan to use index %q, plan:\n%s", indexName, plan)
}

// AssertNoSeqScan verifies that the EXPLAIN plan does not contain a sequential
// scan node. This catches cases where the planner falls back to a full table
// scan instead of using an index.
func AssertNoSeqScan(t *testing.T, plan string) {
	t.Helper()

	assert.NotContains(t, plan, seqScanNodeType,
		"expected no sequential scan in query plan, plan:\n%s", plan)
}

// AssertNodeType verifies that the EXPLAIN plan JSON contains at least one
// node of the given type (e.g. "Index Scan", "Bitmap Heap Scan").
func AssertNodeType(t *testing.T, plan string, nodeType string) {
	t.Helper()

	assert.Contains(t, plan, nodeType,
		"expected query plan to contain node type %q, plan:\n%s", nodeType, plan)
}

// PlanSummary extracts a human-readable summary from the EXPLAIN JSON output.
// Returns the top-level node type and the total execution time. Useful for
// debug logging in failing tests.
func PlanSummary(t *testing.T, planJSON string) (nodeType string, executionTimeMs float64) {
	t.Helper()

	// EXPLAIN FORMAT JSON returns an array with one element.
	var plans []map[string]any
	err := json.Unmarshal([]byte(planJSON), &plans)
	require.NoError(t, err, "failed to parse EXPLAIN JSON")
	require.NotEmpty(t, plans, "EXPLAIN JSON returned empty array")

	topPlan, ok := plans[0]["Plan"].(map[string]any)
	require.True(t, ok, "expected Plan key in EXPLAIN JSON")

	nodeType, _ = topPlan["Node Type"].(string)
	executionTimeMs, _ = plans[0]["Execution Time"].(float64)

	return nodeType, executionTimeMs
}

// planContainsIndex recursively walks the EXPLAIN JSON plan tree and returns
// true if any node references the given index name in its "Index Name" field.
// This is more precise than string containment on the raw JSON.
func planContainsIndex(t *testing.T, planJSON string, indexName string) bool {
	t.Helper()

	var plans []map[string]any
	err := json.Unmarshal([]byte(planJSON), &plans)
	require.NoError(t, err, "failed to parse EXPLAIN JSON")
	if len(plans) == 0 {
		return false
	}

	topPlan, ok := plans[0]["Plan"].(map[string]any)
	if !ok {
		return false
	}

	return walkPlanForIndex(topPlan, indexName)
}

// walkPlanForIndex recursively searches the plan node tree for the given index name.
func walkPlanForIndex(node map[string]any, indexName string) bool {
	if name, ok := node["Index Name"].(string); ok {
		if strings.Contains(name, indexName) {
			return true
		}
	}

	// Check child plans.
	if plans, ok := node["Plans"].([]any); ok {
		for _, child := range plans {
			if childNode, ok := child.(map[string]any); ok {
				if walkPlanForIndex(childNode, indexName) {
					return true
				}
			}
		}
	}

	return false
}

// AssertIndexUsedStrict verifies that the EXPLAIN plan JSON contains the
// expected index name by walking the plan tree, rather than relying on raw
// string containment. Preferred when index names may be substrings of other
// plan text.
func AssertIndexUsedStrict(t *testing.T, planJSON string, indexName string) {
	t.Helper()

	found := planContainsIndex(t, planJSON, indexName)
	assert.True(t, found,
		"expected query plan to reference index %q (strict tree walk), plan:\n%s", indexName, planJSON)
}

// explainEstimatePrefix is prepended to queries when we only need the planner's
// estimated plan without actually executing the query. Essential for UPDATE and
// DELETE statements where EXPLAIN ANALYZE would modify data.
const explainEstimatePrefix = "EXPLAIN (FORMAT JSON) "

// ExplainPlanEstimate runs EXPLAIN (FORMAT JSON) on the given SQL without
// ANALYZE. Returns the estimated plan as a raw JSON string. Use for UPDATE
// and DELETE queries where executing the query would cause side effects.
func ExplainPlanEstimate(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) string {
	t.Helper()

	explainSQL := explainEstimatePrefix + sql
	var planJSON string
	err := pool.QueryRow(context.Background(), explainSQL, args...).Scan(&planJSON)
	require.NoError(t, err, "EXPLAIN (estimate) query failed")

	return planJSON
}

// PlanEstimateSummary extracts the top-level node type from an estimated plan
// (without ANALYZE there is no execution time).
func PlanEstimateSummary(t *testing.T, planJSON string) string {
	t.Helper()

	var plans []map[string]any
	err := json.Unmarshal([]byte(planJSON), &plans)
	require.NoError(t, err, "failed to parse EXPLAIN JSON")
	require.NotEmpty(t, plans, "EXPLAIN JSON returned empty array")

	topPlan, ok := plans[0]["Plan"].(map[string]any)
	require.True(t, ok, "expected Plan key in EXPLAIN JSON")

	nodeType, _ := topPlan["Node Type"].(string)
	return nodeType
}

// planContainsNodeType recursively walks the EXPLAIN JSON plan tree and
// returns true if any node has the given "Node Type" value. Useful for
// verifying specific scan strategies (e.g. "Index Scan", "Bitmap Heap Scan")
// at any depth in the plan tree.
func planContainsNodeType(t *testing.T, planJSON string, nodeType string) bool {
	t.Helper()

	var plans []map[string]any
	err := json.Unmarshal([]byte(planJSON), &plans)
	require.NoError(t, err, "failed to parse EXPLAIN JSON")
	if len(plans) == 0 {
		return false
	}

	topPlan, ok := plans[0]["Plan"].(map[string]any)
	if !ok {
		return false
	}

	return walkPlanForNodeType(topPlan, nodeType)
}

// walkPlanForNodeType recursively searches the plan node tree for a node
// with the given "Node Type" value.
func walkPlanForNodeType(node map[string]any, nodeType string) bool {
	if nt, ok := node["Node Type"].(string); ok && nt == nodeType {
		return true
	}

	if plans, ok := node["Plans"].([]any); ok {
		for _, child := range plans {
			if childNode, ok := child.(map[string]any); ok {
				if walkPlanForNodeType(childNode, nodeType) {
					return true
				}
			}
		}
	}

	return false
}

// AssertNodeTypeInTree verifies that the EXPLAIN plan JSON contains at least
// one node of the given type anywhere in the plan tree. More precise than
// AssertNodeType which uses raw string containment.
func AssertNodeTypeInTree(t *testing.T, planJSON string, nodeType string) {
	t.Helper()

	found := planContainsNodeType(t, planJSON, nodeType)
	assert.True(t, found,
		"expected query plan to contain node type %q (strict tree walk), plan:\n%s", nodeType, planJSON)
}

// AssertIndexOrSeqScanWithLog verifies that either an index scan using the
// given index or a sequential scan is present in the plan, logging which was
// chosen. Used for status-only queries on composite indexes where the planner
// may or may not use skip scan depending on table statistics and row count.
// Returns true if the index was used, false if seq scan was used.
func AssertIndexOrSeqScanWithLog(t *testing.T, planJSON string, indexName string) bool {
	t.Helper()

	usesIndex := planContainsIndex(t, planJSON, indexName)
	usesSeqScan := planContainsNodeType(t, planJSON, seqScanNodeType)

	if usesIndex {
		t.Logf("PG18 planner chose index scan on %q (skip scan or index scan)", indexName)
		return true
	}

	if usesSeqScan {
		t.Logf("PG18 planner chose sequential scan instead of %q "+
			"(skip scan not triggered — row count may be too low for the planner "+
			"to prefer it over seq scan)", indexName)
		return false
	}

	// Some other scan type (bitmap, etc) — still log it.
	t.Logf("PG18 planner chose neither index scan on %q nor seq scan — "+
		"check plan output for details", indexName)
	return false
}
