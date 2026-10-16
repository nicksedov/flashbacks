# Proposal

## Why

The gallery folder scan → database sync path is the cost centre for large
galleries. With 100,000 files on a slow HDD, the current implementation:

- hashes every file with a freshly allocated hash state, a freshly allocated
  copy buffer, and two hex/allocation round-trips
  ([`CalculateFileHash`](../../../backend/core/internal/application/imaging/scanner.go:19)),
  none of it reused across files;
- flushes the database every 50 files
  ([`writeBatchSize = 50`](../../../backend/core/internal/application/imaging/scanner.go:232))
  and then performs **one `UPDATE` per changed row** inside a loop
  ([`flushBatch`](../../../backend/core/internal/application/imaging/scanner.go:103)),
  so a 100,000-file scan can emit on the order of 100,000 single-row statements
  plus thousands of extra `SELECT id IN (...)` round-trips;
- walks every gallery directory twice per scan (a metadata-only pre-count in
  [`countImageFiles`](../../../backend/core/internal/application/imaging/scan_manager.go:163)
  followed by the real scan walk in
  [`scanDirectory`](../../../backend/core/internal/application/imaging/scanner.go:126)).

There is **no benchmark** anywhere on this path, so none of this cost can be
measured, compared, or regressed today. The goal is to keep the exact observable
behaviour (identical digests, identical created/updated/deleted classification,
identical `/api/status` and `/api/gallery` output) while removing measurable CPU
and memory churn and collapsing the SQL statement count.

## What Changes

- Add a **measurement harness** for the scan path: `BenchmarkCalculateFileHash`
  and `BenchmarkScanDirectory`/`BenchmarkFlushBatch` with `b.ReportAllocs()`,
  plus a GORM statement counter so the database cost is measurable
  deterministically (SQLite wall-clock alone understates the Postgres win).
- Add **regression guards before optimising**: a golden-digest test for
  `CalculateFileHash`, and a full-sync state-equivalence test that pins the
  exact created/updated/deleted/unchanged result of a fixture tree.
- **P1 — batch the DB writes** in `flushBatch`: replace the per-row `db.Save`
  with a single batched upsert inside one transaction and raise the write batch
  size. The `ContentChanged` flag and the final row set stay identical.
- **P2 — reuse the hash read buffer** in `CalculateFileHash`: a pooled fixed-size
  buffer and a stack-allocated hex buffer, so hashing a file allocates a
  constant, small amount independent of file size and worker count. Digests are
  byte-identical.

No public API, response format, error semantics, or database schema change.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `gallery-browsing`: add requirements that the scan result classification is
  invariant under batching, that the file hash digest is stable under buffer
  reuse, that scan database writes are batched (statement count bounded by the
  batch size, not the file count), and that hashing memory is bounded.

## Impact

- **Core (Go)**: [`scanner.go`](../../../backend/core/internal/application/imaging/scanner.go:19),
  [`scanner_test.go`](../../../backend/core/internal/application/imaging/scanner_test.go:16),
  plus new benchmark files in `backend/core/internal/application/imaging/`.
- **No** API-contract change, **no** schema change, **no** new dependency
  (`sync.Pool` is stdlib), **no** webapp change.
- **Dependencies / schema**: none added.

## Non-goals

- **P3 — single directory walk.** Folding the metadata-only pre-count into the
  scan walk would change `totalFiles` reporting semantics and interleaves with
  the now-archived `gallery-folders-scan-progress` work; kept out of this change
  to avoid a behaviour/semantics decision.
- **P4 — progress strings.** The scan still emits free-text progress messages
  and derives `filesProcessed` by prefix matching. This is a correctness and
  design concern already partially addressed by the archived structured-status
  change, not a hot-path cost in scope here.
- **P5 — delete-sweep index strategy.** Validating `LIKE 'prefix%'` index use
  requires a real Postgres `EXPLAIN ANALYZE` and is not reproducible under the
  in-memory-SQLite test rule.
- **P6 — hash worker count.** The 4-worker pool on a slow HDD may thrash seeks;
  this is hardware-dependent and needs a controlled slow-device benchmark, so it
  is not tuned here.
- Changing any public API, response shape, error text, or scan trigger/scheduling
  behaviour.
