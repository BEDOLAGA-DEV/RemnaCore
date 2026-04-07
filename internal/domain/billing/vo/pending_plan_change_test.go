package vo

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPendingPlanChange_IsZero(t *testing.T) {
	t.Run("zero value is zero", func(t *testing.T) {
		var pc PendingPlanChange
		assert.True(t, pc.IsZero())
	})

	t.Run("populated value is not zero", func(t *testing.T) {
		pc := PendingPlanChange{
			PlanID:         "plan-basic",
			OriginalPlanID: "plan-premium",
			RequestedAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		}
		assert.False(t, pc.IsZero())
	})

	t.Run("empty PlanID with other fields set is still zero", func(t *testing.T) {
		pc := PendingPlanChange{
			OriginalPlanID: "plan-premium",
			RequestedAt:    time.Now(),
		}
		assert.True(t, pc.IsZero())
	})
}
