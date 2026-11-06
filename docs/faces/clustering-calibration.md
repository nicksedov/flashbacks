# Face clustering calibration — two tiers

The faces service calibrates its 1:1 similarity floor on LFW
([`verification-calibration.md`](verification-calibration.md:1)). Clustering asks
a different question — does a gallery come back as the right people? — and needs
a different measurement, because the 1:1 protocol says nothing about whether a
graph merges two identities or splits one.

This document describes the two-tier procedure implemented by
[`tools/lfw_clustering.py`](../../backend/faces/tools/lfw_clustering.py:1) and
[`tools/clustering_benchmark.py`](../../backend/faces/tools/clustering_benchmark.py:1)
and the rules for using their numbers.

## What is measured

Both tiers score the same pipeline the service runs:

1. each image is embedded once through `FaceEngine.analyze` (SCRFD detection,
   alignment, ArcFace, small-face refinement) and cached by content hash;
2. the candidate graph is built the way core queries the vector store — cosine
   similarity at or above the candidate floor, at most `top_k` neighbours per
   face, and a per-photo candidate cap;
3. the graph is labelled by `FaceEngine.cluster_graph`, so `graph-v1` and
   `graph-v2` are compared through production code, not a reimplementation;
4. the labels are compared with the ground-truth identities.

| Metric | Meaning | Why it is here |
|---|---|---|
| B-cubed P / R / F1 | per-face cluster precision/recall, averaged | stable when most identities hold few photos |
| pairwise P / R / F1 | exact same/different pair agreement from contingency counts | the classic clustering figure, no O(n²) matrix |
| `merge rate` | assigned clusters spanning more than one identity | the failure single-linkage is famous for |
| `identity merges` | distinct identity pairs glued into one cluster | the number that must stay at zero |
| `identity integrity` | identities that ended as exactly one person of their own | whole *and* alone |
| `unassigned` | faces no person claimed | the product tolerates this; it is not a merge |
| hard negatives | best cross-identity similarity per face | says whether any floor can separate lookalikes at all |

An unassigned face (`-1`) is scored as its own singleton cluster: a missed face
is a split, never a merge, which is how the open-set product behaves. Because
`merge rate` and `identity integrity` can disagree with pairwise precision on
skewed data, every report prints all of them, and `rank()` sorts merge safety
first, quality second.

## Tier 1 — LFW as a 1:N benchmark

LFW carries identity labels, so it can be repurposed from 1:1 verification to
closed-set clustering — with limits that must be stated together with the
numbers:

* usable ground truth is the **1,680 identities with two or more images**
  (9,164 faces); the 4,069 single-photo identities cannot be clustered and are
  skipped;
* the corpus is **skewed** (one identity holds hundreds of photos), so quality is
  reported per face and the ranking prefers zero merges rather than raw F1;
* every image holds **one face**, so `CLUSTER_MIN_PHOTOS`, the burst candidate cap
  and `CLUSTER_MAX_SHARE` are not exercised — Tier 2 covers those;
* it is **closed-set and small**: no "unknown" faces, and 9k faces against a
  production gallery measured in tens of thousands.

```bash
cd backend/faces

# shipped graph-v1 and production graph-v2, same work directory and cache
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw --algorithm graph-v1
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw --algorithm graph-v2

# the chaining ceiling: how many faces have a different person as nearest neighbour
.venv/bin/python -m tools.lfw_clustering hard-negatives --work-dir /tmp/flashbacks-lfw

# candidate floor x core neighbours x assign x merge floor, ranked merge-safety first
.venv/bin/python -m tools.lfw_clustering sweep --work-dir /tmp/flashbacks-lfw
```

Regression gates for CI compare against the recorded baseline, not against
zero — LFW contains genuine lookalikes:

```bash
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw \
  --algorithm graph-v1 --max-merge-rate 0.035 --min-b3 0.97
.venv/bin/python -m tools.lfw_clustering sweep --work-dir /tmp/flashbacks-lfw \
  --max-merges 58
```

`sweep` ranks merge safety first and fails only when the best configuration
merges more identity pairs than `--max-merges` allows. `--identities N` bounds a
smoke run; the dataset comes from Kaggle via `tools.lfw_calibration` and a
successful run removes the work directory unless `--keep`.

## Tier 2 — product-like benchmark

LFW cannot reproduce album bursts, group photos or the gallery scale, so the
second tier scores a labelled set with the same metrics:

```bash
cd backend/faces

# repository fixtures: flat <identity>_<rest>.jpg names, three photos of one person,
# plus two synthetic burst variants per photo (separate photos of one event)
.venv/bin/python -m tools.clustering_benchmark score \
  --images ../../tests/dataset/faces/scientists \
  --flat-names --minimum-images 3 --bursts 2

# any labelled export of the production gallery: <root>/<identity>/<image>
.venv/bin/python -m tools.clustering_benchmark score --images /path/to/gallery
.venv/bin/python -m tools.clustering_benchmark hard-negatives --images /path/to/gallery
```

The burst mode re-encodes every photo with a small exposure/crop/quality drift
and gives each variant its own photo id, which is what a burst is for the
labeller: many photos of one person in one event. That is the only way the
`CLUSTER_MIN_PHOTOS` rule and the per-photo candidate cap can be exercised
without a private album.

Group photos are deliberately out of scope: the harness embeds the largest face
of each image, so a photo of five people is one label. Use the existing
integration suite for presence, group recognition and geometry.

## Rules for changing a shipped floor

