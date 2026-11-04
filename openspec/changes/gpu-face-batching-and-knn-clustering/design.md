# Design

## Context

See [`proposal.md`](proposal.md) for motivation and
[`specs/face-analysis/spec.md`](specs/face-analysis/spec.md) /
[`specs/face-clustering/spec.md`](specs/face-clustering/spec.md) for the
requirements this design satisfies.

Current constraints that shape the approach:

- `backend/faces/app/engine.py` hard-codes `providers=["CPUExecutionProvider"]`
  and calls `app.prepare(ctx_id=0, det_size=(640, 640))`. In InsightFace
  `ctx_id >= 0` selects a **GPU device** and `ctx_id < 0` selects CPU, so the
  current comment ("ctx_id=0 selects CPU") is wrong — provider choice and
  `ctx_id` must be derived from one decision.
- Small faces (< 64 px wide) are re-embedded by calling `analysis.get()` on an
  upscaled crop — one full detection pass **per small face**; the 27-face G7 photo
  pays ~12 s for the frame plus one pass per small face.
- `backend/core/internal/application/faces/manager.go` sends one image per HTTP
  request (120 s timeout per image) and `clusterAssign` reads **every** unassigned
  embedding and posts the matrix to `/faces/cluster`.
- `face_embeddings.embedding` is already `halfvec`; `infrastructure/database/embedding_tables.go`
  already creates an HNSW index for tag embeddings
  (`USING hnsw (embedding halfvec_cosine_ops)`), so the pattern exists in core.
- Core unit tests run on in-memory SQLite — pgvector SQL cannot be assumed there.

## Goals / Non-Goals

**Goals:**

- Full request → provider → device selection chain configurable per environment,
  with a guaranteed CPU path on any host and no behavior change when nothing is
  configured.
- One faces-service request carries a batch of images; the recognition model sees
  up to N crops per forward pass so a GPU is kept busy.
- Clustering reads a bounded candidate neighbourhood from PostgreSQL when
  available, while every similarity threshold stays in the faces service.
- Deploy stacks may reserve a GPU for the faces container without disturbing the
  running ollama containers.

**Non-Goals:**

- No new model weights, no threshold re-calibration.
- No per-insert HNSW maintenance, no online index rebuild scheduling UI.
- No face-search endpoint or webapp surface.
- No change to `/faces/analyze`, `/faces/detect`, `/faces/embed` request/response
  shapes (only additive health fields).

## Decisions

### D1 — Provider resolution by the service, from an ordered env list

`FACES_PROVIDERS` (default `cpu`) is a comma-separated, ordered list accepting
shorthands (`cpu`, `cuda`) and full ONNX Runtime names
(`CPUExecutionProvider`, `CUDAExecutionProvider`, `TensorrtExecutionProvider`).
At engine load the service intersects the request with
`onnxruntime.get_available_providers()`:

- empty result after the intersection → fall back to `["CPUExecutionProvider"]`
  and log a warning that names both lists;
- resolved list starts with a CUDA provider → `ctx_id = FACES_GPU_ID` (default
  `0`); otherwise `ctx_id = -1` (CPU).

Device choice is stored as a small `ProviderSelection` value
(`active`, `requested`, `available`, `device_id`) exposed through
`/faces/health` as `activeProvider` plus the service thresholds (D5).

*Alternatives:* auto-prefer CUDA whenever present (rejected — ollama already owns
VRAM on this host; accelerating faces must be an operator decision);
fail fast when CUDA is requested but missing (rejected — a CPU-only staging host
would become undeployable).

### D2 — `/faces/analyze-batch` with per-image isolation

