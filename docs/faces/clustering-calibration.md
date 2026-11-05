# Face clustering calibration

This document records how the face-clustering thresholds are calibrated and
what the labelled fixtures measure. It is the reproducible A/B report for the
`rework-face-clustering` change: one identity that the production pipeline split
into six people must resolve to one.

The private fixture is **not** a CI gate (it is excluded from version control).
The behavioural gate is the hermetic pytest suite over synthetic candidate
graphs (`backend/faces/tests/test_cluster_graph.py`,
`test_csls.py`, `test_clustering_quality.py`). This report is the evidence used
to choose the numbers.

## What is measured

- **fragments per identity** — how many people one ground-truth identity is
  split into (target: `1`);
- **pairwise precision** — share of same-person pairs that really share an
  identity (must stay `1.0` on the impostor sets);
- **pairwise recall** — share of same-identity pairs placed in one person;
- **unassigned share** — faces left without a person;
- **person count** — distinct people produced.

The harness rebuilds the candidate graph the core service sends (k nearest
neighbours above the recognition floor, capped per source photo) and labels it
with the algorithm under test. It never re-implements the algorithm.

## Fixtures

| Fixture | Faces | Identity | Notes |
|---|---|---|---|
| `tests/dataset/faces/dima` | 63 | one (Dima) | private; the six production labels `1905`, `1906`, `1924`, `1934`, `1974`, `5810` are fragments of one face |
| `tests/dataset/faces/politicians` | 4 | Obama ×2, Merkel, Macron | portraits only; group photos are skipped because their faces span identities |
| `tests/dataset/faces/scientists` | 5 | Einstein ×3, Curie, Bohr | portraits only |

Why the baseline is the **stored partition**, not a re-run: the production
fragments were produced by repeated passes (each pass minted new people) and by
the cap-driven split of a 2 895-face group. A 63-face sample is below the cap
of 250, so a single-batch re-run of `graph-v1` converges to one person and cannot
reproduce the failure. The manifest `person_id` column records what the stored
gallery actually holds — six people — and that is the `6` in `6 → 1`.

## Reproduction

The embedding cache (`embeddings.json`, keyed by each photo's manifest
`sha256`) ships next to a fixture: a second run reports `cache hit` and does not
load the model.

### Local virtualenv (repository root)

```bash
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py embed
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py baseline
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py v2
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py sweep
```

The first `embed` run reports `63 faces embedded, cache written`; every later run
of any stage reports `cache hit` for the fixture.

### Inside the `flashbacks-faces` container

`docker cp` is blocked on hosts that front the Docker socket with an allow-list,
so the fixture and the tool are streamed with `tar`:

```bash
tar -C tests/dataset/faces -cf - dima politicians scientists \
  | docker exec -i flashbacks-faces sh -c \
      'rm -rf /tmp/calibration && mkdir -p /tmp/calibration && tar -xf - -C /tmp/calibration'
# The tool must run against this working tree, not the image's frozen copy of
# the service: stage the sources in /tmp and put them first on PYTHONPATH.
tar -C backend/faces -cf - app tools \
  | docker exec -i flashbacks-faces sh -c \
      'rm -rf /tmp/facestool && mkdir -p /tmp/facestool && tar -xf - -C /tmp/facestool'

docker exec -w /tmp -e PYTHONPATH=/tmp/facestool flashbacks-faces \
  python -m tools.calibrate_clustering baseline --dataset /tmp/calibration/dima
docker exec -w /tmp -e PYTHONPATH=/tmp/facestool flashbacks-faces \
  python -m tools.calibrate_clustering v2 --dataset /tmp/calibration/dima
docker exec -w /tmp -e PYTHONPATH=/tmp/facestool flashbacks-faces \
  python -m tools.calibrate_clustering sweep \
    --dataset /tmp/calibration/dima \
    --dataset /tmp/calibration/politicians \
    --dataset /tmp/calibration/scientists
```

