# Design

## Context

See [`proposal.md`](proposal.md) for motivation.

Current state that shapes the approach:

- [`CalculateFileHash`](../../../backend/core/internal/application/imaging/scanner.go:19)
  opens the file, creates `md5.New()`, and uses `io.Copy` (which allocates a
  32 KiB buffer per call when neither side specialises), then
  `hash.Sum(nil)` + `hex.EncodeToString` — several allocations per file, ×100,000
  files, none reused.
- [`flushBatch`](../../../backend/core/internal/application/imaging/scanner.go:72)
  batches the old-hash lookup (`SELECT id, hash WHERE id IN (...)`) but then
  calls `db.Save(&f)` **per row** inside a loop
  ([scanner.go:103](../../../backend/core/internal/application/imaging/scanner.go:103)).
- [`scanDirectory`](../../../backend/core/internal/application/imaging/scanner.go:114)
  and [`fastScanGalleryDirectory`](../../../backend/core/internal/application/imaging/scanner.go:280)
  flush via `writeBatchSize = 50`
  ([scanner.go:232](../../../backend/core/internal/application/imaging/scanner.go:232)).
- `imaging` coverage is 52.6% with 27 existing tests pinning scan semantics
  (`TestScanDirectory_NewFiles`, `_CachedFiles`, `_ModifiedFiles`,
  `TestFastScanGalleryDirectory_*`, `TestScanManager_*`), and **zero**
  benchmarks on the path.
- Unit tests run on in-memory SQLite
  ([`docs/agent/go-services.md`](../../../docs/agent/go-services.md)), which does
  not reproduce Postgres batching/transaction or index behaviour, so database
  wins are measured by **statement count** plus wall-clock, not wall-clock alone.

## Goals / Non-Goals

**Goals:**

- Identical observable behaviour (digests, row classification, `ContentChanged`
  flags, API output).
- A benchmark + statement-count harness so every change is justified by numbers.
- Collapse per-row writes into batched writes (P1) and bound hashing allocations
  with a pooled buffer (P2).

**Non-Goals:**

- Restructuring the directory walk or the `totalFiles` pre-count (P3).
- Replacing the free-text progress channel (P4).
- Delete-sweep index strategy (P5) and hash worker-count tuning (P6).
- Any API, schema, or error-semantics change.

## Decisions

### D1 — Measurement harness with a GORM statement counter

Add `BenchmarkCalculateFileHash` and `BenchmarkScanDirectory` (a synthetic tree
on a temporary directory, exercising the create/update/delete/unchanged paths)
with `b.ReportAllocs()`. To measure database work deterministically, attach a
counting `gorm.Logger` whose `Trace` callback increments per-operation counters
(`Query`, `Create`, `Update`, `Delete`, `Raw`); the benchmark asserts on the
**number of executed statements**, not just elapsed time.

- *Alternative considered:* wall-clock only. Rejected: SQLite timing understates
  the Postgres per-statement and per-transaction cost by an order of magnitude.

### D2 — P1: batched writes in `flushBatch`

Replace the per-row `db.Save(&f)` with one batched upsert (or one `Save` of the
whole `toUpdate` slice) inside a single transaction, and raise `writeBatchSize`
from 50 to a larger value (e.g. 500). The `ContentChanged` computation is
unchanged: it still compares each new hash against the old hash fetched by the
single `SELECT id, hash WHERE id IN (...)` already present
([scanner.go:93](../../../backend/core/internal/application/imaging/scanner.go:93)).
`ScanBatchResult.Created/Updated/Deleted` is preserved so post-processing events
are unchanged.

- *Alternative considered:* raw SQL multi-row upsert. Rejected unless GORM's
  batch `Save` proves insufficient; the GORM path keeps the code simpler and the
  behaviour visibly unchanged.
- *Deliberate, approved correction:* the boxed per-row `db.Save(&f)` also
  overwrote `created_at` with the zero time on every update (measured:
  `createdAt=0001-01-01`), silently discarding the record's creation timestamp.
  The batched upsert does not include `created_at` in its `DO UPDATE` set, so the
  value survives. This is the one intentional data change in an otherwise
  behaviour-preserving optimisation; it is specified by the
  `gallery-browsing.scan-preserves-created-at` scenario and pinned by
  `TestFastScanGalleryDirectory_StateEquivalence`.

### D3 — P2: pooled read buffer in `CalculateFileHash`

Use a `sync.Pool` of fixed-size `[]byte` (32 KiB) with `io.CopyBuffer`, keep
`md5.New()`, and write the digest into a stack-allocated 32-byte hex buffer
instead of `hex.EncodeToString(hash.Sum(nil))`. The returned digest is
byte-identical; only the transient allocations change.

- *Alternative considered:* a single package-level buffer guarded by a mutex.
  Rejected: `sync.Pool` scales with the worker count and is goroutine-safe by
  construction.
- *Trade-off:* one pooled buffer per hashing worker, comfortably within the 1 MB
  memory budget even at 4 workers (4 × 32 KiB = 128 KiB).

### D4 — Consciously out of scope (P3, P4, P5, P6)

Each is a behaviour or hardware-dependent change, not a mechanical cost removal:
P3 changes `totalFiles` semantics; P4 touches the progress channel already
reworked by the archived `gallery-folders-scan-progress` change; P5 needs real
Postgres `EXPLAIN ANALYZE`; P6 needs a controlled slow-device benchmark. They are
recorded as non-goals and left untouched.
