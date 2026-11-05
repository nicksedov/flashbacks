# Design

## Context

See [`proposal.md`](proposal.md) for motivation. The constraints that shape the
approach:

- **Core owns the people table.** [`clusterAssign()`](../../../backend/core/internal/application/faces/manager.go:695)
  loads only `person_id = 0` faces, asks the faces service for labels and then
  inserts one `people` row per label. `Person`
  ([`domain/faces.go`](../../../backend/core/internal/domain/faces.go:19)) stores no
  reference vector, so the job cannot recognise a person it created earlier.
- **The faces service owns every threshold.** They are module constants in
  [`config.py`](../../../backend/faces/app/config.py:32) and are reported by
  `/faces/health`; core reads the recognition floor from there
  ([`manager.go`](../../../backend/core/internal/application/faces/manager.go:932)).
- **Labelling is single linkage.** [`_graph_groups()`](../../../backend/faces/app/engine.py:629)
  union-finds candidate edges; [`_split_chained_groups()`](../../../backend/faces/app/engine.py:669)
  then re-splits an oversized group at 0.70…0.90 similarity with no merge-back.
- **Candidates are truncated kNN.** Core queries pgvector for 16 neighbours per
  face at the 0.25 recognition floor
  ([`face_neighbours.go`](../../../backend/core/internal/infrastructure/database/face_neighbours.go:17)).
- **The calibration fixture is private.** It held 64 photos of one identity and
  was excluded from version control, then removed once calibration finished.
- **Environment for calibration.** The faces container ships Python 3.11,
  `insightface 2.1` (`buffalo_l`), `numpy`, `sklearn`, two GPUs; `docker cp` is
  blocked on this host, so files move via `docker exec … cat`/`tar`.

## Goals / Non-Goals

**Goals:**

- One identity resolves to one person — in a fresh pass *and* across passes, and
  previously fragmented people converge on later passes.
- A measurable, reproducible calibration procedure instead of aggregate counts.
- A contract change that keeps clustering semantics inside the faces service.

**Non-Goals:**

- No model change, no new Python dependency, no GPU batching change.
- No webapp work and no manual merge/rename UI.
- No cross-library identity.

## Decisions

### D1 — People carry a reference set; assignment precedes creation

`people` gets a reference set of up to **E = 5** exemplar vectors in a new
`person_references` table (`person_id`, `face_id`, `embedding halfvec(512)`,
`quality`, `created_at`) plus a derived medoid. A clustering pass runs three steps
instead of one:

1. **Assign** — every unassigned face is compared with the reference set of every
   existing person; it joins a person when `best ≥ τ_assign` **and**
   `best − runner_up ≥ margin`, and it may only join through its own candidates.
2. **Cluster** — the remaining faces are labelled by the faces service (D2) and
   become **candidates** for a new person, not people yet.
3. **Reconcile** — a new cluster is matched against the reference sets again; it
   extends the matching person or creates a person only when nothing matches.

Rationale: a single centroid blurs modes of one identity (age, beard, glasses,
lighting), which is exactly what produced the fragments; exemplars keep those
modes apart without a full re-cluster. Alternatives considered: a single
`embedding` column on `people` (cheaper, loses modes); recomputing a centroid
from member faces on demand (O(person size) per comparison, and a 2 900-face
person dominates the pass).

Layering: domain entity `PersonReference` + port
`domain/repository/person_ref_repo.go`, GORM implementation under
`infrastructure/database`, consumed by `application/faces`. If the port is
registered as a new Wire provider, `wire.go` and the regenerated `wire_gen.go`
must ship together (`go generate ./...`).

### D2 — Density-constrained graph instead of connected components

Inside the faces service:

- only **mutual** edges count (both faces name each other) — a one-way link is
  what a lookalike bridge looks like;
- a face is a **core point** only with `k_core` (3–5) mutual neighbours at
  `τ_core`; components are built from core points only;
- non-core faces never bridge components; they are assigned by reference vote
  against the resulting clusters (bounded by `τ_assign` and the margin);
- the gallery-share cap stays, but as a **diagnostic plus fallback split**; after
  splitting, the merge stage (D3) runs over *all* resulting components, so a
  person cut by the cap is rejoined when the evidence agrees.

Alternatives considered: HDBSCAN (already rejected in
[`engine.py`](../../../backend/faces/app/engine.py:455) for labelling small
galleries as noise and merging strangers — its behaviour depends on
`min_cluster_size`, which cannot be derived from gallery size reliably);
spectral clustering (O(N³) and a full affinity matrix); Chinese Whispers
(no calibrated threshold semantics, unstable partitions).

### D3 — Merge stage with support, after splitting

