// Package proxy provides the SubscriptionProxy which serves VPN subscription
// configs to clients through a multi-tier cache (L1 in-memory LRU, L2 Valkey,
// L3 Remnawave).
package proxy

import (
	"sync"
	"time"
)

// lruEntry holds a cached value with its expiration time.
type lruEntry struct {
	body      []byte
	expiresAt time.Time
}

// LRUCache is a thread-safe bounded cache with LRU eviction and TTL.
type LRUCache struct {
	mu      sync.Mutex
	entries map[string]*lruNode
	head    *lruNode // most recently used
	tail    *lruNode // least recently used
	maxSize int
}

type lruNode struct {
	key   string
	entry lruEntry
	prev  *lruNode
	next  *lruNode
}

// NewLRUCache creates a bounded LRU cache that holds at most maxSize entries.
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		entries: make(map[string]*lruNode, maxSize),
		maxSize: maxSize,
	}
}

// Get retrieves an entry by key. It returns the cached body and true on hit,
// or nil and false on miss or expiration. Accessing an entry promotes it to
// the most-recently-used position.
func (c *LRUCache) Get(key string, now time.Time) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if now.After(node.entry.expiresAt) {
		c.removeNode(node)
		delete(c.entries, key)
		return nil, false
	}
	c.moveToFront(node)
	return node.entry.body, true
}

// Set inserts or updates an entry. If the cache is at capacity, the
// least-recently-used entry is evicted.
func (c *LRUCache) Set(key string, body []byte, expiresAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.entries[key]; ok {
		node.entry = lruEntry{body: body, expiresAt: expiresAt}
		c.moveToFront(node)
		return
	}

	if len(c.entries) >= c.maxSize {
		c.evictLRU()
	}

	node := &lruNode{key: key, entry: lruEntry{body: body, expiresAt: expiresAt}}
	c.entries[key] = node
	c.addToFront(node)
}

// Len returns the number of entries currently in the cache.
func (c *LRUCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// moveToFront detaches node from its current position and places it at the head.
func (c *LRUCache) moveToFront(node *lruNode) {
	if c.head == node {
		return
	}
	c.removeNode(node)
	c.addToFront(node)
}

// addToFront inserts node at the head of the doubly-linked list.
func (c *LRUCache) addToFront(node *lruNode) {
	node.prev = nil
	node.next = c.head
	if c.head != nil {
		c.head.prev = node
	}
	c.head = node
	if c.tail == nil {
		c.tail = node
	}
}

// removeNode detaches node from the doubly-linked list.
func (c *LRUCache) removeNode(node *lruNode) {
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}
	node.prev = nil
	node.next = nil
}

// evictLRU removes the least-recently-used entry (the tail).
func (c *LRUCache) evictLRU() {
	if c.tail == nil {
		return
	}
	delete(c.entries, c.tail.key)
	c.removeNode(c.tail)
}
