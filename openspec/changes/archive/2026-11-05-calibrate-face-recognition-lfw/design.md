# Design

## Context

See [`proposal.md`](proposal.md) for motivation. The constraints that shape the
approach:

- **The faces service already owns the verification primitives.** `FaceEngine`
  runs InsightFace `buffalo_l` (SCRFD-10G detection + ArcFace `w600k_r50`,
  512-dim L2-normalised embeddings) through [`analyze()`](../../../backend/faces/app/engine.py:267)
  and [`embed()`](../../../backend/faces/app/engine.py:398); `/faces/embed` and
  `/faces/analyze-batch` are thin wrappers over them.
- **Thresholds are module constants reported by `/faces/health`.** The
  recognition floor is [`RECOGNITION_MIN_SIMILARITY`](../../../backend/faces/app/config.py:57)
  (0.25), the runner-up margin is `RECOGNITION_MARGIN` (0.05), and the clustering
  floors are `CLUSTER_MIN_SIMILARITY` (0.4), `CLUSTER_ASSIGN_SIMILARITY` (0.30),
  `CLUSTER_MERGE_SIMILARITY` (0.50) and the `graph-v2` core/merge-support values.
  Core reads the recognition floor from `/faces/health` rather than duplicating
  it, so changing the constant is enough to change the job's behaviour.
- **The previous calibration fixture and harness are gone.** Commit `f40ee13`
  removed the private `dima` fixture, `backend/faces/tools/` and the clustering
  calibration report. There is no calibration code to extend; this change
  reintroduces tooling on a public benchmark.
- **LFW is the canonical 1:1 benchmark.** The archive at
  `https://vis-www.cs.umass.edu/lfw/lfw.tgz` (about 173 MB) contains 13,233
  images of 5,749 people; `pairs.txt` defines 6,000 pairs in 10 folds of 300
  same-person and 300 different-person pairs.
- **Tests must stay hermetic.** The pytest suite runs without network or model
  downloads; only an explicit calibration run may reach the internet.

## Goals / Non-Goals

**Goals:**

- One command that downloads LFW, scores the official 1:1 protocol, sweeps the
  decision threshold, prints an A/B table and enforces the ≥90% floor.
- Byte-for-byte reproducibility: the recorded report can be regenerated from the
  download command alone, on any host with outbound HTTPS.
- A clear adoption rule for candidates (threshold → TTA → model swap) with the
  migration cost of each option made explicit.
- The dataset exists only for the duration of a run and is deleted on success.

**Non-Goals:**

- No 1:N clustering benchmark; LFW identities are used only through the official
  pairs file.
- No runtime download inside the running service, and no dataset in git.
- No change to the `/faces/*` request/response shapes or to core's data model.
- No automatic production-gallery re-clustering unless a model swap is adopted.

## Decisions

### D1 — Use the official LFW pairs protocol, mean of ten folds

Score exactly the pairs in `pairs.txt`: 10 folds, 600 pairs each (300 same, 300
different). Accuracy per fold = correct decisions / 600; the reported accuracy is
the unweighted mean of the ten folds, which is the standard LFW verification
figure and is robust to any fold-level imbalance. A pair is accepted as the same
person when the cosine similarity of its two embeddings is ≥ the decision
threshold.

Alternatives considered: pooling all 6,000 pairs into one accuracy (simpler, but
not comparable with published LFW numbers); building custom pairs from identity
folders (more data, but no official protocol).

### D2 — Embed through the production path, cache by content hash

Every distinct image is embedded once through `FaceEngine` (detection + alignment
+ ArcFace), and the result is cached in the work directory keyed by
`sha256(image bytes)` + model id + preprocessing version. This measures the same
pipeline the service runs, including its detector and small-face handling. Multi-
GPU sessions and `analyze-batch` are reused; a second A/B candidate reuses the
cache and never reloads images or the model.

Alternatives considered: calling `/faces/embed` over HTTP (needs a running
service, serialises the run); embedding pre-aligned LFW crops directly (skips the
detector, so the measured number would not describe production).

If either image of a pair yields no face, the pair is scored as a non-match: a
same-person pair becomes an error and a different-person pair becomes correct.
This mirrors production, where an undetected face cannot be matched. Excluding
such pairs was rejected because it inflates accuracy and hides detector failures.

### D3 — Sweep the cosine threshold; tie-break toward the stricter value

Sweep τ from 0.05 to 0.95 in 0.005 steps and report accuracy for every value.
The selected threshold is the accuracy-maximising τ; ties are broken toward the
higher τ (fewer false accepts, and the service uses the threshold as a floor for
"same person"). The run reports accuracy at the currently shipped τ and at the
selected τ, so the A/B comparison is explicit.

Adoption keeps threshold ordering: the selected `RECOGNITION_MIN_SIMILARITY` must
stay at or below every clustering floor (`CLUSTER_MIN_SIMILARITY`,
`CLUSTER_ASSIGN_SIMILARITY`, `CLUSTER_MERGE_SIMILARITY`, `graph-v2` core/merge
values). A test asserts the ordering so a future floor edit cannot silently make
clustering stricter than verification or vice versa.

### D4 — Candidate ladder: threshold → flip TTA → model swap

Candidates are scored on the identical pair set and fold split:

1. `shipped` — the current constant (baseline);
2. `sweep` — the best τ from D3;
3. `flip` — horizontal-flip TTA: average the embedding of the original image and
   its mirrored copy, then L2-normalise. Same 512-dim space, no re-embedding of
   stored vectors;
4. `model` — a different InsightFace pack such as `antelopev2`. Same dimension
   (512) but a different embedding space, so every stored face embedding becomes
   invalid.