Two clusters merge when the cosine similarity of their medoids clears `τ_merge`
**and** at least `m` mutual cross-cluster face pairs clear `τ_support`, applied
agglomeratively until stable. Requiring support (not just medoid similarity)
keeps two similar-looking strangers apart while still rejoining pieces of one
person, whose cross links are numerous.

### D4 — Hub correction (CSLS) from the candidates already fetched

`score(i, j) = 2·s(i,j) − r̄(i) − r̄(j)`, where `r̄(i)` is the mean similarity to
`i`'s own k nearest candidates. No extra query: core already sends each face's
candidate list, and the faces service can compute `r̄` from it. This removes the
hub effect (a face moderately close to everything) that turns any single-linkage
graph into one component.

### D5 — Photo diversity

Every cluster face carries its `photoId`; an edge between two faces of the same
photo is counted once, a candidate list keeps at most 2 faces per source photo,
and a person must span at least 2 distinct photos. Rationale: burst sequences
otherwise fill the 16-neighbour budget and dominate every similarity vote.

### D6 — Contract: `/faces/cluster-graph` gains aligned face facts (**BREAKING**, internal)

Request gains `photoIds`, `widths`, `qualities` aligned with `faceIds`; the
response is unchanged. `/faces/cluster` (memory path) accepts the same optional
arrays. Core supplies facts, the service decides semantics — the alternative
(core filters edges) would split clustering policy across two components. When
the arrays are absent the service falls back to one unique photo per face, so an
older core keeps today's behaviour. Contracts: `docs/api-contracts/faces.yaml`
(and the core proxy surface if exposed), then `make generate-types`.

### D7 — Person-level reconciliation runs every pass, not only on demand

`people.cluster_version` (mirroring `facesScanVersion` on images) marks which
clustering generation produced a person. A version mismatch triggers the
reconcile step automatically; `ReclusterAll` remains as the manual lever. This
also lets the historic production gallery converge without a wipe: reconciling
merges existing people rather than deleting and recreating them, so user-visible
ids disappear only for merged duplicates.

### D8 — Calibration and what counts as evidence

Calibration ran against a private labelled fixture through a local harness that
embedded each photo once (cached by `sha256`), scored any variant (fragments per
identity, pairwise precision/recall, unassigned share, person count) and swept
`τ_core`, `τ_assign`, `τ_merge`, `margin`, `k`, `k_core`, `m`. The fixture, the
harness and its report were removed afterwards.

Because the fixture was private and gitignored, it **cannot** be the focused-spec
evidence: an always-skipped scope is not a gate. Behavioural evidence is
hermetic pytest cases over synthetic vectors (well-separated identities, a bridge
face, a burst photo, an ambiguous face). Go-level evidence for the person
lifecycle lives in `manager_test.go` on in-memory SQLite.

## Risks / Trade-offs

- [Merge stage over-merges] → merge requires medoid similarity **and** `m`
  mutual cross-links; `τ_merge` is calibrated above the impostor ceiling
  measured on `politicians`/`scientists`; the `distinct-identities-stay-separate`
  scenario is a regression guard.
- [Assignment cost] O(faces × persons × E) dot products → compute in numpy as one
  matrix product per pass; only faces that failed the graph step are compared
  when the graph already covers the gallery.
- [Residual fragments] a face below `τ_assign` stays a separate person →
  reconciliation repeats every pass, and the residual is reported, not hidden.
- [Contract break] core and faces must ship together → the service defaults
  missing `photoIds` to unique values, so an old core degrades instead of failing.
- [Person id churn after a merge] links keyed by a merged-away id 404 → the merge
  keeps the oldest id and reassigns faces; no webapp change is needed because the
  People view re-reads the list.
- [Private fixture] A/B numbers are not reproducible in CI → manifest `sha256` +
  documented regeneration SQL make the fixture reproducible on any host that has
  the gallery.

## Migration Plan

1. Additive schema only (`person_references`, `people.cluster_version`) via the
   existing `AutoMigrate` list — no rewrite of `faces`/`people` rows.
2. Build references lazily on the first reconcile pass from existing members; no
   blocking migration job.
3. Deploy faces first (contract defaults keep the running core working), then
   core.
4. Rollback without a redeploy: `FACES_CLUSTER_MODE=memory` stays as today's
   lever, and the new labelling is behind `FACES_CLUSTER_ALGO=graph-v2|graph-v1`.
5. Production verification: A/B report on the fixture, then a reconcile pass on
   the stored gallery comparing person count, fragments per identity and
   unassigned share before/after.

## Open Questions

- Exemplar count `E` fixed at 5 versus adaptive per person — storage is small
  either way, so this can be answered from the sweep output.
- Whether the merge stage should later move into the faces service once the
  service also owns person identity; today it stays in core because core owns the
  people table.
