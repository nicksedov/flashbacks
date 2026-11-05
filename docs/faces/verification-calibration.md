# Face verification calibration — LFW

Recorded result of the 1:1 face-verification calibration of the `faces`
service against the Labelled Faces in the Wild (LFW) benchmark, produced by
[`backend/faces/tools/lfw_calibration.py`](../../backend/faces/tools/lfw_calibration.py:1).

**Outcome:** mean LFW accuracy **0.9822** at the shipped recognition threshold
`RECOGNITION_MIN_SIMILARITY = 0.25`. The sweep selected the same value, so the
shipped threshold is already the accuracy-maximising one and the configuration
did not change.

| Item | Value |
|---|---|
| Shipped threshold scored | `0.25` |
| Mean accuracy at the shipped threshold | `0.9822` |
| Acceptance floor | `0.90` — met |
| Selected threshold (sweep, strictest tie) | `0.25` |
| Configuration change | none (selected == shipped) |

## Protocol

- The official LFW verification protocol: `pairs.txt` with 10 folds of 600
  pairs each (300 same-person, 300 different-person), 6,000 pairs in total.
- Each distinct image is embedded once through the service's own
  [`FaceEngine.analyze()`](../../backend/faces/app/engine.py:267) — SCRFD-10G
  detection, alignment, ArcFace `w600k_r50`, plus the small-face refinement —
  so the measurement covers the production pipeline rather than pre-aligned
  crops.
- A pair is a match when the cosine similarity of its two embeddings is at
  least the scored threshold.
- Accuracy per fold is the share of its 600 pairs decided correctly; the
  reported accuracy is the **unweighted mean of the ten folds**.
- An image that yields no detectable face scores its pair as a non-match
  instead of being dropped from the denominator. 27 of the 7,701 distinct
  images in this run produced no face (0.35%).
- The threshold sweep runs from 0.05 to 0.95 in 0.005 steps; ties are broken
  toward the stricter (higher) threshold.

## Environment

| Component | Value |
|---|---|
| Service image | `localhost:5000/flashbacks/faces:latest` (`app` user, `/opt/venv`) |
| InsightFace | 2.1 |
| ONNX Runtime | 1.31.1, `CUDAExecutionProvider` |
| OpenCV | 5.0.0 |
| NumPy | 2.4.6 (container) |
| Model pack | `buffalo_l`, 512-dim embeddings |
| Device | GPU 0 — NVIDIA RTX A4000 (`FACES_GPU_IDS=0`) |
| Service env | `FACES_PROVIDERS=cuda,cpu`, `FACES_RECOGNITION_BATCH=32` |
| Host CPU runtime (cross-check) | same image on CPU: ~0.66 s/image vs ~59 ms/image on the A4000 |
| Run time | 7,701 images embedded in ~7 minutes |

## Dataset

The harness fetches LFW from Kaggle and from nowhere else (this host cannot
resolve `vis-www.cs.umass.edu`):

- dataset `atulanandjha/lfwpeople` — the official `pairs.txt` (`10 300` header,
  6,000 pairs) plus `lfw-funneled.tgz`, the funneled archive that keeps the
  canonical `lfw_funneled/<Name>/<Name>_NNNN.jpg` layout: 13,233 images of
  5,749 people, every pair image present;
- authentication — a Kaggle access token read from `--kaggle-token-file`, then
  `KAGGLE_API_TOKEN`/`KAGGLE_ACCESS_TOKEN`, then the repository's
  `.kaggle/access_token`. Resolution is lazy, so scoring a prepared work
  directory needs no credentials;
- transport — the Kaggle per-file endpoint answers `pairs.txt` raw and returns
  `lfw-funneled.tgz` inside a single-member ZIP, so a download is unwrapped by
  its signature before extraction;
- layout — after extraction the harness renames `lfw_funneled/` to `lfw/`, the
  layout the pair protocol and the embedding cache address, and validates all
  6,000 pairs against the extracted images.

```bash
cd backend/faces
.venv/bin/python -m tools.lfw_calibration download --work-dir /tmp/flashbacks-lfw
.venv/bin/python -m tools.lfw_calibration verify   --work-dir /tmp/flashbacks-lfw
.venv/bin/python -m tools.lfw_calibration calibrate --work-dir /tmp/flashbacks-lfw
```

