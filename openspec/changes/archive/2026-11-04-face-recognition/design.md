# Design

## Context

Flashbacks is a set of Go microservices over HTTP REST; `core` owns persistence
(GORM, Postgres + pgvector in production, in-memory SQLite in unit tests) and the
background jobs (`TagScanManager`, `EmbeddingBackfillManager`, `ScanManager`)
driven by the unified `schedules` table and the reusable `background.Manager`.
Services discover each other through the registry REST facade and never touch
another service's database.

The requested ML stack — SCRFD detection, ArcFace embeddings, HDBSCAN
clustering — has no mature, maintained pure-Go implementation. It lives natively
in the Python ecosystem: InsightFace ships SCRFD/ArcFace as ONNX models on ONNX
Runtime, and `hdbscan` (scikit-learn-contrib) is the reference clustering
implementation. See [`proposal.md`](proposal.md) for the why and what.

## Goals / Non-Goals

**Goals:**

- Person-centric browsing: list discovered people and browse one person's photos.
- A schedule-driven, resumable background face classification job that fits the
  existing `schedules` + `background.Manager` model.
- Keep the ML pipeline in its own service so `core` stays an orchestrator that
  owns persistence and API only.
- Reuse the existing design-system primitives and settings patterns in the UI.

**Non-Goals:**

- Identity resolution against an external directory; people stay unnamed
  clusters until the user renames them.
- GPU scheduling, video, real-time/webcam inference, or multi-tenant isolation.
- Manual face merge/split and rename are deliberately out of this change's
  scope (noted as follow-ups).

## Decisions

### D1 — `faces` is a new Python (FastAPI) microservice, not Go

The `faces` service runs the full ML pipeline natively in Python:

- **Detection**: SCRFD-10G (`det_10g.onnx`) — the speed/accuracy sweet spot
  (~90+% AP on WIDER FACE at a few ms/image on CPU, ~16 MB model).
- **Embedding**: ArcFace (iResNet-50, `w600k_r50.onnx`) — 512-d embeddings, the
  de facto open standard with strong LFW/CFP/agedb benchmarks.
- **Clustering**: HDBSCAN (cosine metric) over the batch of embeddings.

It exposes a small REST surface over `/faces`: `POST /faces/detect` (image →
boxes), `POST /faces/embed` (crop → vector), `POST /faces/cluster` (embeddings →
labels), plus `/faces/health`. `core` talks to it exactly like it talks to
`exif`/`ocr`: a typed HTTP client with context timeouts, resolved through the
registry with a bootstrap-URL fallback.

*Why Python over a Go service using ONNX Runtime Go bindings:* InsightFace +
`hdbscan` are the reference, maintained implementations of the exact algorithms
requested; hand-porting HDBSCAN or binding ONNX Runtime from Go adds significant
ML-maintenance risk for no user-visible gain. The cost — a second runtime in the
ops story — is bounded: the repo already ships Python tooling for the OCR
integration suite, and the deploy model is per-service Docker images.

*Alternative considered:* a Go `faces` service on `onnxruntime_go` with a
Go-native clustering fallback (DBSCAN/Chinese Whispers). Rejected as the
default because the clustering quality and maintenance burden are strictly
worse, but it remains a fallback if the team later mandates a Go-only service
stack.

### D2 — Detection, embedding, clustering are separate `faces` endpoints

Each ML step is a separate stateless endpoint so `core` can batch and checkpoint
independently: detect all unprocessed images first, embed only the detected
crops, then cluster all embeddings in one batch. This mirrors the OCR split and
keeps each call small, retryable, and idempotent. Detection and embedding are
GPU/CPU-neutral (ONNX Runtime); clustering is a pure CPU batch step.

### D3 — Clustering is batch, incremental, and idempotent

`core` accumulates embeddings for images not yet clustered and calls
`POST /faces/cluster` with the full new batch. HDBSCAN labels noise as `-1`
(unclustered). The job is resumable: a `faces`/`people` row records what was
processed, so a restart continues instead of recomputing. Re-clustering is a
re-run of the batch; stable embeddings make the result deterministic for a given
input set.

*Alternatives considered:*

- **UMAP + HDBSCAN**: higher clustering quality on high-dimensional data; adds
  latency and a reduction step. Noted as an opt-in quality upgrade, not the v1
  default (512-d ArcFace + cosine HDBSCAN is already strong).
- **Faiss IVF/HNSW + connected components**: far better for very large
  libraries (100k+ faces) where the O(n²) pairwise HDBSCAN step dominates.
  Documented as the scale upgrade path, with approximate nearest-neighbour
  pre-filtering feeding HDBSCAN.
- **Chinese Whispers / LPA**: O(n) graph clustering, fastest but lower
  precision. Rejected as default; useful only as a last-resort scale fallback.

### D4 — `core` owns persistence and orchestration

New domain models (GORM auto-migrated on boot, like the rest of the schema):

- `Person` (`people`): `id`, `name` (default `""`, display falls back to a
  localized "Person N"), `coverFaceID`, `createdAt`, `updatedAt`.
