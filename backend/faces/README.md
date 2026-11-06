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
- **Size criteria** — the size of a face is its smaller box side in stored
  pixels (`widths` on the cluster requests), which is orientation-invariant
  because boxes live in the un-rotated frame. Three roles follow:

  | role | size | behaviour |
  |---|---|---|
  | `tooSmall` | `< MIN_CLUSTERABLE_FACE_PX` (32) | never labelled: stays a detection, is never assigned a person and never founds one |
  | `weak` | 32…63 | may join a person a full-size face founded; never a core point, an anchor, an exemplar or a cover |
  | `full` | `>= SMALL_FACE_WIDTH` (64) | full participant |

  Measured on the production gallery (52 407 faces): 15-21 % of the faces below
  32 px do not even reach `RECOGNITION_MIN_SIMILARITY` against their own
  person's full-size faces, while the 32-63 px band stays at 4-9 %. The floors
  are advertised in `/faces/health` (`minClusterableFacePx`, `weakFacePx`) and
  core reads them: it skips such faces when clustering and detaches the ones
  attached by an older criteria set.

## Endpoints

| Method | Path | Description |
|---|---|---|
| GET | `/faces/health` | Health probe: `status`, `activeProvider`, `requestedProviders`, the session `devices` and the calibrated `thresholds`. |
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

## Clustering

Both clustering endpoints group faces by absolute cosine similarity (ArcFace
embeddings are calibrated for that): the strict floor is `CLUSTER_MIN_SIMILARITY`
(0.40), a leftover face joins a group at `RECOGNITION_MIN_SIMILARITY` (0.25) only
when it beats the runner-up by `RECOGNITION_MARGIN` (0.05), and only portraits
(the sole face of their photo) may recognise or anchor a person.

Because the graph is built by single linkage, a gallery with many photos of the
same few people chains into one group: measured on a 39,024-face library, the
strict pass merges 32,442 faces into a single "person". A group above
`CLUSTER_MAX_SHARE` (10% of the submitted faces, never below 250 faces) is
therefore re-clustered on its own at a stricter floor (`+0.05` per round, up to
`0.90`), and that re-split follows **mutual** links only — a one-way candidate
link is what a lookalike bridge looks like. On the same library this yields
2,837 people, 60% of the faces in a person, and the largest person at 8.4% of
the gallery.

### Clustering revisions

`FACES_CLUSTER_ALGO` selects the labelling revision:

- `graph-v1` (default) — the behaviour above.
- `graph-v2` — density-aware labelling: mutual, hub-corrected (CSLS) links; a
  core point needs `CLUSTER_CORE_NEIGHBOURS` mutual neighbours at
  `CLUSTER_CORE_SIMILARITY`; a candidate list keeps at most
  `CLUSTER_CANDIDATES_PER_PHOTO` faces per source photo; a person spans at least
  `CLUSTER_MIN_PHOTOS` photos; and after the cap-driven split, components merge
  when their medoid similarity clears `CLUSTER_MERGE_SIMILARITY` with
  `CLUSTER_MERGE_SUPPORT` mutual cross-component pairs at
  `CLUSTER_MERGE_SUPPORT_SIMILARITY`.

`/faces/health` reports the effective thresholds. The clustering requests
accept optional arrays aligned with the face ids — `photoIds` (used by
`graph-v2`; absent means one unique photo per face), `widths` and `qualities`
(accepted for quality-aware rules). That keeps an older core working unchanged.

## Execution providers

`FACES_PROVIDERS` is an ordered, comma-separated list of ONNX Runtime providers
accepting both shorthands (`cpu`, `cuda`, `tensorrt`, `openvino`, `directml`,
`coreml`) and full names (`CPUExecutionProvider`, `CUDAExecutionProvider`, ...).
It defaults to `cpu`: accelerating a host is an operator decision, because the
ollama containers already own most of the VRAM.

`FACES_GPU_IDS` lists the CUDA devices the service keeps a session on
(`FACES_GPU_IDS=0,1` on a two-card host). Every device owns one `FaceAnalysis`
session and one slot in the request pool, so concurrent analysis requests run on
different cards instead of queueing behind a single session. `FACES_GPU_ID`
(single device) remains supported when `FACES_GPU_IDS` is unset. All sessions
resolve the same provider list; each gets its own `ctx_id`.

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

## Verification calibration (LFW)

[`tools/lfw_calibration.py`](tools/lfw_calibration.py) measures 1:1 face
verification on the Labelled Faces in the Wild benchmark with the engine the
service itself runs — detection, alignment and ArcFace, including the small-face
refinement — so a threshold, model or algorithm change can be judged against a
public, reproducible number instead of a private fixture.

The protocol is the official one: 6,000 pairs in 10 folds, 300 same-person and
300 different-person pairs per fold. A pair is a match when the cosine
similarity of its two embeddings is at least the scored threshold; accuracy per
fold is the share of its 600 pairs decided correctly, and the reported accuracy
is the unweighted mean of the ten folds. When either image of a pair yields no
detectable face the pair is scored as a non-match instead of being dropped, so
detector failures cannot inflate the number.

