package vo_test

import (
	"testing"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/billing/vo"
)

func TestBillingPeriod_DaysRemaining(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	period := vo.NewBillingPeriod(start, vo.IntervalMonth) // ends 2026-05-01

	tests := []struct {
		name string
		now  time.Time
		want int
	}{
		{
			name: "48 hours remaining returns 2",
			now:  period.End.Add(-48 * time.Hour),
			want: 2,
		},
		{
			name: "period already ended returns 0",
			now:  period.End.Add(24 * time.Hour),
			want: 0,
		},
		{
			name: "exact end returns 0",
			now:  period.End,
			want: 0,
		},
		{
			name: "full period remaining",
			now:  start,
			want: 30, // April has 30 days
		},
		{
			name: "partial day not counted",
			now:  period.End.Add(-36 * time.Hour),
			want: 1, // 36h / 24 = 1.5, truncated to 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := period.DaysRemaining(tt.now)
			if got != tt.want {
				t.Errorf("DaysRemaining(%v) = %d, want %d", tt.now, got, tt.want)
			}
		})
	}
}
