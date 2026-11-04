# faces — Flashbacks face recognition service

Runs the Flashbacks face recognition ML pipeline over HTTP:

- **Detection** — SCRFD-10G (`det_10g.onnx`) via InsightFace.
- **Embedding** — ArcFace (`w600k_r50.onnx`), 512-dimensional vectors.
- **Clustering** — DBSCAN over cosine distance with an identity threshold
  (`CLUSTER_MIN_SIMILARITY`, 0.40): faces below it never share a person, and a
  face seen once stays unassigned noise (`-1`). A person is formed from at
  least `minClusterSize` faces (default `2`), so an identity photographed twice
  still becomes a person.
- **Recognition of leftovers** — a small face in a group photo never reaches the
  strict floor, so faces the DBSCAN pass left as noise are linked at the relaxed
  `RECOGNITION_MIN_SIMILARITY` (0.25) when they beat the runner-up group by
  `RECOGNITION_MARGIN` (0.05). Only a portrait (the sole face of its photo,
  flagged by `soloFaces`) may anchor a new person; group-shot faces can only
  join a person that already exists.
- **Small faces** — a face narrower than `SMALL_FACE_WIDTH` (64 px) is embedded
  again from a padded crop upscaled so the face is about
  `UPSCALE_TARGET_WIDTH` (160 px) wide. Measured on the G7 group photo: a 21 px
  wide face scores 0.22 against its portrait from the full frame and 0.29 from
  the upscaled crop, while the highest cosine between two different people in
  the calibration set is 0.15.

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/faces/health` | Health probe: `status`, `activeProvider`, `requestedProviders` and the calibrated `thresholds`. |
| POST | `/faces/analyze` | `{"image": "<base64>"}` → `{"faces": [{x,y,width,height,confidence,embedding}]}` |
| POST | `/faces/analyze-batch` | `{"images": ["<base64>", ...]}` → `{"results": [{"faces": [...]} \| {"error": "..."}]}`, one entry per image in request order. |
| POST | `/faces/detect` | `{"image": "<base64>"}` → `{"boxes": [{x,y,width,height,confidence}]}` |
| POST | `/faces/embed` | `{"image": "<base64>", "box": {...}?}` → `{"embedding": [512 floats]}` |
| POST | `/faces/cluster` | `{"embeddings": [[...], ...], "minClusterSize": 2}` → `{"labels": [int]}` |
| POST | `/faces/cluster-graph` | `{"faceIds": [...], "soloFaces": [...], "edges": [{faceId,candidateId,similarity}]}` → `{"labels": [int]}`, one label per face id. |

`/faces/health` never loads a model, so it is cheap enough to be the container
health probe and still answers "is the GPU path live?". `activeProvider` is the
short name (`cpu`, `cuda`, `tensorrt`) of the provider the model session actually
uses, and `thresholds` reports the calibrated similarities so callers do not
have to keep their own copies.

## Execution providers

`FACES_PROVIDERS` is an ordered, comma-separated list of ONNX Runtime providers
accepting both shorthands (`cpu`, `cuda`, `tensorrt`, `openvino`, `directml`,
`coreml`) and full names (`CPUExecutionProvider`, `CUDAExecutionProvider`, ...).
It defaults to `cpu`: accelerating a host is an operator decision, because the
ollama containers already own most of the VRAM.

The request is intersected with `onnxruntime.get_available_providers()` at the
first inference or health call:

- a provider that is not offered is dropped and a warning naming both lists is
  logged, so `cuda,cpu` on a CPU-only host runs on `cpu` instead of failing;
- when nothing requested is available the service falls back to
  `CPUExecutionProvider` with a warning;
- `ctx_id` is derived from the resolved list — a CUDA provider first passes
  `FACES_GPU_ID` (default `0`) as the device, anything else passes `-1` (CPU).

The same decision is logged at startup (`execution providers: requested=...
available=... active=... ctx_id=...`), so a CUDA/cuDNN mismatch shows up as a
warning plus `cpu` rather than a crash.

`/faces/analyze-batch` is the call the recognition job uses: one request covers
several photos, and small-face recognition crops are collected and sent through
the recognizer in chunks of `FACES_RECOGNITION_BATCH` (see below), so one GPU
submission covers many faces. Requests over `FACES_MAX_BATCH` are rejected with
`422` before any model work, and an image that cannot be decoded produces an
`error` entry instead of failing its whole batch. `/faces/analyze` remains the
single-image call and runs the same code path.

Asking `/faces/embed` once per face instead costs a full detection pass each — a
27-face group photo needed ~180 s of CPU and its later faces were dropped by the
caller's request timeout.

`/faces/cluster` groups embeddings the service received in full. When the vector
store can answer nearest-neighbour queries, core instead posts the candidate
similarity graph to `/faces/cluster-graph`: only the links inside the clustering
radius travel, while the strict floor (0.40), the recognition floor (0.25) and
the runner-up margin (0.05) stay in this service.

`/faces/embed` always detects faces on the **whole** image — SCRFD needs the
context around a face and finds nothing in a tight crop of a detection box. The
optional `box` only *selects* which detected face to embed: the one with the
highest IoU against the box. Without a box the largest detected face is used.
An empty `embedding` means no face was detected (in the image, or inside the
requested box).

Detection and embedding models are downloaded by InsightFace on first use
(`~/.insightface`) and loaded lazily. Clustering is pure CPU.

## Local run

```bash
cd backend/faces
python -m venv .venv && . .venv/bin/activate
pip install -e ".[dev]"
uvicorn app.main:app --host 0.0.0.0 --port 5173
```

## Tests

```bash
cd backend/faces
python -m pytest
```

The unit tests exercise the endpoint plumbing with a deterministic fake engine and
the clustering with the real DBSCAN implementation over synthetic embeddings, so
they run without downloading the ML models.

Recognition over the shared photo dataset ([`tests/dataset/faces`](../../tests/dataset/faces/README.md))
is covered by the HTTP integration suite in
[`tests/faces/integration`](../../tests/faces/integration): it runs the real
models inside this service and compares the returned embeddings (same person vs.
impostor, person inside group photos, clustering into people). It needs the
models downloaded and a running service:

```bash
make test-faces-integration
```

See [`tests/faces/README.md`](../../tests/faces/README.md) for the covered
scenarios and the cosine thresholds.

## Configuration

| Env | Default | Purpose |
|---|---|---|
| `FACES_PORT` | `5173` | HTTP listen port. |
| `FACES_PROVIDERS` | `cpu` | Ordered ONNX Runtime providers (shorthands or full names); unavailable ones degrade to CPU with a warning. |
| `FACES_GPU_ID` | `0` | CUDA device id passed as InsightFace `ctx_id` when a CUDA provider is active. |
| `FACES_MAX_BATCH` | `16` | Maximum images one `/faces/analyze-batch` request may carry (larger requests are rejected with `422`). |
| `FACES_RECOGNITION_BATCH` | `32` | Aligned small-face crops sent through the recognizer per forward pass; `1` keeps the per-image path. |
| `FACES_MAX_CONCURRENCY` | `1` | Concurrent batch analyses; one GPU is kept busy by one batch, so more only duplicates VRAM use. |
| `FACES_CLUSTER_MODE` | `auto` | Read by **core**, not this service: clustering strategy (`auto`, `store`, `memory`) for the recognition job's candidate graph. |
| `FACES_CLUSTER_NEIGHBOURS` | `16` | Read by **core**, not this service: candidates per unassigned face when it queries the store for neighbours. |
| `REGISTRY_URL` | (empty) | Registry facade URL; empty disables registration. |
| `SERVICE_HOST` | `localhost` | Host peers use to reach this service. |
| `SERVICE_ID` | `<host>-<pid>` | Unique instance id. |
| `SERVICE_VERSION` | `dev` | Advertised version. |
| `REGISTRY_TOKEN` | (empty) | Bearer token for the registry facade. |

Registration is self-healing: the service retries with backoff until the registry
answers, heartbeats the lease, and re-registers after repeated heartbeat failures.
