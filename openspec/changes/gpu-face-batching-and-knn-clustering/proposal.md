# Proposal

## Why

Face recognition is now correct but slow and memory-bound: the faces service runs
InsightFace on `CPUExecutionProvider` only (a 1280 px frame takes ≈6.8 s, the
27-face G7 photo ≈12 s), and `clusterAssign` ships every unassigned embedding to
the service so Python builds an all-pairs matrix — the cost grows quadratically
with the gallery and the whole vector set has to fit in one request.

The host already owns two GPUs (the `ollama-rtx3090` / `ollama-a4000`
containers), so the inference path can move to CUDA without new hardware, and
`face_embeddings.embedding` is already `halfvec(512)` in PostgreSQL, so the same
rows can answer nearest-neighbour queries instead of being re-read into Python.

## What Changes

- **Execution provider is configuration.** `FACES_PROVIDERS` (default `cpu`)
  selects an ordered ONNX Runtime provider list; a provider that is requested but
  unavailable (no CUDA runtime, wrong onnxruntime build) SHALL fall back to CPU
  with a warning instead of failing the request. The active provider is reported
  by `/faces/health`.
- **Batch analysis endpoint.** `POST /faces/analyze-batch` accepts N images and
  returns per-image face lists, so one GPU submission covers many photos. The
  recognition job sends images in batches of `FACES_BATCH_SIZE` instead of one
  request per photo; `POST /faces/analyze` stays as the single-image call.
- **GPU image variant.** `backend/faces/Dockerfile` gains a GPU build stage on a
  CUDA 12 / cuDNN 9 base with `onnxruntime-gpu`; the default stage stays CPU-only
  on `python:3.11-slim`. Deploy stacks reserve the GPU device
  (`deploy.resources.reservations.devices` + NVIDIA container toolkit) and cap
  VRAM use so ollama keeps its share.
- **Clustering decides on a kNN candidate graph.** When the store supports it,
  core asks PostgreSQL/pgvector for each face's neighbourhood inside the
  clustering radius (`1 - (a <=> b) >= 0.40`) and sends the service candidate
  links instead of the full matrix; the service keeps owning every threshold
  (0.40 strict, 0.25 recognition, 0.05 runner-up margin) and turns the graph into
  labels. When the store cannot answer (SQLite unit tests, pgvector < 0.7) the
  job keeps the in-memory all-pairs path, so behavior stays available everywhere.
- **HNSW index for face embeddings.** `face_embeddings(embedding halfvec_cosine_ops)`
  gets an HNSW index created after bulk scanning rather than on every insert, so
  writes during a scan stay fast.
- OpenAPI contract [`docs/api-contracts/faces.yaml`](../../../docs/api-contracts/faces.yaml:1)
  gains `/faces/analyze-batch`, the extended `/faces/health` payload, and the
  candidate-graph variant of `/faces/cluster`.

## Capabilities

### New Capabilities

- `face-analysis`: how the faces service executes its models — which ONNX Runtime
  execution provider is used, how an unavailable provider degrades to CPU, and
  how many images one analysis request carries.
- `face-clustering`: how detected faces become people when the gallery no longer
  fits in one request — candidate neighbourhood selection, threshold ownership,
  and the maintained fallback path.

### Modified Capabilities

None. The end-to-end guarantees of `face-recognition` (a detected face references
exactly one person, one person per identity, status counters) are unchanged by
this work — batching and candidate-graph clustering change how the job reaches
them, not what it promises. Those existing scenarios stay the regression gate.

## Non-goals

- No change to the recognition models (`buffalo_l`: SCRFD-10G + ArcFace w600k_r50)
  and no re-tuning of the calibrated similarity thresholds.
- No face *search* UI ("find photos of this face") — the kNN index is prepared for
  it, but no endpoint or webapp surface is added here.
- No multi-GPU scheduling, no shared GPU pool with ollama beyond per-container
  device reservation and a VRAM/concurrency cap.
- No change to People view, person naming, or any webapp component.
- No new message broker or job queue: batching stays inside the existing pass loop.
- No online index maintenance service: the HNSW index is (re)built as a scan
  post-step, not per insert.