The smallest change that reaches ≥90% and maximises accuracy is adopted;
threshold-only is preferred, then TTA, then a model swap. This ordering is driven
by migration cost: the first two need no data migration, the third requires a
full re-embed/recluster of the gallery.

### D5 — Dataset lifecycle: system temp by default, deleted on success

`download` streams the LFW archive and `pairs.txt` into a work directory created
with `tempfile.mkdtemp(prefix="flashbacks-lfw-")` (outside the repository),
extracts the archive and validates the pairs file (10 folds × 600 pairs,
300 + 300 per fold) before any scoring. `--work-dir`/`LFW_WORK_DIR` can point at
a prepared copy; `--keep` disables cleanup.

Source of record: the harness fetches both files from the Kaggle dataset
`atulanandjha/lfwpeople`, because the canonical host is unreachable here. That
dataset carries the official `pairs.txt` and `lfw-funneled.tgz`, whose identity
folders sit under `lfw_funneled/`; the download renames them to the canonical
`lfw/`, and Kaggle serves the archive inside a single-member ZIP (pairs arrive
raw), so the transport is unwrapped by signature before extraction. Requests use
a Kaggle access token resolved lazily from `--kaggle-token-file`,
`KAGGLE_API_TOKEN`/`KAGGLE_ACCESS_TOKEN`, then the repository's
`.kaggle/access_token`, so scoring a prepared directory needs no credentials.

A successful `calibrate` run removes the entire work directory — archive,
extracted images, embedding cache and scratch output. A failed run retains it and
prints the path so the failure can be diagnosed; `clean` removes a retained
directory. Alternative layouts considered: caching under `tests/dataset/faces/lfw`
(rejected: the request requires removal after success, and a repo-adjacent path
invites accidental commits) and a permanent model-side cache (rejected: no
dataset may survive a successful run).

### D6 — Hermetic tests with injected downloader and engine

`lfw_calibration.py` takes its HTTP opener and engine factory as parameters.
Tests inject a local opener serving tiny synthetic archives and a fake engine
returning deterministic embeddings, and cover:

- pairs parsing and fold validation (10 × 600, 300 same / 300 different);
- pair scoring including the undetected-face rule;
- fold aggregation, threshold sweep and tie-breaking;
- the acceptance gate: below-floor accuracy exits non-zero and leaves the
  configuration untouched;
- A/B table shape with identical pair counts per candidate;
- cleanup on success and retention on failure, using `tmp_path`.

No test downloads LFW or loads a model. The real run is a calibration task and is
recorded in the report, not re-executed in CI.

### D7 — Result report is the acceptance artifact

`docs/faces/verification-calibration.md` records the exact command, environment
(model id, provider, device), the per-fold table, the A/B table, the selected
threshold and the resulting accuracy. It contains no images or embeddings, so it
is safe to commit; running the documented download command reproduces every
number.

### D8 — Core consumption needs no contract change

Core reads the recognition floor from `/faces/health`; the endpoint already
reports the threshold list, so a recalibrated constant propagates without an
OpenAPI or Go change. No Wire regeneration is involved. If a model swap is
adopted, core's existing re-embed/recluster path is used, which is an operational
runbook step rather than a code change.

## Risks / Trade-offs

- [Download host unavailable or rate-limited] → clear non-zero exit with the URL;
  `--work-dir` accepts a pre-downloaded copy; no configuration is touched.
- [LFW is street photography, not the production gallery] → keep the existing
  `tests/faces` integration checks for presence and group recognition; never
  raise a floor beyond the LFW-selected τ without gallery evidence.
- [A stricter threshold increases unassigned faces in production] → monitor the
  job's unassigned share after rollout; reverting the constant is the rollback,
  and `RECOGNITION_MARGIN` is unchanged by this change.
- [Flip TTA doubles inference cost] → it is only adopted when threshold-only
  misses the floor, and only the calibration path pays the cost; decide runtime
  cost before enabling it in the service.
- [A model swap invalidates stored embeddings] → the candidate ladder delays it
  to last, and adoption includes a documented re-embed/recluster run with
  `FACES_RECLUSTER_ALL`; rollback requires re-embedding from originals.
- [Counting detection failures as non-matches understates accuracy] → accepted as
  conservative and production-faithful; the report states the rule.
- [Mirror drift changes the archive contents] → validate pair structure and image
  count before scoring and fail fast with the observed numbers.
- [Real calibration is slow (≈13k images)] → the content-hash cache makes repeat
  and multi-candidate runs cheap; GPU batching is already configured.

## Migration Plan

1. Run the calibration in the faces environment (venv or `flashbacks-faces`
   container) against a fresh download; record the baseline at the current
   threshold.
2. Threshold-only outcome: update `RECOGNITION_MIN_SIMILARITY` in
   [`config.py`](../../../backend/faces/app/config.py:57), restart the faces
   service, and verify `/faces/health` reports the new value. No data migration;
   rollback is the previous constant.
3. TTA outcome: ship the engine variant behind the existing configuration
   surface, re-run the calibration, and keep stored embeddings unchanged.
4. Model outcome: update the model pack, re-embed the gallery and re-cluster with
   `FACES_RECLUSTER_ALL`, then re-run the calibration. Rollback requires
   re-embedding with the previous pack — record this in the report and runbook.
5. Confirm cleanup: the work directory is gone and `git status` lists only the
   planned source, test and documentation files.

## Open Questions

- Which alternative model pack to adopt if the baseline misses the floor is
  decided by the measured A/B in the fallback tasks; the specs, approach and task
  breakdown do not depend on the winner.
