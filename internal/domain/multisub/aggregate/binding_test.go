package aggregate_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/BEDOLAGA-DEV/RemnaCore/internal/domain/multisub/aggregate"
)

func mustNewBinding(t *testing.T, subID, platformUserID string, purpose aggregate.BindingPurpose, index int, trafficLimit int64, now time.Time) *aggregate.RemnawaveBinding {
	t.Helper()
	b, err := aggregate.NewBinding(subID, platformUserID, purpose, index, trafficLimit, now)
	require.NoError(t, err)
	return b
}

func TestNewBinding(t *testing.T) {
	b, err := aggregate.NewBinding("sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())

	require.NoError(t, err)
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

func TestNewBinding_EmptySubscriptionID(t *testing.T) {
	b, err := aggregate.NewBinding("", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrEmptySubscriptionID)
	assert.Nil(t, b)
}

func TestNewBinding_EmptyPlatformUserID(t *testing.T) {
	b, err := aggregate.NewBinding("sub-1", "", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrEmptyPlatformUserID)
	assert.Nil(t, b)
}

func TestNewBinding_InvalidPurpose(t *testing.T) {
	b, err := aggregate.NewBinding("sub-1", "user-abc12345xyz", aggregate.BindingPurpose("invalid"), 0, 100_000_000_000, time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidPurpose)
	assert.Nil(t, b)
}

func TestMarkProvisioned(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	before := b.UpdatedAt

	err := b.MarkProvisioned("rw-uuid-123", "rw-short-456", time.Now())

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingActive, b.Status)
	assert.Equal(t, "rw-uuid-123", b.RemnawaveUUID)
	assert.Equal(t, "rw-short-456", b.RemnawaveShortUUID)
	assert.NotNil(t, b.SyncedAt)
	assert.True(t, b.UpdatedAt.After(before) || b.UpdatedAt.Equal(before))
}

func TestMarkFailed(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())

	err := b.MarkFailed("connection refused", time.Now())

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingFailed, b.Status)
	assert.Equal(t, "connection refused", b.FailReason)
}

func TestDisable(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	require.NoError(t, b.MarkProvisioned("rw-uuid-123", "rw-short-456", time.Now()))

	err := b.Disable(time.Now())

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingDisabled, b.Status)
}

func TestEnable(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	require.NoError(t, b.MarkProvisioned("rw-uuid-123", "rw-short-456", time.Now()))
	require.NoError(t, b.Disable(time.Now()))

	err := b.Enable(time.Now())

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingActive, b.Status)
}

func TestDeprovision(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	require.NoError(t, b.MarkProvisioned("rw-uuid-123", "rw-short-456", time.Now()))

	err := b.Deprovision(time.Now())

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingDeprovisioned, b.Status)
}

func TestMarkProvisioned_InvalidTransition(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	require.NoError(t, b.MarkProvisioned("rw-uuid-123", "rw-short-456", time.Now()))

	// active -> active (MarkProvisioned) is not allowed
	err := b.MarkProvisioned("rw-uuid-999", "rw-short-999", time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidBindingTransition)
}

func TestDisable_InvalidTransition_FromPending(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())

	err := b.Disable(time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidBindingTransition)
}

func TestEnable_InvalidTransition_FromPending(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())

	err := b.Enable(time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidBindingTransition)
}

func TestDeprovision_FromFailed(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	require.NoError(t, b.MarkFailed("some error", time.Now()))

	err := b.Deprovision(time.Now())

	require.NoError(t, err)
	assert.Equal(t, aggregate.BindingDeprovisioned, b.Status)
}

func TestDeprovisioned_IsTerminal(t *testing.T) {
	b := mustNewBinding(t, "sub-1", "user-abc12345xyz", aggregate.PurposeBase, 0, 100_000_000_000, time.Now())
	require.NoError(t, b.MarkProvisioned("rw-uuid-123", "rw-short-456", time.Now()))
	require.NoError(t, b.Deprovision(time.Now()))

	err := b.Enable(time.Now())

	require.Error(t, err)
	assert.ErrorIs(t, err, aggregate.ErrInvalidBindingTransition)
}
