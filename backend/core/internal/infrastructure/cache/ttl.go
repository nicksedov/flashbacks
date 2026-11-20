// Package cache provides a small in-process TTL cache used by the core service
// to keep hot read paths off Postgres.
//
// The cache is deliberately minimal: it is a map with per-entry absolute expiry
// and explicit invalidation. It never loads values itself, so a miss is always
// answered by the caller's data source.
package cache

import (
	"sync"
	"time"
)

// sweepEvery is how many writes may happen before expired entries are swept.
// The sweep keeps entries that are never read again from pinning memory.
const sweepEvery = 256

// TTLCache is a thread-safe map with a per-entry absolute expiry.
//
// Concurrent reads take only a read lock; the write lock is taken to store a
// value or to delete an entry that is observed as expired. A TTL of zero or
// less means entries never expire.
type TTLCache[K comparable, V any] struct {
	mu     sync.RWMutex
	data   map[K]entry[V]
	ttl    time.Duration
	now    func() time.Time
	writes int
}

// entry holds a cached value and its absolute expiry. A zero expiresAt means
// the entry never expires.
type entry[V any] struct {
	value     V
	expiresAt time.Time
}

// expired reports whether the entry may no longer be served.
func (e entry[V]) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && !now.Before(e.expiresAt)
}

// NewTTLCache creates a cache with the given TTL. If ttl <= 0, items never
// expire. An optional clock can be injected for testing.
func NewTTLCache[K comparable, V any](ttl time.Duration, clock func() time.Time) *TTLCache[K, V] {
	if clock == nil {
		clock = time.Now
	}
	return &TTLCache[K, V]{
		data: make(map[K]entry[V]),
		ttl:  ttl,
		now:  clock,
	}
}

// Get returns the value and whether it was a hit. An expired entry is removed
// and reported as a miss.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		var zero V
		return zero, false
	}
	if e.expired(c.now()) {
		c.mu.Lock()
		// Re-check under the write lock: another goroutine may have replaced
		// the entry with a fresh value in the meantime.
		if current, still := c.data[key]; still && current.expired(c.now()) {
			delete(c.data, key)
		}
		c.mu.Unlock()
		var zero V
		return zero, false
	}
	return e.value, true
}

// Set stores a value with the cache TTL measured from now.
func (c *TTLCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var expiresAt time.Time
	if c.ttl > 0 {
		expiresAt = c.now().Add(c.ttl)
	}
	c.data[key] = entry[V]{value: value, expiresAt: expiresAt}
	c.writes++
	if c.writes >= sweepEvery {
		c.writes = 0
		c.sweepLocked()
	}
}

// Invalidate removes a key if present.
func (c *TTLCache[K, V]) Invalidate(key K) {
	c.mu.Lock()
	delete(c.data, key)
	c.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (c *TTLCache[K, V]) InvalidateAll() {
	c.mu.Lock()
	clear(c.data)
	c.writes = 0
	c.mu.Unlock()
}

// Len returns the number of live entries. Expired entries are not counted and
// are removed as a side effect.
func (c *TTLCache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sweepLocked()
}

// DeleteExpired drops every expired entry and returns the number of live
// entries left. It lets long-lived caches reclaim memory without a background
// goroutine.
func (c *TTLCache[K, V]) DeleteExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sweepLocked()
}

// sweepLocked removes expired entries and returns the number of live entries.
// The caller must hold the write lock.
func (c *TTLCache[K, V]) sweepLocked() int {
	now := c.now()
	live := 0
	for key, e := range c.data {
		if e.expired(now) {
			delete(c.data, key)
			continue
		}
		live++
	}
	return live
}
