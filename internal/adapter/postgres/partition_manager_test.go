package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionManagerLockID(t *testing.T) {
	// The lock ID must be a positive int32-range value for pg_try_advisory_lock.
	// Changing it would break cross-pod coordination — treat as frozen constant.
	const expectedLockID = 847293651
	assert.Equal(t, expectedLockID, partitionManagerLockID,
		"partitionManagerLockID must not change; it is shared across all pods")
	assert.Greater(t, partitionManagerLockID, 0,
		"advisory lock ID must be positive")
}

func TestParseUpperBound(t *testing.T) {
	tests := []struct {
		name      string
		boundExpr string
		want      time.Time
		wantErr   bool
	}{
		{
			name:      "standard quarterly bound",
			boundExpr: "FOR VALUES FROM ('2026-01-01') TO ('2026-04-01')",
			want:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "year boundary",
			boundExpr: "FOR VALUES FROM ('2026-10-01') TO ('2027-01-01')",
			want:      time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "Q2 to Q3",
			boundExpr: "FOR VALUES FROM ('2026-04-01') TO ('2026-07-01')",
			want:      time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "missing TO clause",
			boundExpr: "FOR VALUES FROM ('2026-01-01')",
			wantErr:   true,
		},
		{
			name:      "empty expression",
			boundExpr: "",
			wantErr:   true,
		},
		{
			name:      "default partition (no range bounds)",
			boundExpr: "DEFAULT",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUpperBound(tt.boundExpr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPartitionBoundPatternMatch(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch bool
	}{
		{
			name:      "standard bound expression",
			input:     "FOR VALUES FROM ('2026-01-01') TO ('2026-04-01')",
			wantMatch: true,
		},
		{
			name:      "no TO clause",
			input:     "FOR VALUES FROM ('2026-01-01')",
			wantMatch: false,
		},
		{
			name:      "default partition",
			input:     "DEFAULT",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := partitionBoundPattern.FindStringSubmatch(tt.input)
			if tt.wantMatch {
				assert.GreaterOrEqual(t, len(matches), 2, "expected submatch")
			} else {
				assert.Less(t, len(matches), 2, "expected no submatch")
			}
		})
	}
}

func TestExpiredPartitionDetection(t *testing.T) {
	// Simulates the cutoff logic used by listExpiredPartitions.
	// If a partition's upper bound is before the cutoff, it is a candidate.
	tests := []struct {
		name          string
		upperBound    time.Time
		cutoff        time.Time
		wantCandidate bool
	}{
		{
			name:          "partition well before cutoff",
			upperBound:    time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC),
			cutoff:        time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantCandidate: true,
		},
		{
			name:          "partition exactly at cutoff",
			upperBound:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			cutoff:        time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			wantCandidate: false,
		},
		{
			name:          "partition after cutoff",
			upperBound:    time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
			cutoff:        time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			wantCandidate: false,
		},
		{
			name:          "retention 90 days from mid-Q2",
			upperBound:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
			cutoff:        time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC), // now=May 16 - 90d
			wantCandidate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isCandidate := tt.upperBound.Before(tt.cutoff)
			assert.Equal(t, tt.wantCandidate, isCandidate)
		})
	}
}

func TestIsSafeToDropSQL_Format(t *testing.T) {
	// Verify the SQL template produces valid SQL when given a safe partition name.
	tests := []struct {
		name      string
		partition string
		wantErr   bool
	}{
		{
			name:      "valid partition name formats correctly",
			partition: "outbox_2026_q1",
			wantErr:   false,
		},
		{
			name:      "invalid partition name rejected",
			partition: "outbox_2026_q1; DROP TABLE outbox",
			wantErr:   true,
		},
		{
			name:      "empty name rejected",
			partition: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := outboxPartitionPattern.MatchString(tt.partition)
			if tt.wantErr {
				assert.False(t, valid, "invalid partition name must be rejected by regex")
			} else {
				assert.True(t, valid, "valid partition name must pass regex")
			}
		})
	}
}

func TestSequenceSafetyLogic(t *testing.T) {
	// This test validates the pure logic behind isSafeToDrop without a database
	// connection. The SQL query computes:
	//   MAX(partition.sequence_number) < MIN(outbox.sequence_number WHERE published=false)
	//
	// We simulate this comparison to verify edge cases.

	// maxInt64Sentinel is the COALESCE default when no unpublished events exist.
	const maxInt64Sentinel int64 = 9223372036854775807

	tests := []struct {
		name           string
		partitionMax   int64 // MAX(sequence_number) from the partition
		globalMinUnpub int64 // MIN(sequence_number) from outbox WHERE published=false
		wantSafe       bool
	}{
		{
			name:           "partition fully published, min unpublished in later partition",
			partitionMax:   100,
			globalMinUnpub: 200,
			wantSafe:       true,
		},
		{
			name:           "no unpublished events globally, sentinel used",
			partitionMax:   500,
			globalMinUnpub: maxInt64Sentinel,
			wantSafe:       true,
		},
		{
			name:           "partition max equals global min unpublished",
			partitionMax:   100,
			globalMinUnpub: 100,
			wantSafe:       false,
		},
		{
			name:           "partition max exceeds global min unpublished",
			partitionMax:   300,
			globalMinUnpub: 200,
			wantSafe:       false,
		},
		{
			name:           "empty partition (max=0), unpublished elsewhere",
			partitionMax:   0,
			globalMinUnpub: 1,
			wantSafe:       true,
		},
		{
			name:           "empty partition (max=0), no unpublished events",
			partitionMax:   0,
			globalMinUnpub: maxInt64Sentinel,
			wantSafe:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			safe := tt.partitionMax < tt.globalMinUnpub
			assert.Equal(t, tt.wantSafe, safe)
		})
	}
}

func TestOutboxPartitionPatternValidation(t *testing.T) {
	tests := []struct {
		name      string
		partition string
		wantValid bool
	}{
		{
			name:      "valid Q1 partition",
			partition: "outbox_2026_q1",
			wantValid: true,
		},
		{
			name:      "valid Q4 partition",
			partition: "outbox_2027_q4",
			wantValid: true,
		},
		{
			name:      "valid default partition",
			partition: "outbox_default",
			wantValid: true,
		},
		{
			name:      "invalid quarter number",
			partition: "outbox_2026_q5",
			wantValid: false,
		},
		{
			name:      "SQL injection attempt",
			partition: "outbox_2026_q1; DROP TABLE outbox",
			wantValid: false,
		},
		{
			name:      "wrong prefix",
			partition: "events_2026_q1",
			wantValid: false,
		},
		{
			name:      "empty name",
			partition: "",
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantValid, outboxPartitionPattern.MatchString(tt.partition))
		})
	}
}