- `Face` (`faces`): `id`, `imageFileID`, `personID`, bounding box
  (`x,y,width,height`), `confidence`, `embeddingID`, `createdAt`, `updatedAt`.
- `FaceEmbedding` (`face_embeddings`): `id`, `faceID`, `dimension`, `embedding`
  (`halfvec`), plus a `model` tag identifying ArcFace so future model swaps are
  distinguishable.

JSON tags are camelCase and will be mirrored one-to-one in the webapp TS types.

A new `FaceRecognitionManager` in `internal/application/faces` follows the
`TagScanManager` pattern: `background.Manager` + a `ScheduleKind
face_recognition` row in the unified `schedules` table (default: enabled,
weekdays 22:00–07:00 window, mirroring tag scan). It fetches unprocessed image
files, calls `faces` for detection/embedding, persists faces/embeddings, then
clusters and assigns `personID`.

### D5 — API and contracts

New `core` endpoints (all session-auth, CSRF on mutations):

- `GET /api/people` — person cards (`id`, `name`, `photoCount`, cover
  thumbnail URL).
- `GET /api/people/:id/photos` — paginated photos of a person (same pagination
  metadata shape as `GET /api/gallery`).
- `GET /api/faces/status` — job running state + `processed`/`total` counters.
- `POST /api/faces/start`, `POST /api/faces/stop` — launch/stop the job.
- Face recognition settings ride the existing settings DTO (an `enabled` flag
  and schedule fields next to tag scan), persisted via the same update path.

Contracts: extend [`docs/api-contracts/core.yaml`](../../../docs/api-contracts/core.yaml)
and add [`docs/api-contracts/faces.yaml`](../../../docs/api-contracts/faces.yaml);
run `make generate-types` to refresh `webapp/src/types/api.ts`.

### D6 — UI

- New lazy-loaded `PeopleTab` (grid of person cards using the shared
  `Tile`/`EmptyState`/`Skeleton` primitives), and a person detail view that
  reuses the gallery grid/lightbox for the person's photos.
- A face recognition settings section in `AdminAnalysisTab` reusing the shared
  `ScheduleEditor` and the tag-scan status badge pattern (enable toggle,
  schedule, Start/Stop, progress).
- All strings added to `translations.en.ts` and `translations.ru.ts` together.

### D7 — Wire DI and i18n

`core`'s Wire graph gains the `FaceRecognitionManager`, the `faces` REST client,
and the face repositories — `wire.go` and the regenerated `wire_gen.go` are both
committed. i18n `Msg*` constants and en/ru locale entries ship together, and the
new service follows the registry client's self-healing registration rules.

## Evidence plan

The delta scenarios carry `focused-spec:id` markers. At apply time their
`planned:` entries are bound in [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml)
under `scopes.current`:

- `face-recognition.face-assigned`, `same-person-grouped`, `no-face-skipped`,
  `status-progress`, `people-listed`, `person-photos`, `settings-persisted`,
  `job-started` → `go::` selectors against `backend/core` tests (a mock `faces`
  client supplies deterministic detections/embeddings/cluster labels).
- `face-recognition.people-grid`, `people-empty`, `people-localized` →
  `vitest::` selectors against `PeopleTab` tests.

Then `focused-spec validate --scope current --strict` and `run --scope current`
are executed, and every new/changed evidence entry is mutation-checked.

## Risks / Trade-offs

- **Model licensing** → Verify SCRFD (MIT) and ArcFace/InsightFace licenses and
  add the `faces` service and its models to the existing third-party-components
  list before shipping.
- **CPU inference throughput** on large libraries → Batch processing with
  checkpoints, configurable concurrency, and the documented Faiss/GPU upgrade
  path; the job runs in an overnight window like tag scan.
- **O(n²) HDBSCAN memory on large batches** → Clustering runs on incremental
  batches sized by a configurable `clusterBatchSize`; approximate nearest
  neighbours is the scale path.
- **False merges/splits of lookalikes** → Confidence thresholding on detection,
  `min_cluster_size` on HDBSCAN, and keeping the raw embeddings so re-clustering
  is cheap; manual merge/split is a follow-up.
- **New Python runtime in ops** → Dedicated `backend/faces/Dockerfile`, its own
  deploy asset, and a README; the service degrades gracefully when `core`
  reaches it (timeouts + structured errors), so a missing `faces` service never
  breaks gallery browsing.
- **Person rows orphaned by re-clustering** → Re-clustering upserts by face ID;
  empty people are hidden from the listing, never force-deleted.

## Migration Plan

1. Ship the `faces` service image and registry registration first (no `core`
   dependency yet); verify `/faces/health`.
2. Deploy `core` with the new models, job, endpoints, and Wire changes; the job
   is additive and starts disabled until an administrator enables it.
3. Deploy the webapp (People tab + settings); regenerate TS types.
4. Rollback: remove the `faces` service and revert `core`/webapp — the feature
   is additive (new tables + new endpoints), so no destructive migration exists;
   `people`/`faces`/`face_embeddings` tables are simply unused if rolled back.
