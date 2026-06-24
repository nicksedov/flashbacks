// Package background provides a reusable Manager for coordinating background worker goroutines.
// It handles start/stop lifecycle, prevents double-start, and provides a stop channel
// for graceful shutdown coordination.
package background

import "sync"

// Manager provides lifecycle management for a single background worker.
// It is safe for concurrent use and prevents double-start or double-stop panics.
type Manager struct {
	mu       sync.Mutex
	running  bool
	stopChan chan struct{}
	Name     string // for logging
}

// New creates a new Manager with the given name.
func New(name string) *Manager {
	return &Manager{
		stopChan: make(chan struct{}),
		Name:     name,
	}
}

// TryStart attempts to start the manager.
// Returns true if the manager was not running and is now started.
// Returns false if already running (no-op).
func (m *Manager) TryStart() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return false
	}
	m.running = true
	m.stopChan = make(chan struct{}) // recreate for reuse after Stop/Start cycle
	return true
}

// Stop signals the worker to stop and marks it as not running.
// Safe to call multiple times.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
}

// Done returns a channel that is closed when Stop is called.
// Use in select statements to detect shutdown.
func (m *Manager) Done() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopChan
}

// IsRunning returns whether the manager is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

// MarkStopped marks the manager as no longer running.
// Call this in a defer from the worker's run function.
func (m *Manager) MarkStopped() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
}
