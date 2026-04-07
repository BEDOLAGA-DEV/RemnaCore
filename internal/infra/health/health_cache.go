// Package health provides in-process node health monitoring and caching.
// The HealthMonitor periodically polls Remnawave for node status and updates
// the shared in-memory NodeHealthCache, which is read by the routing package.
package health

import (
	"sync"
	"time"
)

// NodeHealth represents the cached health state of a single Remnawave node.
type NodeHealth struct {
	NodeID        string
	Name          string
	IsOnline      bool
	CountryCode   string
	TrafficUsed   int64
	Latitude      float64
	Longitude     float64
	LastLatencyMs float64
	UpdatedAt     time.Time
}

// NodeHealthCache is a thread-safe in-memory cache of node health data shared
// between the HealthMonitor (writer) and the SmartRouter (reader).
type NodeHealthCache struct {
	mu          sync.RWMutex
	nodes       map[string]NodeHealth
	lastUpdated time.Time
}

// NewNodeHealthCache returns an initialised, empty cache.
func NewNodeHealthCache() *NodeHealthCache {
	return &NodeHealthCache{
		nodes: make(map[string]NodeHealth),
	}
}

// Update replaces the cache contents with the supplied health entries.
func (c *NodeHealthCache) Update(health []NodeHealth) {
	c.mu.Lock()
	defer c.mu.Unlock()

	updated := make(map[string]NodeHealth, len(health))
	for _, h := range health {
		updated[h.NodeID] = h
	}
	c.nodes = updated
	c.lastUpdated = time.Now()
}

// GetAll returns a snapshot of every cached node.
func (c *NodeHealthCache) GetAll() []NodeHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]NodeHealth, 0, len(c.nodes))
	for _, h := range c.nodes {
		out = append(out, h)
	}
	return out
}

// GetHealthy returns a snapshot of nodes that are currently online.
func (c *NodeHealthCache) GetHealthy() []NodeHealth {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]NodeHealth, 0, len(c.nodes))
	for _, h := range c.nodes {
		if h.IsOnline {
			out = append(out, h)
		}
	}
	return out
}

// Get returns a single node's health data and whether it was found.
func (c *NodeHealthCache) Get(nodeID string) (NodeHealth, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	h, ok := c.nodes[nodeID]
	return h, ok
}

// IsFresh reports whether the cache has been updated within the given maxAge
// window. Returns false if the cache has never been updated.
func (c *NodeHealthCache) IsFresh(maxAge time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lastUpdated.IsZero() {
		return false
	}
	return time.Since(c.lastUpdated) < maxAge
}

// StaleDuration returns how long ago the cache was last updated. Returns zero
// if the cache has never been updated.
func (c *NodeHealthCache) StaleDuration() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.lastUpdated.IsZero() {
		return 0
	}
	return time.Since(c.lastUpdated)
}