The copied `dima/embeddings.json` makes these container runs cache hits, so the
reported numbers match the local run without a model pass. Running from `/tmp`
prevents the image's `/app/app` from shadowing the staged sources.

## Baseline — stored partition and `graph-v1`

```bash
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py baseline
```

| Variant | Faces | People | Fragments | Precision | Recall | Unassigned |
|---|---|---|---|---|---|---|
| stored production partition | 63 | 6 | **6** | 1.000 | 0.429 | 0.0% |
| `graph-v1` single pass | 63 | 1 | 1 | 1.000 | 1.000 | 0.0% |

Per production label after the `graph-v1` re-run:

| Label | Faces | Clusters | Largest | Unassigned |
|---|---|---|---|---|
| `1905` | 40 | 1 | 40 | 0 |
| `1906` | 5 | 1 | 5 | 0 |
| `1924` | 2 | 1 | 2 | 0 |
| `1934` | 9 | 1 | 9 | 0 |
| `1974` | 5 | 1 | 5 | 0 |
| `5810` | 2 | 1 | 2 | 0 |

## Result — `graph-v2` at the calibrated thresholds

```bash
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py v2
```

| Fixture | Faces | People | Fragments | Precision | Recall | Unassigned |
|---|---|---|---|---|---|---|
| `dima` | 63 | 1 | **1** | 1.000 | 1.000 | 0.0% |
| `politicians` (4 portraits) | 4 | 1 | 1 (Obama) | 1.000 | 1.000 | 50.0% |
| `scientists` (5 portraits) | 5 | 0 | — | 1.000 | 1.000 | 100.0% |

Fragments per identity therefore go from **6 → 1**, with pairwise precision
unchanged at `1.0` on both impostor sets. A single-photo face alone cannot make
a person (a person spans at least two photos), so the unassigned share on the
portrait-only sets is expected; in the gallery those faces are picked up by the
person-reference assignment (`τ_assign` + margin).

## Threshold sweep

```bash
backend/faces/.venv/bin/python backend/faces/tools/calibrate_clustering.py sweep
```

The grid covers `k ∈ {8, 16, 24}`, `τ_core ∈ {−0.05, 0, 0.05, 0.1}`,
`k_core ∈ {2, 3, 4}`, `τ_assign ∈ {0.25, 0.30, 0.35}`,
`τ_merge ∈ {0.45, 0.50, 0.55}`, `m ∈ {2, 3}` and
`margin ∈ {0.03, 0.05, 0.08}` — 1 944 combinations. Variants that merge two
different people on either impostor set are discarded before the Pareto front,
which is computed over fragments, precision and unassigned share.

Output on the fixtures above:

```
Pareto front (1215 of 1944 combinations, impostor precision 1.0 on 2 sets; 1 distinct outcomes)
  k   core  kcore  assign  merge  m  margin  frag   prec  recall  unassigned  people
 16   0.00      3    0.30   0.50  2    0.05     1  1.000   1.000        0.0%       1
```

The front has a single distinct outcome; the printed representative is the grid
point closest to the configured defaults, which is why the chosen values below
are the defaults rather than a boundary point.

### Chosen values

| Knob | Constant in `backend/faces/app/config.py` | Value |
|---|---|---|
| algorithm | `CLUSTER_ALGO` / `FACES_CLUSTER_ALGO` | `graph-v1` by default, `graph-v2` when enabled |
| candidates per face (`k`) | core `ClusterNeighbours` (query side) | 16 |
| core floor (`τ_core`) | `CLUSTER_CORE_SIMILARITY` | `0.00` (hub-corrected scale) |
| core neighbours (`k_core`) | `CLUSTER_CORE_NEIGHBOURS` | `3` |
| assign floor (`τ_assign`) | `CLUSTER_ASSIGN_SIMILARITY` | `0.30` |
| runner-up margin | `RECOGNITION_MARGIN` | `0.05` |
| merge floor (`τ_merge`) | `CLUSTER_MERGE_SIMILARITY` | `0.50` |
| cross-component support (`m`) | `CLUSTER_MERGE_SUPPORT` | `2` |
| support floor (`τ_support`) | `CLUSTER_MERGE_SUPPORT_SIMILARITY` | `0.40` |
| candidates per source photo | `CLUSTER_CANDIDATES_PER_PHOTO` | `2` |
| photos per person | `CLUSTER_MIN_PHOTOS` | `2` |

