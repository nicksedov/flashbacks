package auth

import (
	"context"
	"log"
	"sync"
	"time"
)

// SessionCleanupJob periodically cleans up expired sessions
type SessionCleanupJob struct {
	sessionRepo *SessionRepository
	interval    time.Duration
	stopCh      chan struct{}
	mu          sync.Mutex
	running     bool
}

// NewSessionCleanupJob creates a new session cleanup job
func NewSessionCleanupJob(sessionRepo *SessionRepository, interval time.Duration) *SessionCleanupJob {
	if interval == 0 {
		interval = 1 * time.Hour // Default: run every hour
	}
	return &SessionCleanupJob{
		sessionRepo: sessionRepo,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the periodic cleanup job.
// Safe to call multiple times; subsequent calls are no-ops while running.
func (j *SessionCleanupJob) Start() {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return
	}
	j.running = true
	j.stopCh = make(chan struct{}) // recreate for reuse after Stop/Start cycle
	j.mu.Unlock()

	go func() {
		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()

		log.Printf("[SessionCleanup] Started, running every %v", j.interval)

		for {
			select {
			case <-ticker.C:
				j.runCleanup()
			case <-j.stopCh:
				j.mu.Lock()
				j.running = false
				j.mu.Unlock()
				log.Println("[SessionCleanup] Stopped")
				return
			}
		}
	}()
}

// Stop stops the cleanup job. Safe to call multiple times.
func (j *SessionCleanupJob) Stop() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.running {
		return
	}
	j.running = false
	close(j.stopCh)
}

// runCleanup performs a single cleanup operation
func (j *SessionCleanupJob) runCleanup() {
	if err := j.sessionRepo.CleanupExpiredSessions(context.Background()); err != nil {
		log.Printf("[SessionCleanup] Error cleaning up sessions: %v", err)
	} else {
		log.Println("[SessionCleanup] Cleanup completed successfully")
	}
}
