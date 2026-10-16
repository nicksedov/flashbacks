package imaging

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"
	"github.com/flashbacks/core/internal/testutil"
	"github.com/flashbacks/core/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedContent returns a deterministic payload of the given length. IsImageFile
// classifies by extension only, so an exact byte length can be used to control
// the scanned file size precisely.
func fixedContent(n int, fill byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = fill
	}
	return b
}

// TestFastScanGalleryDirectory_StateEquivalence pins the complete sync outcome —
// unchanged / modified / created / deleted classification plus the
// ContentChanged flag of every updated record — for a tree that exercises every
// case at once.
//
// It is the regression guard for the write-batching optimisation: batching must
// not change a single classification or flag.
func TestFastScanGalleryDirectory_StateEquivalence(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	dir := fixtures.CreateTempDir(t)

	// Seed: four files the second scan will classify differently.
	unchangedPath := fixtures.CreateTestFile(t, dir, "unchanged.jpg", fixedContent(100, 'A'))
	sizeChangedPath := fixtures.CreateTestFile(t, dir, "size_changed.jpg", fixedContent(100, 'B'))
	sameSizePath := fixtures.CreateTestFile(t, dir, "same_size.jpg", fixedContent(150, 'C'))
	deletedPath := fixtures.CreateTestFile(t, dir, "deleted.jpg", fixedContent(120, 'D'))

	seed := make(chan string, 1000)
	seedBatch, err := scanDirectory(db, dir, seed, 2)
	require.NoError(t, err)
	require.Len(t, seedBatch.Created, 4, "seed scan must create all four files")

	var seedCount int64
	db.Model(&domain.ImageFile{}).Count(&seedCount)
	require.Equal(t, int64(4), seedCount)

	// Capture the creation timestamp the seed scan assigned, so the later update
	// can be proven to preserve it.
	var seeded domain.ImageFile
	require.NoError(t, db.Where("path = ?", filepath.ToSlash(sizeChangedPath)).First(&seeded).Error)
	require.False(t, seeded.CreatedAt.IsZero(), "the seed insert must stamp created_at")

	// Mutate the tree between scans.
	// 1. size change: different byte count.
	fixtures.CreateTestFile(t, dir, "size_changed.jpg", fixedContent(250, 'B'))
	// 2. same-size content change: identical length, different bytes. Its
	//    modification time must differ by more than the one-second truncation
	//    the scanner uses, or the file is (correctly) treated as unchanged.
	fixtures.CreateTestFile(t, dir, "same_size.jpg", fixedContent(150, 'Z'))
	future := time.Now().Add(5 * time.Second)
	require.NoError(t, os.Chtimes(sameSizePath, future, future))
	// 3. a new file.
	newPath := fixtures.CreateTestFile(t, dir, "new.jpg", fixedContent(180, 'E'))
	// 4. a deleted file.
	require.NoError(t, os.Remove(deletedPath))

	progress := make(chan string, 1000)
	stats, batch := fastScanGalleryDirectory(db, dir, progress, 2)
	t.Logf("stats=%+v created=%d updated=%d deleted=%d",
		stats, len(batch.Created), len(batch.Updated), len(batch.Deleted))

	// Classification counts.
	assert.Equal(t, 1, stats.Unchanged, "unchanged.jpg must be reported unchanged")
	assert.Equal(t, 2, stats.Modified, "size_changed.jpg and same_size.jpg must be reported modified")
	assert.Equal(t, 1, stats.Created, "new.jpg must be reported created")
	assert.Equal(t, 1, stats.Deleted, "deleted.jpg must be reported deleted")

	// Batch result sets.
	require.Len(t, batch.Created, 1)
	assert.Equal(t, filepath.ToSlash(newPath), batch.Created[0].Path)
	require.Len(t, batch.Updated, 2, "exactly the two modified files must be updated")
	require.Len(t, batch.Deleted, 1)
	assert.Equal(t, filepath.ToSlash(deletedPath), batch.Deleted[0].Path)

	// Each updated record must flag a content change, keyed to the right file.
	changedByPath := map[string]bool{}
	for _, u := range batch.Updated {
		changedByPath[u.File.Path] = u.ContentChanged
	}
	assert.True(t, changedByPath[filepath.ToSlash(sizeChangedPath)], "size change must flag ContentChanged")
	assert.True(t, changedByPath[filepath.ToSlash(sameSizePath)], "same-size content change must flag ContentChanged")

	// Final database state.
	var rows []domain.ImageFile
	require.NoError(t, db.Order("path").Find(&rows).Error)
	require.Len(t, rows, 4, "deleted.jpg must be gone and the other four must remain")

	remaining := map[string]bool{}
	for _, r := range rows {
		remaining[r.Path] = true
	}
	for _, p := range []string{unchangedPath, sizeChangedPath, sameSizePath, newPath} {
		assert.True(t, remaining[filepath.ToSlash(p)], "%s must remain in the database", p)
	}
	assert.False(t, remaining[filepath.ToSlash(deletedPath)], "deleted.jpg must not remain in the database")

	// Pin the persisted row of an updated file so batching cannot change any
	// column, including the timestamps GORM manages.
	wantHash, err := CalculateFileHash(sizeChangedPath)
	require.NoError(t, err)

	var updatedRow domain.ImageFile
	require.NoError(t, db.Where("path = ?", filepath.ToSlash(sizeChangedPath)).First(&updatedRow).Error)
	t.Logf("updated row: createdAt=%v updatedAt=%v", updatedRow.CreatedAt, updatedRow.UpdatedAt)

	assert.Equal(t, int64(250), updatedRow.Size, "updated size must be persisted")
	assert.Equal(t, wantHash, updatedRow.Hash, "updated hash must be persisted")

	// The update path must no longer destroy the creation timestamp (the removed
	// per-row Save overwrote it with the zero time).
	assert.True(t, updatedRow.CreatedAt.Equal(seeded.CreatedAt),
		"created_at must survive an update unchanged: got %v, want %v",
		updatedRow.CreatedAt, seeded.CreatedAt)
	assert.False(t, updatedRow.UpdatedAt.IsZero(), "updated_at must be stamped by the update")
}