`POST /faces/analyze-batch` takes `{"images": ["<base64>", ...]}`, capped at
`FACES_MAX_BATCH` (default 16, enforced by the request model's `max_length` → 422)
and returns `{"results": [{"faces": [...]}, {"error": "..."}, ...]}` — one entry
per submitted image, positionally aligned so the caller never needs an ID map. An
image that fails to decode produces an `error` entry; the rest of the batch is
still analysed. `/faces/analyze` remains and delegates to the same code path with
a single image.

A module-level `threading.Semaphore(FACES_MAX_CONCURRENCY)` (default 1) serialises
batch execution: two concurrent GPU batches would double VRAM and gain nothing on
one device.

### D3 — Batch the recognition forward pass, not just the request

Batch mode collects **aligned crops** per image instead of calling
`analysis.get()` per image:

1. one `analysis.get(img)` per image for detection + the large faces' embeddings
   (unchanged, and the dominant cost);
2. small faces (< `SMALL_FACE_WIDTH`) are aligned with
   `insightface.utils.face_align.norm_crop(face.kps)` from the padded upscaled
   crop and appended to a batch;
3. the batch is chunked by `FACES_RECOGNITION_BATCH` (default 32) and each chunk
   goes through `app.models["recognition"].get_feat(crops)` once, then
   `l2_normalize`, reproducing what `get()` does internally.

If `FACES_RECOGNITION_BATCH == 1`, or the crop batch is empty, the existing
per-image `get()` path runs unchanged — so a CPU deployment keeps today's exact
numerics. Parity is guarded by the existing dataset integration suite: embeddings
from the batched path must match the per-image path at cosine ≥ 0.99, and the
recognition/clustering scenarios keep their thresholds.

*Alternatives:* rely on `/faces/analyze` concurrency only (rejected — InsightFace
has no multi-image `get()`, so the small-face re-embedding stays serial and the
GPU stays idle between calls); raise `det_size` (rejected — accuracy is already
calibrated, cost would grow).

### D4 — Core batches images, keeps failures retryable

`facesclient.Client` gains `AnalyzeBatch(ctx, images [][]byte) ([]BatchResult, error)`
with `BatchResult{Faces []AnalyzedFace; Error string}`. `Manager.runPass` collects
up to `FACES_BATCH_SIZE` (default 8) images from its existing cursor query and
sends one request:

- entry with `Faces` → `replaceFaces` + `markScanned` for that image;
- entry with `Error` → `setError` and **no** mark, so the next pass retries it;
- request-level failure → the whole batch stays unmarked (spec
  `face-analysis.failed-batch-retried`).

Timeout moves to `FACES_ANALYZE_TIMEOUT` (default `300s`) per batch request.
`FACES_BATCH_SIZE=1` degenerates to today's behavior.

### D5 — Clustering reads candidates from the store; thresholds stay in the service

New application port (`internal/application/faces`):

```go
type NeighbourSource interface {
    // Supported reports whether the store can answer kNN queries.
    Supported(ctx context.Context) bool
    // Neighbours returns, for each seed face, its nearest faces at or above
    // minSimilarity, capped at limit per seed.
    Neighbours(ctx context.Context, seeds []uint, minSimilarity float64, limit int) (map[uint][]Neighbour, error)
}
```

PostgreSQL implementation (`internal/infrastructure/database`), one query per
seed batch, letting the HNSW index answer each lateral probe:

```sql
SELECT src.face_id, n.face_id AS candidate_id, n.similarity
FROM unnest($1::bigint[]) AS src(face_id)
JOIN face_embeddings seed ON seed.face_id = src.face_id
CROSS JOIN LATERAL (
    SELECT e.face_id, 1 - (e.embedding <=> seed.embedding) AS similarity
    FROM face_embeddings e
    WHERE e.face_id <> seed.face_id
      AND 1 - (e.embedding <=> seed.embedding) >= $2
    ORDER BY e.embedding <=> seed.embedding
    LIMIT $3
) AS n
```

`minSimilarity` is **not** a core config value: core reads
`thresholds.recognitionMinSimilarity` from `/faces/health` (fallback `0.25` when
the field is missing, i.e. an older service) and `limit` is
`FACES_CLUSTER_NEIGHBOURS` (default 16). The faces service receives
`edges: [{faceId, candidateId, similarity}]` at `POST /faces/cluster-graph` and
applies the strict floor (0.40), the recognition floor (0.25) and the runner-up
margin (0.05) exactly as it does today, returning one label per face.

`FACES_CLUSTER_MODE` (`auto` | `store` | `memory`, default `auto`) selects the
strategy: `auto` uses the store when `Supported()` is true, `memory` forces the
existing all-pairs path (rollback lever and SQLite tests).

*Alternatives:* decide links in core (rejected — duplicates the calibrated
thresholds in Go and makes the service a dumb vectorizer); recursive CTE
union-find in SQL (rejected for now — the "only a portrait may anchor a new
person" and runner-up-margin rules are graph-shaped logic that reads far better in
the service, and the service already owns them).

### D6 — HNSW index built between the scan and clustering

`EnsureFaceEmbeddingIndex(db)` mirrors the existing tag-embedding helper:
`CREATE INDEX IF NOT EXISTS idx_face_embeddings_hnsw ON face_embeddings USING hnsw (embedding halfvec_cosine_ops)`.
It is an application port (`IndexMaintainer.EnsureNeighbourIndex`) called once in
`runPass` **after** the analysis loop and **before** `clusterAssign` — inserts
during the scan therefore hit a heap-only table. On SQLite the implementation is a
no-op that reports `false`, which is exactly what `NeighbourSource.Supported()`
reports, so the two stay consistent.

`Supported()` = dialect is `postgres` **and** `SELECT extversion FROM pg_extension WHERE extname = 'vector'`
parses to ≥ 0.7 (`halfvec` support).

### D7 — Deploy surface

- **Image**: `backend/faces/Dockerfile` becomes two targets — `cpu` (today's
  `python:3.11-slim` + `onnxruntime`, the default) and `gpu` (CUDA 12/cuDNN 9
  base from the local cache, pinned as `ARG FACES_CUDA_BASE`, + `onnxruntime-gpu`).
  The apply step resolves the base tag with
  `docker images | grep -Ei 'cuda|cudnn|code-server'` and records it in the
  Dockerfile `ARG` default and the README. Both targets keep `USER app` and a
  `HEALTHCHECK` (start period raised for CUDA init on the GPU target).
- **Compose (staging + production, `.roo/skills/deploy-*/deploy/docker-compose.yml`)**:
  faces service block gains
  `deploy.resources.reservations.devices: [{driver: nvidia, device_ids: ["${FACES_GPU_ID:-0}"], capabilities: [gpu]}]`,
  `NVIDIA_DRIVER_CAPABILITIES: compute,utility`, `FACES_PROVIDERS`, and keeps the
  `faces_models` volume. The same change is mirrored in both stacks.
- **Env/seed**: `FACES_PROVIDERS`, `FACES_GPU_ID`, `FACES_MAX_BATCH`,
  `FACES_BATCH_SIZE`, `FACES_RECOGNITION_BATCH`, `FACES_CLUSTER_MODE` documented
  in `backend/faces/README.md`, `backend/core/.env.example`, both
  `flashbacks-*.env`/`.seed.env` files and `scripts/dev.sh` exports.
- **Scripts**: the per-service build spec in both deploy scripts learns the new
  target (`--target gpu --build-arg FACES_CUDA_BASE=...`) and a
  `FACES_IMAGE_ACCEL`/relabel switch so a CPU-only host still deploys.

### D8 — Wire and contract updates

`ProvideFaceRecognitionManager` gains the `NeighbourSource` and `IndexMaintainer`
dependencies → `wire.go` and `wire_gen.go` are regenerated with
`go generate ./...` in the same change. `docs/api-contracts/faces.yaml` gains
`/faces/analyze-batch`, `/faces/cluster-graph`, and the extended health schema;
`faces` is not consumed by the webapp, so no `make generate-types` run is needed
(verify by grep for `faces` in `webapp/src/types`).

## Risks / Trade-offs

- **CUDA/cuDNN mismatch at build or run time** (onnxruntime-gpu needs cuDNN 9 with
  CUDA 12; a cached base may carry an older cuDNN) → pin both the base tag and
  `onnxruntime-gpu`, and log `get_available_providers()` at startup so a wrong
  pairing shows up as a warning + `cpu` rather than a crash; the compose deploy
  is verified by reading `/faces/health.activeProvider` after rollout.
- **No compile-free guarantee that GPU is faster on the target host** → the
  change proves the *path* is live and batched (health + integration suite +
  documented benchmark command); the 10–30× figure is measured by the operator
  running `make test-faces-integration` before/after on the GPU host.
- **kNN cap can hide a true match** (16 nearest may all be other people) →
  default `FACES_CLUSTER_NEIGHBOURS=16`, floor filter in SQL, and a parity test on
  the dataset asserting identical labels to the in-memory path.
- **Re-implemented crop alignment drifts from InsightFace** → parity gate at
  cosine ≥ 0.99 on the dataset; `FACES_RECOGNITION_BATCH=1` restores the old path
  without a rebuild.
- **VRAM contention with ollama** → single batch concurrency (semaphore), cap on
  `FACES_BATCH_SIZE`/`FACES_MAX_BATCH`, and faces only runs inside its schedule
  window; the operator can pin a device via `FACES_GPU_ID`.
- **HNSW build time on a large gallery** delays the first `clusterAssign` → the
  build is idempotent (`IF NOT EXISTS`) and runs once; later passes skip it.
- **Postgres-only SQL is invisible to unit tests** → the port + fake keeps the
  manager logic on SQLite; the real query is covered by a Postgres-gated test that
  skips when `FACES_TEST_PG_DSN` is unset, and the labels parity test runs on the
  dataset.

## Migration Plan

1. Ship with defaults that change nothing: `FACES_PROVIDERS=cpu`,
   `FACES_BATCH_SIZE=1`, `FACES_CLUSTER_MODE=auto` on a host without pgvector ≥ 0.7
   still takes the memory path.
2. Stage: build the `gpu` target, set `FACES_PROVIDERS=cuda,cpu` and
   `FACES_BATCH_SIZE=8`, confirm `/faces/health` reports `cuda`, then run the
   recognition integration suite on the dataset.
3. Production: deploy the GPU image to the faces service only; the index is
   created on the first pass (`CREATE INDEX IF NOT EXISTS`).
4. Rollback: `FACES_PROVIDERS=cpu`, `FACES_BATCH_SIZE=1`, `FACES_CLUSTER_MODE=memory`
   and redeploy the `cpu` image target — no schema change is reverted because the
   HNSW index is additive.

## Open Questions

- Exact `FACES_BATCH_SIZE`/`FACES_RECOGNITION_BATCH` for the RTX 3090 vs A4000
  split is measured on the host; the defaults are conservative starting points,
  not contract values.
