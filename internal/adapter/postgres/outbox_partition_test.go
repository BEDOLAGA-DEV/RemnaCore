package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestQuarterOf(t *testing.T) {
	tests := []struct {
		name        string
		from        time.Time
		offset      int
		wantYear    int
		wantQuarter int
	}{
		{
			name:        "Q1 offset 0",
			from:        time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			offset:      0,
			wantYear:    2026,
			wantQuarter: 1,
		},
		{
			name:        "Q1 offset 1",
			from:        time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			offset:      1,
			wantYear:    2026,
			wantQuarter: 2,
		},
		{
			name:        "Q4 offset 0",
			from:        time.Date(2026, time.December, 31, 0, 0, 0, 0, time.UTC),
			offset:      0,
			wantYear:    2026,
			wantQuarter: 4,
		},
		{
			name:        "Q4 offset 1 rolls to next year",
			from:        time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC),
			offset:      1,
			wantYear:    2027,
			wantQuarter: 1,
		},
		{
			name:        "Q1 offset 4 rolls to next year Q1",
			from:        time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
			offset:      4,
			wantYear:    2027,
			wantQuarter: 1,
		},
		{
			name:        "Q2 offset 7 covers two years",
			from:        time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
			offset:      7,
			wantYear:    2028,
			wantQuarter: 1,
		},
		{
			name:        "Q3 start boundary",
			from:        time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
			offset:      0,
			wantYear:    2026,
			wantQuarter: 3,
		},
		{
			name:        "March is Q1",
			from:        time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
			offset:      0,
			wantYear:    2026,
			wantQuarter: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotYear, gotQuarter := quarterOf(tt.from, tt.offset)
			assert.Equal(t, tt.wantYear, gotYear, "year mismatch")
			assert.Equal(t, tt.wantQuarter, gotQuarter, "quarter mismatch")
		})
	}
}

func TestQuarterStart(t *testing.T) {
	tests := []struct {
		name    string
		year    int
		quarter int
		want    time.Time
	}{
		{
			name:    "Q1",
			year:    2026,
			quarter: 1,
			want:    time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "Q2",
			year:    2026,
			quarter: 2,
			want:    time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "Q3",
			year:    2026,
			quarter: 3,
			want:    time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "Q4",
			year:    2026,
			quarter: 4,
			want:    time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quarterStart(tt.year, tt.quarter)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTimeRange(t *testing.T) {
	t1 := time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, time.June, 30, 23, 59, 59, 0, time.UTC)
	t4 := time.Date(2026, time.March, 15, 10, 0, 0, 0, time.UTC) // duplicate of t1

	tests := []struct {
		name    string
		times   []time.Time
		wantMin time.Time
		wantMax time.Time
	}{
		{
			name:    "single element",
			times:   []time.Time{t1},
			wantMin: t1,
			wantMax: t1,
		},
		{
			name:    "ascending order",
			times:   []time.Time{t2, t1, t3},
			wantMin: t2,
			wantMax: t3,
		},
		{
			name:    "descending order",
			times:   []time.Time{t3, t1, t2},
			wantMin: t2,
			wantMax: t3,
		},
		{
			name:    "all identical",
			times:   []time.Time{t1, t4},
			wantMin: t1,
			wantMax: t1,
		},
		{
			name:    "min at end",
			times:   []time.Time{t3, t1, t2},
			wantMin: t2,
			wantMax: t3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := timeRange(tt.times)
			assert.Equal(t, tt.wantMin, gotMin, "min mismatch")
			assert.Equal(t, tt.wantMax, gotMax, "max mismatch")
		})
	}
}

func TestNextQuarterStart(t *testing.T) {
	tests := []struct {
		name    string
		year    int
		quarter int
		want    time.Time
	}{
		{
			name:    "Q1 to Q2",
			year:    2026,
			quarter: 1,
			want:    time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "Q4 rolls to next year Q1",
			year:    2026,
			quarter: 4,
			want:    time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextQuarterStart(tt.year, tt.quarter)
			assert.Equal(t, tt.want, got)
		})
	}
}