// TestScanDirectory_UpdateStatementsAreBatched asserts that updating many files
// costs far fewer SQL write statements than files. Statement count is the
// deterministic acceptance criterion for the batching optimisation, because
// in-memory SQLite wall-clock time cannot reveal the per-row cost that dominates
// on Postgres.
func TestScanDirectory_UpdateStatementsAreBatched(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	counter := newStatementCounter()
	db.Logger = counter

	dir := fixtures.CreateTempDir(t)
	const files = 40
	for i := 0; i < files; i++ {
		fixtures.CreateTestFile(t, dir, fmt.Sprintf("img_%03d.jpg", i), fixedContent(100+i, byte('a'+i%26)))
	}

	// Seed scan (creates); its statements are excluded from the measurement.
	seed := make(chan string, 1000)
	_, err := scanDirectory(db, dir, seed, 2)
	require.NoError(t, err)

	// Change every file so the second scan must update every row.
	for i := 0; i < files; i++ {
		fixtures.CreateTestFile(t, dir, fmt.Sprintf("img_%03d.jpg", i), fixedContent(500+i, byte('A'+i%26)))
	}

	counter.reset()

	progress := make(chan string, 1000)
	batch, err := scanDirectory(db, dir, progress, 2)
	require.NoError(t, err)
	require.Len(t, batch.Updated, files, "every file must be updated")

	counts, total := counter.snapshot()
	t.Logf("statements for %d updated files: %v (total %d)", files, counts, total)

	assert.Less(t, counter.writes(), files,
		"writes must be batched (fewer statements than rows), not one UPDATE per file")
}
