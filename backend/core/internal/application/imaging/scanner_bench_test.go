package imaging

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/flashbacks/core/internal/domain"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Benchmarks for the gallery scan → database sync path.
//
//	cd backend/core && go test ./internal/application/imaging/ -run '^$' -bench=. -benchmem -count=5
//
// Every benchmark reports allocations so a change that trades CPU for memory
// churn (or vice versa) is visible in the before/after diff.

// Package-level sinks prevent the compiler from eliminating the work under
// measurement.
var (
	sinkHash  string
	sinkBatch ScanBatchResult
)

// benchDB opens a fresh in-memory SQLite database with the image_files table,
// mirroring the production table this path writes to.
func benchDB(b *testing.B) *gorm.DB {
	b.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	require.NoError(b, err)

	sqlDB, err := db.DB()
	require.NoError(b, err)
	// In-memory SQLite is scoped to one connection; pin the pool so every
	// statement sees the same schema.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	require.NoError(b, db.AutoMigrate(&domain.ImageFile{}))
	return db
}

// BenchmarkCalculateFileHash measures hashing cost across representative sizes.
func BenchmarkCalculateFileHash(b *testing.B) {
	for _, size := range []int{1 << 10, 64 << 10, 1 << 20} {
		b.Run(fmt.Sprintf("%dKB", size>>10), func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "blob.jpg")
			require.NoError(b, os.WriteFile(path, fixedContent(size, 'q'), 0o644))

			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				hash, err := CalculateFileHash(path)
				if err != nil {
					b.Fatal(err)
				}
				sinkHash = hash
			}
		})
	}
}

// BenchmarkFlushBatchUpdates measures the database write path for a full batch
// of updates — the hot path the batching optimisation targets. The batch size
// matches the post-optimisation writeBatchSize.
func BenchmarkFlushBatchUpdates(b *testing.B) {
	const rows = 500

	db := benchDB(b)

	seeded := make([]domain.ImageFile, rows)
	for i := range seeded {
		seeded[i] = domain.ImageFile{
			Path:    fmt.Sprintf("/gallery/img_%04d.jpg", i),
			Size:    int64(i),
			Hash:    "old",
			ModTime: time.Now(),
		}
	}
	require.NoError(b, db.Create(&seeded).Error)

	base := make([]domain.ImageFile, rows)
	for i := range base {
		base[i] = seeded[i]
		base[i].Hash = "new"
	}
	toUpdate := make([]domain.ImageFile, rows)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		// flushBatch truncates the slice, so restore its length and refill from
		// the prepared rows (a constant-size copy, no allocation).
		toUpdate = toUpdate[:rows]
		copy(toUpdate, base)

		var toCreate []domain.ImageFile
		sinkBatch = flushBatch(db, &toCreate, &toUpdate)
	}
}

// BenchmarkScanDirectoryFirstScan measures a full scan indexing a directory tree
// of new files. Fixture creation and database setup happen with the timer
// stopped, so the measured time is the scan itself.
//
// The timer must be running when b.Loop() is evaluated again, so the body ends
// with StartTimer after its cleanup.
func BenchmarkScanDirectoryFirstScan(b *testing.B) {
	const files = 50

	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		db := benchDB(b)
		dir := b.TempDir()
		for i := 0; i < files; i++ {
			name := fmt.Sprintf("img_%03d.jpg", i)
			payload := fixedContent(256+i%32, byte('a'+i%26))
			if err := os.WriteFile(filepath.Join(dir, name), payload, 0o644); err != nil {
				b.Fatal(err)
			}
		}
		progress := make(chan string, files*2)
		b.StartTimer()

		batch, err := scanDirectory(db, dir, progress, 2)

		b.StopTimer()
		close(progress)
		if err != nil {
			b.Fatal(err)
		}
		sinkBatch = batch
		b.StartTimer()
	}
}
