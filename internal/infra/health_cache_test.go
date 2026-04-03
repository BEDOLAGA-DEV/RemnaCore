package infra

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

func TestNodeHealthCache_ConcurrentAccess(t *testing.T) {
	cache := NewNodeHealthCache()
	now := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache.Update([]NodeHealth{
				{NodeID: "a", IsOnline: true, UpdatedAt: now},
			})
		}()
	}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cache.GetAll()
			_ = cache.GetHealthy()
			cache.Get("a")
		}()
	}
	wg.Wait()

	// No race condition panic means success.
	all := cache.GetAll()
	assert.NotEmpty(t, all)
}
