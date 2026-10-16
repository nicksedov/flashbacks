package imaging

import (
	"fmt"
	"sync"
	"testing"

	"github.com/flashbacks/core/internal/testutil/fixtures"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCalculateFileHash_StableUnderConcurrency guards the pooled read buffer and
// reused digest: with several workers hashing two different files in a loop, a
// buffer or digest leaking state between calls would produce a wrong digest.
func TestCalculateFileHash_StableUnderConcurrency(t *testing.T) {
	dir := fixtures.CreateTempDir(t)
	pathA := fixtures.CreateTestFile(t, dir, "a.jpg", fixedContent(4096, 'A'))
	pathB := fixtures.CreateTestFile(t, dir, "b.jpg", fixedContent(4096, 'B'))

	wantA, err := CalculateFileHash(pathA)
	require.NoError(t, err)
	wantB, err := CalculateFileHash(pathB)
	require.NoError(t, err)
	require.NotEqual(t, wantA, wantB, "the fixtures must have different hashes")

	const (
		workers = 8
		rounds  = 200
	)

	var wg sync.WaitGroup
	failures := make(chan string, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				gotA, err := CalculateFileHash(pathA)
				if err != nil {
					failures <- err.Error()
					return
				}
				if gotA != wantA {
					failures <- fmt.Sprintf("file A: got %s, want %s", gotA, wantA)
					return
				}

				gotB, err := CalculateFileHash(pathB)
				if err != nil {
					failures <- err.Error()
					return
				}
				if gotB != wantB {
					failures <- fmt.Sprintf("file B: got %s, want %s", gotB, wantB)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(failures)

	for msg := range failures {
		t.Fatal(msg)
	}
}

// TestCalculateFileHash_BoundedAllocations asserts the hashing cost is a constant,
// small number of allocations independent of file size — i.e. the read buffer is
// reused rather than freshly allocated per call.
//
// The budget is deliberately below the pre-optimisation cost (7 allocations per
// call, a constant 33 KB), so the assertion fails if the pooled buffer is
// removed.
func TestCalculateFileHash_BoundedAllocations(t *testing.T) {
	const budget = 4.0

	for _, size := range []int{1 << 10, 1 << 20} {
		t.Run(fmt.Sprintf("%dKB", size>>10), func(t *testing.T) {
			dir := fixtures.CreateTempDir(t)
			path := fixtures.CreateTestFile(t, dir, "blob.jpg", fixedContent(size, 'q'))

			// Warm the pool so the first-call fill is not attributed to the
			// measured run.
			_, err := CalculateFileHash(path)
			require.NoError(t, err)

			allocs := testing.AllocsPerRun(50, func() {
				if _, err := CalculateFileHash(path); err != nil {
					t.Fatalf("hashing failed: %v", err)
				}
			})

			t.Logf("CalculateFileHash allocations per call: %.1f", allocs)
			assert.LessOrEqual(t, allocs, budget,
				"hashing must reuse a pooled buffer instead of allocating one per call")
		})
	}
}