The recorded run predates the built-in Kaggle download: its work directory was
assembled once by hand from `akirasosa/lfwfaces` (`lfw/raw/images/*.jpg`, the
same 13,233 images stored flat) plus the `pairs.txt` of
`atulanandjha/lfwpeople`, hardlinked into `lfw/<Name>/<Name>_NNNN.jpg`. The
protocol and the scored images are the same corpus and the same 6,000 pairs, so
the numbers below stand; a fresh run now needs no manual step.

The commands actually executed for this record (inside the faces image, with
the repository's `backend/faces/app` and `backend/faces/tools` on
`PYTHONPATH`, dataset mounted at `/tmp/dataset`):

```bash
python -m tools.lfw_calibration verify --work-dir /tmp/dataset
python -m tools.lfw_calibration calibrate --work-dir /tmp/dataset --keep
python -m tools.lfw_calibration calibrate --work-dir /tmp/dataset   # cleanup check
```

`verify` printed:

```text
pairs: 6000 (3000 same-person, 3000 different-person) in 10 folds of 600
images: 13233
```

## Per-fold accuracy at the shipped threshold (0.25)

| Fold | Pairs | Accuracy |
|---|---|---|
| 1 | 600 | 0.9850 |
| 2 | 600 | 0.9817 |
| 3 | 600 | 0.9900 |
| 4 | 600 | 0.9867 |
| 5 | 600 | 0.9833 |
| 6 | 600 | 0.9783 |
| 7 | 600 | 0.9767 |
| 8 | 600 | 0.9733 |
| 9 | 600 | 0.9833 |
| 10 | 600 | 0.9833 |
| **Mean** | **6000** | **0.9822** |

## A/B comparison

Every candidate is scored on the identical pair set and fold split.

| Candidate | Threshold | Pairs | Folds | Accuracy |
|---|---|---|---|---|
| shipped | 0.250 | 6000 | 10 | 0.9822 |
| sweep (selected) | 0.250 | 6000 | 10 | 0.9822 |

The sweep is a flat plateau: 0.235, 0.240, 0.245 and 0.250 all score 0.9822,
and 0.265 scores 0.9820. The tie-break rule picks the strictest threshold of
the maximum, which is exactly the shipped 0.25 — so no recalibration is
adopted and `app/config.py` stays as it is.

The threshold ordering is preserved: `RECOGNITION_MIN_SIMILARITY` (0.25) stays
at or below every raw-cosine clustering floor (0.30 assign, 0.40 cluster,
0.50 merge), which
[`backend/faces/tests/test_threshold_ordering.py`](../../backend/faces/tests/test_threshold_ordering.py:1)
guards.

Core keeps no copy of these numbers. It reads them from `/faces/health` on every
pass, and when the service does not report them the pass substitutes nothing:
it queries no candidates, skips merges and clusters in memory (or retries), so a
recalibration reaches the job with a faces restart and no core change. The
behaviour lives in [`Manager.thresholds()`](../../backend/core/internal/application/faces/reconcile.go:90)
and is guarded by `TestThresholdsRequireTheServicesValues`,
`TestClusterAssignDegradesWithoutThresholds` and
`TestReconcileSkipsWithoutThresholds`.

## Defect found and fixed while measuring

The first real run aborted inside the small-face refinement path:

```text
cv2.error: ... (-215:Assertion failed) dims <= 2 in function 'operator()'
```

[`FaceEngine._embed_crops`](../../backend/faces/app/engine.py:327) passed a
stacked NumPy array to `recognizer.get_feat(...)`. InsightFace wraps a non-list
argument into a one-element list, so OpenCV received one four-dimensional
"image" and `blobFromImages` rejected it. Every group photo with a face narrower
than `SMALL_FACE_WIDTH` (64 px) hit this crash, and LFW contains such faces.

Fixed by passing the aligned crops as a list; the regression test
`test_refined_crops_reach_the_recognizer_as_a_list` in
[`backend/faces/tests/test_engine_batch.py`](../../backend/faces/tests/test_engine_batch.py:1)
uses a fake recognizer that enforces the real `get_feat` contract (list of
`HxWxC` crops) and fails on the previous ndarray call.

## Dataset lifecycle

The first run used `--keep`, so the work directory (archive-free images,
`pairs.txt` and the 87 MB `embeddings.json` cache) stayed for inspection. The
second run without `--keep` reused the cache (`images: 7701 distinct, 0 freshly
embedded`), reproduced every number above and removed the directory:

```text
accepted: mean accuracy 0.9822 at the shipped threshold 0.25; selected 0.25 (0.9822)
work directory removed: /tmp/dataset
```

No image, embedding or archive from LFW is stored in this repository; running
the commands above reproduces every number in this document.
