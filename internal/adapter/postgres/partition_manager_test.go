package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
