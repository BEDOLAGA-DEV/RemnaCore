package vo

import (
	"testing"
	"time"
)

func FuzzBillingPeriodDaysRemaining(f *testing.F) {
	now := time.Now()
	f.Add(now.Unix(), now.Add(30*24*time.Hour).Unix(), now.Unix())
	f.Add(now.Unix(), now.Unix(), now.Unix())
	f.Add(int64(0), int64(86400), int64(43200))

	f.Fuzz(func(t *testing.T, startUnix, endUnix, nowUnix int64) {
		start := time.Unix(startUnix, 0)
		end := time.Unix(endUnix, 0)
		now := time.Unix(nowUnix, 0)

		bp := BillingPeriod{Start: start, End: end}
		days := bp.DaysRemaining(now)

		if days < 0 {
			t.Errorf("DaysRemaining returned negative: %d", days)
		}
	})
}
