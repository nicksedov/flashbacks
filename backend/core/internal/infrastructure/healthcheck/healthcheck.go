// Package healthcheck provides a unified periodic health checker for external
// HTTP services (OCR, EXIF). Eliminates duplicated healthcheck logic across
// client implementations.
package healthcheck

import (
	"context"
	"sync"
	"time"
)

// Status represents the current health status.
type Status string

const (
	StatusUnknown   Status = "unknown"
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
)

// Checker is implemented by each external service client to provide
// a health check against its backend.
type Checker interface {
	Check(ctx context.Context) error
}

// HealthStatus holds the current health check result.
type HealthStatus struct {
	Status    Status    `json:"healthStatus"`
	Error     string    `json:"error,omitempty"`
	LastCheck time.Time `json:"lastCheck"`
}

// PeriodicChecker runs health checks on a configurable interval.
type PeriodicChecker struct {
	mu      sync.RWMutex
	checker Checker
	status  HealthStatus
	stopCh  chan struct{}
	running bool
	timeout time.Duration // per-check timeout
}

// NewPeriodicChecker creates a PeriodicChecker for the given Checker.
// interval is the time between health checks. timeout is the per-check timeout.
func NewPeriodicChecker(checker Checker, interval time.Duration, timeout time.Duration) *PeriodicChecker {
	return &PeriodicChecker{
		checker: checker,
		status: HealthStatus{
			Status:    StatusUnknown,
			LastCheck: time.Now(),
		},
		stopCh:  make(chan struct{}),
		timeout: timeout,
	}
}

// Start begins the periodic health check loop. Does nothing if already running.
func (pc *PeriodicChecker) Start(interval time.Duration) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.running {
		return
	}
	pc.running = true
	go pc.loop(interval)
}

// Stop stops the periodic health check loop.
func (pc *PeriodicChecker) Stop() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.running {
		return
	}
	close(pc.stopCh)
	pc.running = false
}

// IsHealthy returns true if the last health check succeeded.
func (pc *PeriodicChecker) IsHealthy() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.status.Status == StatusHealthy
}

// GetStatus returns the current health status.
func (pc *PeriodicChecker) GetStatus() HealthStatus {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.status
}

func (pc *PeriodicChecker) loop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-pc.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), pc.timeout)
			err := pc.checker.Check(ctx)
			cancel()

			pc.mu.Lock()
			pc.status.LastCheck = time.Now()
			if err != nil {
				pc.status.Status = StatusUnhealthy
				pc.status.Error = err.Error()
			} else {
				pc.status.Status = StatusHealthy
				pc.status.Error = ""
			}
			pc.mu.Unlock()
		}
	}
}