The dataset is downloaded from Kaggle — `atulanandjha/lfwpeople`, which carries
both the official `pairs.txt` and `lfw-funneled.tgz` — with a Kaggle access
token read from `--kaggle-token-file`, then `KAGGLE_API_TOKEN` /
`KAGGLE_ACCESS_TOKEN`, then the repository's `.kaggle/access_token`. The token
is resolved lazily, so scoring a prepared work directory works offline.

```bash
cd backend/faces

# the four commands
.venv/bin/python -m tools.lfw_calibration --help

# fetch, extract and validate the dataset (kept until cleaned)
.venv/bin/python -m tools.lfw_calibration download
.venv/bin/python -m tools.lfw_calibration verify --work-dir /tmp/flashbacks-lfw

# score the protocol, sweep the threshold, enforce the 0.90 floor
.venv/bin/python -m tools.lfw_calibration calibrate --work-dir /tmp/flashbacks-lfw

# adopt the selected threshold in the service configuration
.venv/bin/python -m tools.lfw_calibration calibrate --apply-config app/config.py

# remove a work directory kept after a failed run
.venv/bin/python -m tools.lfw_calibration clean --work-dir /tmp/flashbacks-lfw
```

| Flag | Applies to | Purpose |
|---|---|---|
| `--work-dir PATH` | all | Dataset work directory; also `LFW_WORK_DIR`. Without it a fresh `tempfile.mkdtemp(prefix="flashbacks-lfw-")` outside the repository is used. |
| `--kaggle-token-file PATH` | `download`, `calibrate` | Kaggle access token file; defaults to `$KAGGLE_API_TOKEN` / `$KAGGLE_ACCESS_TOKEN`, then `<repo>/.kaggle/access_token`. |
| `--refresh` | `download` | Re-download even when an extracted copy exists. |
| `--keep` | `calibrate` | Keep the work directory after a successful run. |
| `--apply-config PATH` | `calibrate` | Rewrite `RECOGNITION_MIN_SIMILARITY` in `PATH` with the selected threshold, only when the run is accepted. |
| `--shipped-threshold TAU` | `calibrate` | Baseline threshold to score (defaults to the configured `RECOGNITION_MIN_SIMILARITY`). |
| `--model-id ID` | `calibrate` | Model pack id used in the embedding cache key (default `buffalo_l`); a mismatch with the engine's own pack fails the run. |
| `--quiet` | `calibrate` | Print only failures. |

Dataset lifecycle: `pairs.txt` and `lfw-funneled.tgz` are streamed from the
Kaggle per-file endpoint into the work directory, the pairs file is validated
(10 folds × 600 pairs) before the archive is fetched, the archive is extracted,
its `lfw_funneled/` tree is renamed to the canonical `lfw/`, and every pair image
is checked. Kaggle wraps the archive in a single-member ZIP while it serves
`pairs.txt` raw, so a download is unwrapped by its signature. Embeddings are
cached in `embeddings.json` under `sha256(image) + model id + preprocessing
version`, so repeat and A/B runs do not re-embed anything. A successful
`calibrate` deletes the whole directory (archive, images, cache); a failed run
keeps it and prints the path for diagnosis, and `clean` removes it. No LFW data
is committed or required on the host, and the pytest suite never downloads it.

Exit status: `0` when the mean accuracy at the shipped threshold reaches 0.90
and the selected threshold was reported, `1` when the floor is missed, the
dataset cannot be downloaded/validated or the configuration cannot be rewritten.
A below-floor run prints the measured accuracy and the best candidate threshold
and leaves `app/config.py` untouched.

The selected threshold keeps the threshold ordering: `RECOGNITION_MIN_SIMILARITY`
must stay at or below every raw-cosine clustering floor
(`CLUSTER_MIN_SIMILARITY`, `CLUSTER_ASSIGN_SIMILARITY`,
`CLUSTER_MERGE_SIMILARITY`, `CLUSTER_MERGE_SUPPORT_SIMILARITY`), which
[`tests/test_threshold_ordering.py`](tests/test_threshold_ordering.py) guards.
`CLUSTER_CORE_SIMILARITY` is a hub-corrected (CSLS) floor on a different scale
and cannot be ordered against a raw cosine.

The recorded run — environment, per-fold table, A/B comparison and the selected
threshold — lives in
[`docs/faces/verification-calibration.md`](../../docs/faces/verification-calibration.md:1).
The current result is a mean accuracy of 0.9822 at the shipped 0.25, so the
shipped threshold stays as it is.

## Clustering calibration (1:N)

The verification floor answers a 1:1 question. Clustering is a different one —
does the candidate graph plus the labelling turn a gallery into the right
people? [`tools/lfw_clustering.py`](tools/lfw_clustering.py) scores that on the
LFW identities, and [`tools/clustering_benchmark.py`](tools/clustering_benchmark.py)
does the same on any labelled tree (`<root>/<identity>/<image>`, or
`<identity>_<rest>.jpg` with `--flat-names`).