1. **LFW is a regression gate and a direction, not the decision.** A new
   `CLUSTER_MIN_SIMILARITY`, `CLUSTER_ASSIGN_SIMILARITY`, `CLUSTER_MERGE_*` or
   `CLUSTER_CORE_*` value must first not regress the recorded LFW numbers
   (merges, B-cubed F1, unassigned) and then be confirmed on a labelled gallery
   (Tier 2) before it ships.
2. **Zero merges is a gallery requirement, not an LFW one.** In LFW about 15% of
   faces have a different person as their nearest neighbour above 0.40 (see the
   hard-negative table below), so no configuration reaches zero merges there.
   On LFW, compare merge counts against the baseline; on the gallery, require
   zero identity merges.
3. **Never trade a merge for recall.** `identity_merges` outranks B-cubed and
   pairwise F1 in `rank()`; a configuration that groups more people by gluing
   two identifies is not an improvement.
4. **Keep the threshold ordering** `RECOGNITION_MIN_SIMILARITY <=` every
   raw-cosine clustering floor, guarded by
   [`tests/test_threshold_ordering.py`](../../backend/faces/tests/test_threshold_ordering.py:1).
5. **Record the run.** A change to a shipped clustering floor adds its LFW and
   gallery numbers to the recorded section below, with the command, the model
   pack and the environment.

## Recorded runs

### 2026-11-05 — stage 1 on LFW

Environment: `backend/faces/.venv`, ONNX Runtime `CPUExecutionProvider`,
`buffalo_l`, `FACES_RECOGNITION_BATCH=32`, preprocessing
`analyze-largest-face-v1`. Dataset from Kaggle `atulanandjha/lfwpeople`
(13,233 images); subset scored: **1,680 identities with ≥2 photos, 9,164
faces**. 36 faces (0.39%) yielded no detection and were scored as singletons.
First run embedded everything in ~75 minutes on this CPU host; every later
command reused the 103 MB cache in ~2 minutes.

```bash
cd backend/faces
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw --algorithm graph-v1
.venv/bin/python -m tools.lfw_clustering score --work-dir /tmp/flashbacks-lfw --algorithm graph-v2
.venv/bin/python -m tools.lfw_clustering hard-negatives --work-dir /tmp/flashbacks-lfw
.venv/bin/python -m tools.lfw_clustering sweep --work-dir /tmp/flashbacks-lfw
```

| Configuration | clusters | unassigned | B-cubed P / R / F1 | pairwise P / R / F1 | merge rate | identity merges | identity integrity |
|---|---|---|---|---|---|---|---|
| `graph-v1` @ floor 0.40 (shipped default) | 1,634 | 1.66% | 0.9808 / 0.9696 / **0.9751** | 0.9435 / 0.9695 / 0.9563 | 3.30% | **86** | **0.8887** |
| `graph-v2` @ floor 0.40 (production env) | 1,075 | 24.08% | 0.9899 / 0.8108 / **0.8914** | 0.9718 / 0.9577 / 0.9647 | 2.98% | **57** | **0.5815** |

Hard negatives (best cross-identity similarity per face):

| Statistic | Value |
|---|---|
| best | 0.9754 |
| p99 | 0.7800 |
| median | 0.2642 |
| faces with a foreign neighbour ≥ 0.30 | 2,341 (25.55%) |
| faces with a foreign neighbour ≥ 0.40 | 1,413 (15.42%) |
| faces with a foreign neighbour ≥ 0.50 | 1,254 (13.68%) |

Sweep over 135 configurations (floor 0.30–0.50 x core neighbours 2–4 x assign
0.25–0.35 x merge 0.45–0.55):

| Best by merge count | Value |
|---|---|
| floor / core neighbours / assign / merge | 0.50 / 4 / 0.35 / 0.50 |
| clusters / unassigned | 952 / 31.61% |
| B-cubed F1 / pairwise F1 | 0.8522 / 0.9565 |
| identity merges | 44 |
| identity integrity | 0.5036 |

What the numbers say:

- **LFW cannot reach zero merges.** About 15% of faces have a different person as
  their nearest neighbour above the candidate floor, so single-linkage and
  density labelling both glue some identities; 86 merges under `graph-v1` is the
  baseline, not a defect to gate on.
- **`graph-v2` halves merges but starves recall on this corpus**: 57 vs 86
  merges, yet 24% of faces stay unassigned and only 58% of identities end as one
  person of their own. LFW gives every identity one photo per shot, no bursts and
  no near-duplicates, which is exactly the structure `graph-v2`'s photo-based
  rules assume is present. This result is a reason to run Tier 2, not a reason to
  change a shipped floor.
- **A higher floor trades recall for merges**: at 0.50 the best configuration
  merges 44 pairs but leaves 31.6% unassigned and drops B-cubed F1 to 0.85; at
  0.45 it merges 58 pairs with 15.3% unassigned and F1 0.93.

### 2026-11-05 — stage 2 on the repository scientists fixture

```bash
cd backend/faces
.venv/bin/python -m tools.clustering_benchmark score \
  --images ../../tests/dataset/faces/scientists \
  --flat-names --minimum-images 3 --bursts 2
```

Three Einstein photos (frontal, profile, 1947) plus two synthetic bursts each
scored `identity merges 0`, `identity integrity 1.0000`, `merge rate 0.0000`,
`unassigned 0.0000` and B-cubed F1 `1.0000`, before and after the burst
augmentation (3 and 9 faces). One identity cannot produce a merge, so this run
proves the pipeline and the burst path, not the thresholds.
