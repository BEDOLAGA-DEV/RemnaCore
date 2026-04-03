package infra

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/BEDOLAGA-DEV/RemnaCore/internal/adapter/remnawave"
	"github.com/BEDOLAGA-DEV/RemnaCore/pkg/domainevent"
)

const (
	// DefaultHealthCheckInterval is the period between node health polls.
	DefaultHealthCheckInterval = 10 * time.Second

	// DefaultMaxConcurrentChecks limits fan-out goroutines during a health check.
	DefaultMaxConcurrentChecks = 50

	// EventNodeHealthChanged is published when a node transitions between
	// online and offline.
	EventNodeHealthChanged domainevent.EventType = "node.health.changed"
)

// HealthMonitor periodically polls Remnawave for node status and updates the
// shared in-memory NodeHealthCache. State transitions are published as domain
// events.
type HealthMonitor struct {
	remnawaveClient *remnawave.ResilientClient
	cache           *NodeHealthCache
	publisher       domainevent.Publisher
	logger          *slog.Logger
	interval        time.Duration
	maxConcurrent   int
}

// NewHealthMonitor creates a HealthMonitor with the given dependencies.
func NewHealthMonitor(
	client *remnawave.ResilientClient,
	cache *NodeHealthCache,
	publisher domainevent.Publisher,
	logger *slog.Logger,
) *HealthMonitor {
	return &HealthMonitor{
		remnawaveClient: client,
		cache:           cache,
		publisher:       publisher,
		logger:          logger,
		interval:        DefaultHealthCheckInterval,
		maxConcurrent:   DefaultMaxConcurrentChecks,
	}
}

// Run starts the periodic health check loop. It blocks until ctx is cancelled.
func (hm *HealthMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	// Run an initial check immediately on start.
	hm.checkAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.checkAll(ctx)
		}
	}
}

// checkAll fetches nodes from Remnawave, updates the cache, and publishes
// events for state transitions. Fan-out is bounded by a semaphore.
func (hm *HealthMonitor) checkAll(ctx context.Context) {
	nodes, err := hm.remnawaveClient.GetNodes(ctx)
	if err != nil {
		hm.logger.Error("health check: failed to get nodes", slog.Any("error", err))
		return
	}

	// Snapshot previous state for transition detection.
	previousByID := make(map[string]NodeHealth)
	for _, h := range hm.cache.GetAll() {
		previousByID[h.NodeID] = h
	}

	sem := make(chan struct{}, hm.maxConcurrent)
	var mu sync.Mutex
	results := make([]NodeHealth, 0, len(nodes))

	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot

		go func(n remnawave.RemnawaveNode) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			health := NodeHealth{
				NodeID:      n.UUID,
				Name:        n.Name,
				IsOnline:    n.IsConnected,
				CountryCode: extractCountryCode(n.Name),
				TrafficUsed: n.TrafficUsed,
				UpdatedAt:   time.Now(),
			}

			mu.Lock()
			results = append(results, health)
			mu.Unlock()
		}(node)
	}

	wg.Wait()
	hm.cache.Update(results)

	// Publish events for state transitions.
	for _, current := range results {
		prev, existed := previousByID[current.NodeID]
		if !existed || prev.IsOnline != current.IsOnline {
			hm.publishTransition(ctx, current)
		}
	}
}

// publishTransition emits a node.health.changed event for a single node.
func (hm *HealthMonitor) publishTransition(ctx context.Context, node NodeHealth) {
	status := "offline"
	if node.IsOnline {
		status = "online"
	}

	event := domainevent.New(EventNodeHealthChanged, map[string]any{
		"node_id": node.NodeID,
		"name":    node.Name,
		"status":  status,
		"country": node.CountryCode,
	})

	if err := hm.publisher.Publish(ctx, event); err != nil {
		hm.logger.Error("failed to publish node health event",
			slog.String("node_id", node.NodeID),
			slog.Any("error", err),
		)
	}
}

// extractCountryCode derives a two-letter country code from a node name. The
// convention is that the node name starts with "CC-" (e.g., "US-NewYork-01").
// If the prefix is absent the function returns "XX".
func extractCountryCode(name string) string {
	if len(name) >= 3 && name[2] == '-' {
		return name[:2]
	}
	return "XX"
}