Both build the graph the way core does (cosine floor, `top_k` neighbours, a
per-photo candidate cap) and label it with the service's own
`FaceEngine.cluster_graph`, so a threshold or algorithm change is measured
through production code. Metrics: B-cubed precision/recall/F1, pairwise
precision/recall/F1, `merge rate` (clusters spanning more than one identity),
`identity merges` (distinct identity pairs glued together — the number that must
stay zero), `identity integrity` and the unassigned share. An unassigned face is
scored as its own singleton, never as a merge.

```bash
cd backend/faces

# LFW identities with two or more photos: shipped graph-v1 and production graph-v2
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw --algorithm graph-v1
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw --algorithm graph-v2

# how close different people get: the floor no graph can beat
.venv/bin/python -m tools.lfw_clustering hard-negatives --work-dir /tmp/flashbacks-lfw

# floor x core neighbours x assign x merge, ranked with merges first
.venv/bin/python -m tools.lfw_clustering sweep --work-dir /tmp/flashbacks-lfw

# the same metrics on the repository fixtures, with synthetic bursts added
.venv/bin/python -m tools.clustering_benchmark score \
  --images ../../tests/dataset/faces/scientists --flat-names --minimum-images 3 --bursts 2
```

`score` accepts gates for CI use (`--max-merge-rate`, `--min-b3`,
`--min-integrity`) and exits non-zero when one is missed; `sweep` fails when the
best configuration merges more identity pairs than `--max-merges` allows. Zero
merges is a gallery requirement, not an LFW one: about 15% of LFW faces have a
different person as their nearest neighbour above the 0.40 candidate floor, so
LFW runs are compared against the recorded baseline instead. The dataset and
embeddings share `tools.lfw_calibration`'s Kaggle download and content-addressed
cache, so a second command on the same work directory embeds nothing, and a
successful run removes the directory unless `--keep`.

LFW is closed-set, one face per photo and smaller than the production gallery,
so it cannot exercise `CLUSTER_MIN_PHOTOS`, the burst candidate cap or
`CLUSTER_MAX_SHARE`: those need the fixtures with synthetic bursts or a labelled
gallery export. Treat an LFW result as a regression gate and a direction, not as
the shipped clustering floor — the recorded two-tier procedure is in
[`docs/faces/clustering-calibration.md`](../../docs/faces/clustering-calibration.md:1).

## Configuration

| Env | Default | Purpose |
|---|---|---|
| `FACES_PORT` | `5173` | HTTP listen port. |
| `FACES_PROVIDERS` | `cpu` | Ordered ONNX Runtime providers (shorthands or full names); unavailable ones degrade to CPU with a warning. |
| `FACES_GPU_IDS` | empty (`FACES_GPU_ID`) | Comma-separated CUDA devices that each get their own model session and request slot, e.g. `0,1`. |
| `FACES_GPU_ID` | `0` | Single CUDA device id used when `FACES_GPU_IDS` is unset. |
| `FACES_MAX_BATCH` | `16` | Maximum images one `/faces/analyze-batch` request may carry (larger requests are rejected with `422`). |
| `FACES_RECOGNITION_BATCH` | `32` | Aligned small-face crops sent through the recognizer per forward pass; `1` keeps the per-image path. |
| `FACES_MIN_CLUSTERABLE_FACE_PX` | `32` | Smallest face (smaller box side) this service will cluster. A smaller face stays a detection: it is never labelled, never assigned and never founds a person. Set `0` to disable the rule. |
| `FACES_MAX_CONCURRENCY` | `1` | Concurrent analyses **per device**; each device has its own session, and more per device only duplicates its VRAM use. |
| `FACES_CLUSTER_ALGO` | `graph-v1` | Labelling revision: `graph-v1` (today's single linkage) or `graph-v2` (density-aware, see [Clustering revisions](#clustering-revisions)). |
| `FACES_CLUSTER_MODE` | `auto` | Read by **core**, not this service: clustering strategy (`auto`, `store`, `memory`) for the recognition job's candidate graph. |
| `FACES_CLUSTER_NEIGHBOURS` | `16` | Read by **core**, not this service: candidates per unassigned face when it queries the store for neighbours. |
| `FACES_CLUSTER_TIMEOUT` | `10m` | Read by **core**: budget for one clustering decision and the HNSW index build (`halfvec(512)` is required for the index). |
| `FACES_RECLUSTER_ALL` | `false` | Read by **core**: one-shot repair flag that deletes generated people (named ones are kept) before the next pass clusters the gallery again. |
| `REGISTRY_URL` | (empty) | Registry facade URL; empty disables registration. |
| `SERVICE_HOST` | `localhost` | Host peers use to reach this service. |
| `SERVICE_ID` | `<host>-<pid>` | Unique instance id. |
| `SERVICE_VERSION` | `dev` | Advertised version. |
| `REGISTRY_TOKEN` | (empty) | Bearer token for the registry facade. |

Registration is self-healing: the service retries with backoff until the registry
answers, heartbeats the lease, and re-registers after repeated heartbeat failures.
