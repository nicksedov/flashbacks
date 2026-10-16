package imaging

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/logger"
)

// statementCounter is a gorm logger that counts executed SQL statements by their
// leading keyword.
//
// It exists because the scan→DB-sync cost we care about is the *number* of
// statements and transactions, not elapsed time: the per-row versus batched
// write difference is orders of magnitude on Postgres but nearly invisible on
// the in-memory SQLite used by unit tests. Counting statements makes that
// difference deterministic and measurable without a Postgres instance.
//
// The counter is used through a pointer, so it keeps accumulating even when GORM
// copies the Logger value (for example through Session): every copy holds the
// same pointer.
type statementCounter struct {
	logger.Interface

	mu     sync.Mutex
	counts map[string]int
	total  int
}

func newStatementCounter() *statementCounter {
	return &statementCounter{
		Interface: logger.Discard,
		counts:    make(map[string]int),
	}
}

// LogMode keeps this counting logger installed regardless of the requested
// level, so an accidental Debug() call cannot swap it out.
func (c *statementCounter) LogMode(logger.LogLevel) logger.Interface { return c }

// Trace satisfies logger.Interface and records the statement's leading keyword.
func (c *statementCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()

	c.mu.Lock()
	c.total++
	c.counts[classifySQL(sql)]++
	c.mu.Unlock()
}

// classifySQL returns the leading SQL keyword, stripping an optional leading
// comment (GORM prefixes some statements with an operation comment).
func classifySQL(sql string) string {
	s := strings.TrimSpace(sql)
	if strings.HasPrefix(s, "/*") {
		if end := strings.Index(s, "*/"); end >= 0 {
			s = strings.TrimSpace(s[end+2:])
		}
	}
	if i := strings.IndexAny(s, " \t\r\n("); i >= 0 {
		s = s[:i]
	}
	return strings.ToUpper(s)
}

// writes returns the number of INSERT/UPDATE/DELETE statements executed.
func (c *statementCounter) writes() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts["INSERT"] + c.counts["UPDATE"] + c.counts["DELETE"]
}

// snapshot returns a copy of the per-keyword counts and the running total.
func (c *statementCounter) snapshot() (map[string]int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.counts))
	for k, v := range c.counts {
		out[k] = v
	}
	return out, c.total
}

// reset zeroes every counter.
func (c *statementCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts = make(map[string]int)
	c.total = 0
}

// TestStatementCounter_CountsReadsAndWrites proves the harness actually observes
// GORM statements, so the batched-write assertion later cannot pass vacuously.
func TestStatementCounter_CountsReadsAndWrites(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	counter := newStatementCounter()
	db.Logger = counter

	var count int64
	require.NoError(t, db.Model(&domain.ImageFile{}).Count(&count).Error)
	require.NoError(t, db.Create(&domain.ImageFile{Path: "/gallery/a.jpg", Hash: "h", Size: 10}).Error)

	got, total := counter.snapshot()
	t.Logf("counted statements: %v (total %d)", got, total)

	assert.GreaterOrEqual(t, total, 2, "counter must observe statements")
	assert.GreaterOrEqual(t, got["SELECT"], 1, "the count query must be classified as SELECT")
	assert.GreaterOrEqual(t, got["INSERT"], 1, "the create must be classified as INSERT")
}

// TestStatementCounter_Reset ensures counters can be zeroed between phases.
func TestStatementCounter_Reset(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	counter := newStatementCounter()
	db.Logger = counter

	require.NoError(t, db.Create(&domain.ImageFile{Path: "/gallery/b.jpg", Hash: "h", Size: 10}).Error)
	_, before := counter.snapshot()
	require.Greater(t, before, 0)

	counter.reset()

	_, after := counter.snapshot()
	assert.Zero(t, after, "reset must zero the counters")
}
