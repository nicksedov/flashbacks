// Package retry provides a unified retry mechanism for external HTTP service calls.
// All HTTP clients (exifclient, ocr, geocoder) should use WithRetry for consistent
// exponential backoff behavior.
package retry

import (
	"context"
	"math"
	"time"
)

// BackoffStrategy defines how the delay between attempts grows.
type BackoffStrategy int

const (
	// BackoffConstant keeps the same delay between all attempts.
	BackoffConstant BackoffStrategy = iota
	// BackoffLinear grows the delay linearly: delay * attempt.
	BackoffLinear
	// BackoffExponential grows the delay exponentially: delay * 2^(attempt-1).
	BackoffExponential
)

// Config holds the retry configuration.
type Config struct {
	MaxAttempts int
	Delay       time.Duration
	MaxDelay    time.Duration
	Backoff     BackoffStrategy
}

// DefaultConfig returns a sensible default retry configuration.
// 3 attempts with 1s initial delay, exponential backoff up to 30s.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		Delay:       1 * time.Second,
		MaxDelay:    30 * time.Second,
		Backoff:     BackoffExponential,
	}
}

// WithRetry executes fn with retries according to cfg. If all attempts fail,
// the last error is returned. If ctx is cancelled, the context error is returned.
func WithRetry[T any](ctx context.Context, cfg Config, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if attempt == cfg.MaxAttempts {
			break
		}

		delay := calcDelay(cfg, attempt)

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return zero, lastErr
}

func calcDelay(cfg Config, attempt int) time.Duration {
	var delay time.Duration
	switch cfg.Backoff {
	case BackoffExponential:
		delay = cfg.Delay * time.Duration(math.Pow(2, float64(attempt-1)))
	case BackoffLinear:
		delay = cfg.Delay * time.Duration(attempt)
	default:
		delay = cfg.Delay
	}
	if delay > cfg.MaxDelay {
		delay = cfg.MaxDelay
	}
	return delay
}
