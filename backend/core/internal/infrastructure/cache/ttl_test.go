package cache_test

import (
	"sync"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/infrastructure/cache"
)

// fakeClock is a manually advanced time source.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1_000_000_000, 0)}
}

func TestTTLCacheHitMissAndInvalidation(t *testing.T) {
	clock := newFakeClock()
	c := cache.NewTTLCache[string, string](5*time.Second, clock.Now)

	if _, ok := c.Get("k"); ok {
		t.Fatal("expected a miss on an empty cache")
	}

	c.Set("k", "v")
	if v, ok := c.Get("k"); !ok || v != "v" {
		t.Fatalf("expected hit v, got %q (hit=%v)", v, ok)
	}

	// A hit must not refresh the TTL: the entry still expires 5s after the Set.
	clock.Advance(4 * time.Second)
	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected a hit before expiry")
	}
	clock.Advance(time.Second)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected a miss at the expiry instant")
	}
	if live := c.Len(); live != 0 {
		t.Fatalf("expected the expired entry to be swept, got %d", live)
	}

	c.Set("k", "v")
	c.Invalidate("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected Invalidate to remove the entry")
	}

	c.Set("a", "1")
	c.Set("b", "2")
	c.InvalidateAll()
	if _, ok := c.Get("a"); ok {
		t.Fatal("expected InvalidateAll to clear a")
	}
	if _, ok := c.Get("b"); ok {
		t.Fatal("expected InvalidateAll to clear b")
	}
}

func TestTTLCacheNonPositiveTTLNeverExpires(t *testing.T) {
	for _, ttl := range []time.Duration{0, -time.Minute} {
		clock := newFakeClock()
		c := cache.NewTTLCache[string, int](ttl, clock.Now)
		c.Set("k", 42)

		clock.Advance(1000 * time.Hour)
		if v, ok := c.Get("k"); !ok || v != 42 {
			t.Fatalf("ttl=%v: expected a permanent hit, got %d (hit=%v)", ttl, v, ok)
		}
		if live := c.Len(); live != 1 {
			t.Fatalf("ttl=%v: expected one live entry, got %d", ttl, live)
		}
	}
}

func TestTTLCacheLenCountsOnlyLiveEntries(t *testing.T) {
	clock := newFakeClock()
	c := cache.NewTTLCache[string, int](time.Minute, clock.Now)

	c.Set("short", 1)
	clock.Advance(30 * time.Second)
	c.Set("fresh", 2) // sweeps at most, both are live
	clock.Advance(31 * time.Second)

	// "short" expired while untouched; Len must not count it and must drop it.
	if live := c.Len(); live != 1 {
		t.Fatalf("expected 1 live entry, got %d", live)
	}
	if _, ok := c.Get("short"); ok {
		t.Fatal("expected the expired entry to be gone after Len")
	}
	if _, ok := c.Get("fresh"); !ok {
		t.Fatal("expected the fresh entry to survive")
	}
}

func TestTTLCacheDeleteExpiredReclaimsMemory(t *testing.T) {
	clock := newFakeClock()
	c := cache.NewTTLCache[int, int](time.Second, clock.Now)

	for i := 0; i < 10; i++ {
		c.Set(i, i)
	}
	clock.Advance(2 * time.Second)

	if removed := c.DeleteExpired(); removed != 0 {
		t.Fatalf("expected 0 live entries after expiry, got %d", removed)
	}
	if live := c.Len(); live != 0 {
		t.Fatalf("expected an empty cache, got %d entries", live)
	}
}

// TestTTLCacheConcurrentAccess exercises the lock discipline. Run with -race.
func TestTTLCacheConcurrentAccess(t *testing.T) {
	c := cache.NewTTLCache[int, int](time.Millisecond, nil)

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := (worker*7 + i) % 16
				c.Set(key, i)
				// A miss between Set and Get is legitimate (the entry may have
				// expired), so only the call itself matters here.
				_, _ = c.Get(key)
				if i%17 == 0 {
					c.Invalidate(key)
				}
				if i%53 == 0 {
					c.Len()
				}
				if i%101 == 0 {
					c.DeleteExpired()
				}
			}
		}(worker)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			c.InvalidateAll()
		}
	}()
	wg.Wait()

	if live := c.Len(); live < 0 {
		t.Fatalf("live entries must never be negative, got %d", live)
	}
}
