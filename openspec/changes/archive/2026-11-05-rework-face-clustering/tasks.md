# Tasks

> The private calibration fixture, the harness under `backend/faces/tools/`, its
> tests and the calibration report were removed after the change completed. The
> tasks below record what was done; the remaining gate is the hermetic synthetic
> pytest suite.

## 1. Fixture harness and baseline

- [x] 1.1 Add the calibration harness: read the private fixture manifest, embed each photo once through `FaceEngine.analyze`, and cache `embeddings.json` keyed by the manifest `sha256`. Verify: a first run reports `64 faces embedded, cache written`; a second run reports `cache hit` and does not load the model.
- [x] 1.2 Add the scoring core to the same module — fragments per identity, pairwise precision/recall, unassigned share, person count — and print the baseline of the current algorithm (`engine.cluster_graph` with today's thresholds) over the cached embeddings. Verify: the harness metric tests pass and the CLI prints the baseline table per production label.
- [x] 1.3 Add the threshold sweep (grid over `τ_core`, `τ_assign`, `τ_merge`, margin, `k`, `k_core`, `m`) printing a Pareto front, and record the baseline + candidate numbers in the calibration report. Verify: the report's commands reproduce the tables it prints (run from the repository root inside the `flashbacks-faces` container).

## 2. Faces service — labelling algorithm

- [x] 2.1 Implement hub correction `score(i,j) = 2·s − r̄(i) − r̄(j)` from the candidate list the caller sends, in `backend/faces/app/engine.py`. Verify: `backend/faces/.venv/bin/python -m pytest backend/faces/tests/test_csls.py -q` covers a hub face whose raw similarity links two identities while the corrected score does not.
- [x] 2.2 Replace connected components in `cluster_graph` with mutual-kNN core points (`k_core` mutual neighbours at `τ_core`) plus photo-diversity rules (edge per photo counted once, at most two candidates per source photo, a person spans at least two photos). Verify: `backend/faces/.venv/bin/python -m pytest backend/faces/tests/test_cluster_graph.py -q` covers a bridge face that no longer merges two identities and a burst sequence that yields one person, not several.
- [x] 2.3 Add the agglomerative merge stage after the cap-driven split: medoid similarity ≥ `τ_merge` and ≥ `m` mutual cross-cluster pairs ≥ `τ_support`, iterated to a fixed point. Verify: `pytest backend/faces/tests/test_cluster_graph.py -q` covers a person cut by the size cap rejoining, and `test_clustering_quality.py::test_distinct_identities_stay_separate` keeps two identities apart.
- [x] 2.4 Extend `/faces/cluster-graph` and `/faces/cluster` with optional aligned `photoIds`, `widths`, `qualities`; default `photoIds` to one unique photo per face so an older core keeps today's behaviour, and expose the new thresholds in `/faces/health`. Verify: `pytest backend/faces/tests/test_cluster_api_contract.py -q` covers both the aligned payload and the legacy payload without `photoIds`.
- [x] 2.5 Move the new knobs into `backend/faces/app/config.py` (`FACES_CLUSTER_ALGO=graph-v2|graph-v1` plus the calibrated values), document them in `backend/faces/README.md`, and verify: `pytest backend/faces/tests -q` is green and `docker exec flashbacks-faces python -c "from app.config import FaceSettings; print(FaceSettings.from_env().cluster_algo)"` prints `graph-v1` by default.

## 3. Core — person references and reconciliation

- [x] 3.1 Add the domain entity `PersonReference` and the port `person_ref_repo.go` (up to 5 exemplars per person, medoid lookup, replace-on-reconcile), the GORM implementation, and register the model in the `AutoMigrate` list (`backend/core/internal/infrastructure/database/database.go`). Verify: `cd backend/core && go test ./internal/domain/repository/... ./internal/infrastructure/database/... -count=1` passes on in-memory SQLite.
- [x] 3.2 Add the assignment step to `clusterAssign`: match unassigned faces against existing person references before sending anything for clustering, honouring `τ_assign` and the runner-up margin. Verify: `cd backend/core && go test ./internal/application/faces -run TestFaceRecognitionManager_IdentityStableAcrossPasses -count=1` passes.
- [x] 3.3 Add the reconciliation step (match new clusters against references, merge people whose medoids agree with support, build references lazily) and `people.cluster_version` with automatic re-reconcile on version change. Verify: `go test ./internal/application/faces -run TestFaceRecognitionManager_PeopleReconciled -count=1` passes and `TestFaceRecognitionManager_AmbiguousFaceUnassigned` keeps a weak match unassigned.
- [x] 3.4 Send the new face facts (`photoId`, box width, detection confidence) alongside the candidate links, keep the neighbour query unchanged, and regenerate Wire if a provider was added (`go generate ./...`, commit `wire.go` + `wire_gen.go`). Verify: `cd backend/core && go test ./internal/application/faces ./internal/infrastructure/database -count=1` passes and `git diff --stat` shows `wire_gen.go` only when the graph changed.

## 4. Contracts and generated types

- [x] 4.1 Update `docs/api-contracts/faces.yaml` (and the core proxy surface if it forwards the clustering call) with the new optional request fields, and refresh types with `make generate-types`. Verify: `cd webapp && npm run lint && npx tsc -b` passes and `git diff --stat webapp/src/types/api.ts` shows the additive fields only.

## 5. Calibration and A/B result

- [x] 5.1 Sweep on the fixture, choose the thresholds, and record the before/after table in the calibration report: fragments per identity must go from 6 (baseline) to 1 with pairwise precision unchanged on the impostor sets (`politicians`, `scientists`). Verify: rerunning the documented command reproduces every number in the table.
- [x] 5.2 Run the reconcile pass against a copy of the production gallery and record person count, fragments per identity and unassigned share before/after in the same report. Verify: the recorded numbers match a rerun on the same snapshot, and a second reconcile pass changes nothing (idempotence).

## 6. Evidence and checks

- [x] 6.1 Register the four new scenario IDs in `.focused-spec/evidence.yaml` (the registry entries are intentionally created in the apply phase): `face-recognition.identity-stable-across-passes` → `go::./internal/application/faces::TestFaceRecognitionManager_IdentityStableAcrossPasses`; `face-recognition.people-reconciled` → `go::./internal/application/faces::TestFaceRecognitionManager_PeopleReconciled`; `face-recognition.ambiguous-face-unassigned` → `go::./internal/application/faces::TestFaceRecognitionManager_AmbiguousFaceUnassigned`; `face-recognition.distinct-identities-stay-separate` → `facespytest::backend/faces/tests/test_clustering_quality.py::test_distinct_identities_stay_separate`. Verify each scope with `node_modules/.bin/focused-spec validate --scope <name> --strict` and `node_modules/.bin/focused-spec run --scope <name>`.
- [x] 6.2 Mutation-check every new evidence entry: revert the implementation hunk (not the test), observe `focused-spec run --scope <name>` fail on the behavioural assertion, restore, observe pass. Verify: the fail → restore → pass pair is recorded for each of the four scopes.
- [x] 6.3 Run the component suites end to end: `cd backend/core && go test ./internal/application/... -count=1`, `backend/faces/.venv/bin/python -m pytest backend/faces/tests -q`, and `cd webapp && npm run lint && npx tsc -b`. Verify: all three exit 0 and the change's own scenario scopes remain green.

## Workflow follow-up

- Sync the `face-recognition` delta into `openspec/specs/face-recognition/spec.md`.
- Archive the change after the review in `docs/agent/sdd-workflow.md` §7 passes.
