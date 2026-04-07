package health

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeHealthCache_Update_ReplacesAll(t *testing.T) {
	cache := NewNodeHealthCache()
	cache.Update([]NodeHealth{
		{NodeID: "a", Name: "US-Node-1", IsOnline: true},
		{NodeID: "b", Name: "DE-Node-1", IsOnline: false},
	})

	all := cache.GetAll()
	assert.Len(t, all, 2)

	// Update replaces everything.
	cache.Update([]NodeHealth{
		{NodeID: "c", Name: "JP-Node-1", IsOnline: true},
	})
	all = cache.GetAll()
	assert.Len(t, all, 1)
	assert.Equal(t, "c", all[0].NodeID)
}

func TestNodeHealthCache_GetHealthy(t *testing.T) {
	cache := NewNodeHealthCache()
	cache.Update([]NodeHealth{
		{NodeID: "a", IsOnline: true},
		{NodeID: "b", IsOnline: false},
		{NodeID: "c", IsOnline: true},
	})

	healthy := cache.GetHealthy()
	assert.Len(t, healthy, 2)

	ids := map[string]bool{}
	for _, h := range healthy {
		ids[h.NodeID] = true
	}
	assert.True(t, ids["a"])
	assert.True(t, ids["c"])
}

func TestNodeHealthCache_Get(t *testing.T) {
	cache := NewNodeHealthCache()
	cache.Update([]NodeHealth{
		{NodeID: "x", Name: "TestNode"},
	})

	got, ok := cache.Get("x")
	require.True(t, ok)
	assert.Equal(t, "TestNode", got.Name)

	_, ok = cache.Get("missing")
	assert.False(t, ok)
}

func TestNodeHealthCache_IsFresh_NeverUpdated(t *testing.T) {
	cache := NewNodeHealthCache()
	assert.False(t, cache.IsFresh(5*time.Minute),
		"cache that was never updated should not be fresh")
}

func TestNodeHealthCache_IsFresh_RecentUpdate(t *testing.T) {
	cache := NewNodeHealthCache()
	cache.Update([]NodeHealth{
		{NodeID: "a", IsOnline: true},
	})

	assert.True(t, cache.IsFresh(5*time.Minute),
		"cache updated just now should be fresh within 5 minutes")
}

func TestNodeHealthCache_StaleDuration_NeverUpdated(t *testing.T) {
	cache := NewNodeHealthCache()
	assert.Equal(t, time.Duration(0), cache.StaleDuration(),
		"stale duration should be zero when never updated")
}

func TestNodeHealthCache_StaleDuration_AfterUpdate(t *testing.T) {
	cache := NewNodeHealthCache()
	cache.Update([]NodeHealth{
		{NodeID: "a", IsOnline: true},
	})

	stale := cache.StaleDuration()
	assert.Less(t, stale, 1*time.Second,
		"stale duration should be very small immediately after update")
}

func TestNodeHealthCache_UpdateSetsLastUpdated(t *testing.T) {
	cache := NewNodeHealthCache()

	// Before any update, IsFresh must return false.
	assert.False(t, cache.IsFresh(time.Hour))

	before := time.Now()
	cache.Update([]NodeHealth{
		{NodeID: "a", IsOnline: true},
	})

	// After update, IsFresh with a very large maxAge should return true.
	assert.True(t, cache.IsFresh(time.Hour))

	// StaleDuration should be small and reflect time since the update.
	stale := cache.StaleDuration()
	assert.GreaterOrEqual(t, stale, time.Duration(0))
	assert.Less(t, stale, time.Since(before)+1*time.Millisecond)
}

func TestNodeHealthCache_ConcurrentAccess(t *testing.T) {
	cache := NewNodeHealthCache()
	now := time.Now()

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			cache.Update([]NodeHealth{
				{NodeID: "a", IsOnline: true, UpdatedAt: now},
			})
		})
	}
	for range 100 {
		wg.Go(func() {
			_ = cache.GetAll()
			_ = cache.GetHealthy()
			cache.Get("a")
			_ = cache.IsFresh(5 * time.Minute)
			_ = cache.StaleDuration()
		})
	}
	wg.Wait()

	// No race condition panic means success.
	all := cache.GetAll()
	assert.NotEmpty(t, all)
}
