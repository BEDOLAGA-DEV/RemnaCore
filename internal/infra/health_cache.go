// Package infra provides in-process infrastructure services: node health
// monitoring, smart routing, speed testing, and subscription proxying.
package infra

import (
	"sync"
	"time"
)

// NodeHealth represents the cached health state of a single Remnawave node.
type NodeHealth struct {
	NodeID      string
	Name        string
	IsOnline    bool
	CountryCode string
	TrafficUsed int64
	UpdatedAt   time.Time
}

// NodeHealthCache is a thread-safe in-memory cache of node health data shared
// between the HealthMonitor (writer) and the SmartRouter (reader).
type NodeHealthCache struct {
	mu    sync.RWMutex
	nodes map[string]NodeHealth
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
