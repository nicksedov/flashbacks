package auth

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestRateLimiter() *LoginRateLimiter {
	return NewLoginRateLimiter(10, 5*time.Minute, 15*time.Minute)
}

func TestRateLimiter_Allow_Initial(t *testing.T) {
	limiter := newTestRateLimiter()

	result := limiter.Allow("192.168.1.1")

	assert.True(t, result, "fresh IP should be allowed")
}

func TestRateLimiter_Ban_AfterMaxFailures(t *testing.T) {
	limiter := newTestRateLimiter()

	// Record 10 failures (max=10)
	for i := 0; i < 10; i++ {
		limiter.RecordFailure("192.168.1.1")
	}

	result := limiter.Allow("192.168.1.1")

	assert.False(t, result, "IP should be banned after max failures")
}

func TestRateLimiter_SuccessResetsCounter(t *testing.T) {
	limiter := newTestRateLimiter()

	// Record 9 failures
	for i := 0; i < 9; i++ {
		limiter.RecordFailure("192.168.1.1")
	}

	// Record success (resets counter)
	limiter.RecordSuccess("192.168.1.1")

	// Record 9 more failures
	for i := 0; i < 9; i++ {
		limiter.RecordFailure("192.168.1.1")
	}

	result := limiter.Allow("192.168.1.1")

	assert.True(t, result, "should not be banned after success reset")
}

func TestRateLimiter_WindowReset(t *testing.T) {
	limiter := newTestRateLimiter()

	// Record some failures
	for i := 0; i < 5; i++ {
		limiter.RecordFailure("192.168.1.1")
	}

	// Manually set first attempt to outside window
	limiter.mu.Lock()
	if attempt, exists := limiter.attempts["192.168.1.1"]; exists {
		attempt.FirstAttempt = time.Now().Add(-10 * time.Minute)
	}
	limiter.mu.Unlock()

	// Should reset counter and allow
	result := limiter.Allow("192.168.1.1")

	assert.True(t, result, "should allow after window expires")

	limiter.mu.Lock()
	assert.Equal(t, 0, limiter.attempts["192.168.1.1"].Count, "counter should be reset")
	limiter.mu.Unlock()
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	limiter := newTestRateLimiter()

	// Ban IP-A
	for i := 0; i < 10; i++ {
		limiter.RecordFailure("192.168.1.1")
	}

	// IP-B should still be allowed
	result := limiter.Allow("192.168.1.2")

	assert.True(t, result, "different IP should not be affected")
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := newTestRateLimiter()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			limiter.Allow("192.168.1.1")
			limiter.RecordFailure("192.168.1.1")
		}()
	}

	wg.Wait()
	// If we reach here without panic, test passes
}
