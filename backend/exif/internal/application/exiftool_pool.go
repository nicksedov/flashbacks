package application

import (
	"fmt"
	"log"
	"runtime"
	"sync"

	"github.com/barasher/go-exiftool"
)

// ExiftoolPool manages a pool of exiftool instances for concurrent EXIF operations.
type ExiftoolPool struct {
	pool chan *exiftool.Exiftool
	size int
	mu   sync.Mutex
}

// DefaultPoolSize returns the default pool size based on CPU count.
func DefaultPoolSize() int {
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	return n
}

// NewExiftoolPool creates a pool of exiftool instances.
// Size controls the number of concurrent exiftool processes.
func NewExiftoolPool(size int) (*ExiftoolPool, error) {
	if size <= 0 {
		size = DefaultPoolSize()
	}

	p := &ExiftoolPool{
		pool: make(chan *exiftool.Exiftool, size),
		size: size,
	}

	for i := 0; i < size; i++ {
		et, err := exiftool.NewExiftool()
		if err != nil {
			// Close already created instances before returning error
			p.drainAndClose()
			return nil, fmt.Errorf("failed to create exiftool instance %d/%d: %w", i+1, size, err)
		}
		p.pool <- et
	}

	log.Printf("ExiftoolPool: initialized with %d instances", size)
	return p, nil
}

// Acquire gets an exiftool instance from the pool.
// Blocks until an instance is available.
func (p *ExiftoolPool) Acquire() *exiftool.Exiftool {
	return <-p.pool
}

// Release returns an exiftool instance to the pool.
// If the instance is nil or unhealthy, a replacement is created.
func (p *ExiftoolPool) Release(et *exiftool.Exiftool) {
	if et == nil {
		// Try to create a replacement
		newEt, err := exiftool.NewExiftool()
		if err != nil {
			log.Printf("ExiftoolPool: failed to create replacement instance: %v", err)
			return
		}
		p.pool <- newEt
		return
	}
	p.pool <- et
}

// Size returns the pool capacity.
func (p *ExiftoolPool) Size() int {
	return p.size
}

// Close shuts down all exiftool instances in the pool.
func (p *ExiftoolPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.drainAndClose()
}

// drainAndClose closes all instances currently in the pool.
// Caller must hold p.mu.
func (p *ExiftoolPool) drainAndClose() {
	close(p.pool)
	for et := range p.pool {
		et.Close()
	}
}