`τ_core` sits on the hub-corrected (CSLS) scale, where a pair at its own
neighbourhood mean scores `0`; it is not comparable to the raw `0.40` floor. All
values are reported by `/faces/health` so core never re-implements them.

## Production gallery reconcile

Run on the live gallery (39 024 faces, 4 147 legacy people) after releasing
`graph-v2`. The legacy partition held Dima's sampled faces in six people.

| Metric | Before `graph-v2` | After the repair pass | After a second reconcile |
|---|---|---|---|
| People | 4 147 | **672** | 672 |
| Dima sampled faces (63) | **6 fragments** | **2 fragments (60 + 2)** | 2 |
| Largest person | 2 907 | 4 715 | 4 715 |
| Unassigned faces | 2 911 (7.46%) | 6 896 (17.67%) | 6 896 (17.67%) |
| Person references | 0 | 3 145 | 3 145 |

The first production pass (release 17) exposed a defect the fixtures could not:
the reconcile union-found every qualifying medoid pair, and a chain of lookalike
people collapsed into one person of 23 871 faces. The fix (releases 18–19) makes
reconcile merge only mutual best matches with a bounded round, bounds a merged
person by a share of the gallery, and replaces the faces-service merge loop with
an O(links) version (the old one recomputed medoids per pair and hung on the
39k-face graph). The gallery was then rebuilt with the one-shot re-cluster lever
(`FACES_RECLUSTER_ALL=true`) and reconciled.

After the repair the 4 147 legacy fragments became 672 people. The largest
person holds 4 715 faces (14.7% of the assigned faces); the next four hold
4 085, 3 818, 3 486 and 2 880 — the expected shape of a family gallery's main
identities. Dima's 63 sampled faces now live in two people (60 + 2); the
remaining pair is a density outlier that does not clear the merge evidence,
which is the residual the design reports instead of hiding. The unassigned
share rose to 17.67% because `graph-v2` requires mutual density before a face
joins a person; those faces stay available to the person-reference assignment
on later passes.

The second reconcile call changed no row (0.5 s versus 82 s for the converging
call), so the fixed-point iteration is idempotent.

Reproduction on a host that holds the gallery database:

```bash
docker exec -e PGPASSWORD=<pw> postgres psql -U postgres -d flashbacks -c \
  "CREATE TABLE dima_faces AS SELECT id FROM faces WHERE id IN (
     SELECT face_id FROM unnest(ARRAY[...]) face_id);"  # fill from manifest.csv

cd backend/core
SNAPSHOT_DSN='host=<db-host> port=5432 user=postgres password=<pw> dbname=flashbacks sslmode=disable' \
  go test ./internal/application/faces -run TestSnapshotReconcile -count=1 -v
```

`TestSnapshotReconcile` prints a `SNAPSHOT_REPORT` JSON with the before/first/
second metrics; it skips unless `SNAPSHOT_DSN` is set. Fill `dima_faces` from
the `face_id` column of `tests/dataset/faces/dima/manifest.csv` (63 rows) so the
report follows the labelled sample rather than production row ids.

## Known limits

- The fixture is private and gitignored; it produces this report, not the CI
  gate.
- The portrait sets measure false merges only: recall on them is meaningless
  because each person appears on one or two portraits and group photos are
  skipped (their faces cannot be attributed to one identity without labels).
- The production snapshot report needs a gallery database; it is reproducible
  with the commands above on any host that holds one, but not from the
  repository alone.
