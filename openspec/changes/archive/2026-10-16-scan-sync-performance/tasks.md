# Tasks

## 1. Measurement harness and regression guards (before any optimisation)

- [x] 1.1 Add a counting `gorm.Logger` in a new test helper
      (`internal/application/imaging/statement_counter_test.go`) that increments
      per-operation counters in `Trace`; verified it counts
      `Query`/`Create`/`Update` against the in-memory SQLite test DB
      (`map[INSERT:1 SELECT:1]`).
- [x] 1.2 Add `BenchmarkCalculateFileHash` and a golden-digest test extending
      [`TestCalculateFileHash_Success`](../../../backend/core/internal/application/imaging/scanner_test.go:16)
      so the canonical MD5 of a fixed payload is pinned (`MD5("hello world")`);
      verified green.
- [x] 1.3 Add a full-sync state-equivalence test over a fixture tree containing
      unchanged, size-changed, content-changed-at-same-size, new, and deleted
      files (`TestFastScanGalleryDirectory_StateEquivalence`), asserting the exact
      created/updated/deleted sets and each `ContentChanged` flag; verified green
      with `stats={Unchanged:1 Modified:2 Created:1 Deleted:1}`.
- [x] 1.4 Add `BenchmarkCalculateFileHash`, `BenchmarkFlushBatchUpdates` and
      `BenchmarkScanDirectoryFirstScan` with `b.ReportAllocs()`; captured the
      baseline (see 1.5) plus the statement count from 1.1
      (40 updated files = `map[SELECT:2 UPDATE:40]`).
- [x] 1.5 Captured baseline escape analysis, coverage (**52.6%**), and
      `ns/op`/`B/op`/`allocs/op` for every benchmark.

## 2. P1 — Batch the database writes

- [x] 2.1 Replaced the per-row `db.Save(&f)` in
      [`flushBatch`](../../../backend/core/internal/application/imaging/scanner.go:99)
      with a single batched slice write; `ContentChanged` computation and
      `ScanBatchResult` are unchanged.
- [x] 2.2 Raised `writeBatchSize` from 50 to 500 in both `scanDirectory` and
      `fastScanGalleryDirectory`.
- [x] 2.3 State-equivalence test and all existing `TestScanDirectory_*` /
      `TestFastScanGalleryDirectory_*` / post-processor tests pass with no
      assertion weakened.
- [x] 2.4 Re-ran `BenchmarkFlushBatchUpdates` and the statement counter:
      **writes 40 → 1** for 40 files (`map[INSERT:1 SELECT:2]`), and
      `BenchmarkFlushBatchUpdates` **14.8ms → 8.5ms**, **4.6MB → 1.0MB**,
      **53,740 → 13,990 allocs/op**.
- [x] 2.5 **Approved behaviour correction:** the removed per-row `Save` also
      overwrote `created_at` with the zero time; the batched write preserves it.
      Recorded in `design.md` (D2) and specified by the
      `gallery-browsing.scan-preserves-created-at` scenario.

## 3. P2 — Pool the hash read buffer

- [x] 3.1 Reworked
      [`CalculateFileHash`](../../../backend/core/internal/application/imaging/scanner.go:48)
      to use a `sync.Pool`-backed 32 KiB buffer, a reused digest, and stack hex
      storage. Reading is done directly with the pooled buffer because
      `*os.File` implements `io.WriterTo`, which made `io.CopyBuffer` ignore the
      supplied buffer and allocate its own 32 KiB per call.
- [x] 3.2 Golden-digest test, `TestCalculateFileHash_*` and the concurrency
      stability test pass; digests are unchanged.
- [x] 3.3 Re-ran `BenchmarkCalculateFileHash`:
      `allocs 7 → 4`, `B/op 33,080 → ~202`, `1KB ns/op 9.7µs → 3.66µs`;
      escape analysis confirms the pooled buffer no longer escapes and the only
      remaining per-call allocations are `os.Open`'s handle and the returned
      string.

## 4. Verification and evidence

- [x] 4.1 `cd backend/core && go build ./... && go vet ./...` pass.
- [x] 4.2 `go test ./internal/application/imaging/ -race -count=1` passes and
      coverage rose **52.6% → 53.0%** (no reduction).
- [x] 4.3 Bound the five scenario IDs in
      [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml);
      `focused-spec validate --scope current` reports no new issues (only
      pre-existing markers in `llm-model-metadata` / `llm-settings`).
- [x] 4.4 Ran the controlled revert→FAIL→restore→PASS probe for every new
      evidence reference. With the pre-optimisation `scanner.go` restored,
      `TestScanDirectory_UpdateStatementsAreBatched`,
      `TestCalculateFileHash_BoundedAllocations` and
      `TestFastScanGalleryDirectory_StateEquivalence` all FAIL, and all PASS
      again once restored. A deliberate digest corruption (hashing half of each
      read chunk) makes `TestCalculateFileHash_Success` FAIL and PASS again when
      restored. `TestCalculateFileHash_StableUnderConcurrency` passes in both
      cases by design: it guards pool safety (inconsistent digests), not absolute
      correctness, which the golden-digest test covers.
- [x] 4.5 `openspec validate scan-sync-performance` reports "Change
      'scan-sync-performance' is valid"; the before/after report was presented.
      The focused-spec `run --scope current` for the five new scenarios reports
      5 PASS, 0 FAIL, 0 SKIP, 0 ERROR.
