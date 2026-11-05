# Proposal

## Why

One person is currently discovered as six. In a 39,024-face production gallery the
same identity (Dima) exists as `people` rows 1905, 1906, 1924, 1934, 1974 and
5810, with 2 907 of those faces piled into a single chained group. Two structural
causes make this inevitable rather than accidental:

- the clustering pass only labels **unassigned** faces and always inserts a new
  `people` row, so every pass that analyses new photos mints new people for
  identities that already exist, and nothing ever merges them back;
- labelling itself is single-linkage over candidate links (connected components),
  so one chain of lookalikes merges unrelated people and the oversized group is
  re-split at a stricter similarity with no merge-back step — which cuts one
  person into exactly the fragments observed.

Neither cause can be fixed by moving a constant. There is also no labelled
evidence to measure the result, which is why the drift went unnoticed until it
was visible in the UI.

## What Changes

- **Identity persists across passes.** A clustering pass matches new faces and
  new clusters against **existing** people before creating a person, and a
  reconciliation step merges people whose reference vectors agree. New photos of
  a known identity extend that person instead of adding a new one.
- **Density-aware labelling replaces single linkage.** Candidate links count only
  when they are mutual, a face must have a minimum number of such neighbours to
  join a cluster, and similarity is hub-corrected so a face that is moderately
  close to everything cannot bridge two identities. Small or ambiguous faces are
  assigned to an established person instead of creating one.
- **A merge stage follows the split stage.** Groups that are still implausible
  are split as today, then neighbouring groups are merged when their reference
  vectors and cross-group support agree; the gallery-share cap stops being the
  splitting mechanism and becomes a diagnostic.
- **Photo diversity is enforced.** A person must span several distinct photos, and
  near-duplicates from one burst cannot dominate a person's neighbourhood.
- **Thresholds become a calibrated, reported contract.** Every threshold keeps
  living in the faces service and is reported by `/faces/health`; the values are
  recalibrated against a labelled fixture instead of a handful of aggregate
  counts.
- **A labelled fixture and A/B harness ship with the change.** `tests/dataset/faces/dima`
  holds 64 photos of one identity with a `manifest.csv` (face box, confidence,
  faces-on-photo). The harness embeds them once, caches embeddings, scores any
  clustering variant against them (fragments per identity, pairwise
  precision/recall, unassigned share) and gates the regression in pytest.
- **BREAKING** (internal contract): `/faces/cluster-graph` gains per-face photo
  identity and quality input so the service can enforce photo diversity and
  density; core must send them. The endpoint is consumed only by core, and both
  sides ship in this change.

## Capabilities

### New Capabilities

None. The behaviour that changes is the existing grouping guarantee; clustering
mechanics are the `how`, recorded in `design.md`.

### Modified Capabilities

- `face-recognition`: the guarantee "the same face across photos shares one
  person" is strengthened — one identity maps to exactly one person, that person
  survives later passes, fragments left by an earlier run are reconciled, and
  faces with no confident match stay unassigned instead of becoming a weak person.

## Impact

- `backend/faces/app/engine.py`, `config.py`, `api.py` — labelling, merge,
  reconciliation-relevant thresholds and the extended cluster payload.
- `backend/core/internal/application/faces/manager.go` — person-level assignment
  and reconciliation before `Create(&person)`, candidate query inputs.
- `backend/core/internal/domain/faces.go` — person reference vector/exemplars and
  a cluster version for one-shot re-clustering after an algorithm change.
- `backend/core/internal/infrastructure/database/` — migration, kNN candidate
  parameters.
- `docs/api-contracts/faces.yaml`, `docs/api-contracts/core.yaml` — additive
  request/response fields; `make generate-types` refreshes `webapp/src/types/api.ts`.
- `tests/dataset/faces/dima` (private, ignored by git), `backend/faces/tools/`,
  `backend/faces/tests/`, `tests/faces/` — fixture, harness and regression gate.
- No webapp behaviour change: person ids and names keep their current meaning, so
  no translation or UI work is implied.

## Non-goals

- No recognition-model change (`buffalo_l`: SCRFD-10G + ArcFace w600k_r50 stays)
  and no new model dependency.
- No manual merge/rename UI or "this is not the same person" action — user
  constraints are a separate change, even though the reconciler must not fight
  them later.
- No cross-library identity: matching stays inside one gallery.
- No face search endpoint, no GPU batching change, no webapp work.
- No attempt to make the historic production gallery perfect in this change: the
  reconciler must converge, but the measured acceptance is the labelled fixture
  plus a before/after report on the production snapshot.
