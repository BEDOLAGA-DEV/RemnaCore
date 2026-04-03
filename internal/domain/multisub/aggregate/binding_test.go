package aggregate_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

func TestNewBinding(t *testing.T) {
	b := aggregate.NewBinding("sub-1", "user-abc12345xyz", "base", 0, 100_000_000_000)

	require.NotEmpty(t, b.ID)
	assert.Equal(t, "sub-1", b.SubscriptionID)
	assert.Equal(t, "user-abc12345xyz", b.PlatformUserID)
	assert.Equal(t, aggregate.BindingPending, b.Status)
	assert.Equal(t, aggregate.PurposeBase, b.Purpose)
	assert.Equal(t, int64(100_000_000_000), b.TrafficLimitBytes)
	assert.Contains(t, b.RemnawaveUsername, "p_user-abc")
	assert.Contains(t, b.RemnawaveUsername, "base_0")
	assert.False(t, b.CreatedAt.IsZero())
	assert.False(t, b.UpdatedAt.IsZero())
}

func TestMarkProvisioned(t *testing.T) {
	b := aggregate.NewBinding("sub-1", "user-abc12345xyz", "base", 0, 100_000_000_000)
	before := b.UpdatedAt

	b.MarkProvisioned("rw-uuid-123", "rw-short-456")

	assert.Equal(t, aggregate.BindingActive, b.Status)
	assert.Equal(t, "rw-uuid-123", b.RemnawaveUUID)
	assert.Equal(t, "rw-short-456", b.RemnawaveShortUUID)
	assert.True(t, b.UpdatedAt.After(before) || b.UpdatedAt.Equal(before))
}

func TestMarkFailed(t *testing.T) {
	b := aggregate.NewBinding("sub-1", "user-abc12345xyz", "base", 0, 100_000_000_000)

	b.MarkFailed("connection refused")

	assert.Equal(t, aggregate.BindingFailed, b.Status)
	assert.Equal(t, "connection refused", b.FailReason)
}

func TestDisable(t *testing.T) {
	b := aggregate.NewBinding("sub-1", "user-abc12345xyz", "base", 0, 100_000_000_000)
	b.MarkProvisioned("rw-uuid-123", "rw-short-456")

	b.Disable()

	assert.Equal(t, aggregate.BindingDisabled, b.Status)
}

func TestEnable(t *testing.T) {
	b := aggregate.NewBinding("sub-1", "user-abc12345xyz", "base", 0, 100_000_000_000)
	b.MarkProvisioned("rw-uuid-123", "rw-short-456")
	b.Disable()

	b.Enable()

	assert.Equal(t, aggregate.BindingActive, b.Status)
}

func TestDeprovision(t *testing.T) {
	b := aggregate.NewBinding("sub-1", "user-abc12345xyz", "base", 0, 100_000_000_000)
	b.MarkProvisioned("rw-uuid-123", "rw-short-456")

	b.Deprovision()

	assert.Equal(t, aggregate.BindingDeprovisioned, b.Status)
}
