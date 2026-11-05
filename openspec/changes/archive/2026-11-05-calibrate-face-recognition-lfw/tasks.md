# Tasks

## 1. LFW calibration harness

- [x] 1.1 Add `backend/faces/tools/__init__.py` and `backend/faces/tools/lfw_calibration.py` with a CLI exposing `download`, `verify`, `calibrate` and `clean`, plus injectable HTTPS opener and engine factory. Verify: `backend/faces/.venv/bin/python -m tools.lfw_calibration --help` lists all four commands.
- [x] 1.2 Implement the on-demand download into a `tempfile.mkdtemp` work directory, archive extraction and pairs validation (10 folds × 600 pairs, 300 same + 300 different per fold). Verify: `backend/faces/.venv/bin/python -m pytest backend/faces/tests/test_lfw_calibration.py -q` covers a fake opener writing a synthetic archive and rejects a malformed pairs file.
- [x] 1.3 Implement pair parsing and image-path resolution from the extracted tree. Verify: unit tests cover the ten-fold split, the name-to-path mapping and rejection of a pairs file with the wrong fold count.
- [x] 1.4 Implement the content-hash embedding cache (image sha256 + model id + preprocessing version) and engine-factory usage. Verify: fake-engine tests assert one embedding call per distinct image and a cache hit that constructs no engine on the second run.
- [x] 1.5 Implement pair scoring with the undetected-face non-match rule, per-fold accuracy, the 0.05–0.95 threshold sweep with stricter tie-break, the A/B table and the ≥90% acceptance gate. Verify: unit tests cover each rule, including a below-floor run that exits non-zero and writes no configuration.
- [x] 1.6 Implement the dataset lifecycle (`--work-dir`/`LFW_WORK_DIR`, `--keep`, `clean`), deleting the archive, images and embeddings after success and retaining the directory on failure. Verify: tests on `tmp_path` assert the directory is gone after success, retained after failure, and removed by `clean`.
- [x] 1.7 Register the new scenario IDs from `specs/face-recognition/spec.md` in `.focused-spec/evidence.yaml` and bind them to the harness tests. Verify: `node_modules/.bin/focused-spec validate --scope face-recognition --strict` passes, and each binding is mutation-checked (revert the protected behaviour, observe failure, restore, observe pass).
- [x] 1.8 Document the harness commands, flags and dataset lifecycle in `backend/faces/README.md`. Verify: every documented command runs as written with `--help`.

## 2. Calibration run and shipped threshold

- [x] 2.1 Run `download` and `verify` in the faces environment (venv or `flashbacks-faces` container) and capture the baseline accuracy at the currently shipped threshold. Verify: the run completes with ten folds of 600 pairs and prints the per-fold and mean baseline accuracy.
- [x] 2.2 Run the threshold sweep and A/B comparison; adopt the selected threshold when the mean accuracy is at least 0.90. Verify: the output reports a mean ≥ 0.90 and the A/B table shows identical pair and fold counts per candidate.
- [x] 2.3 Update [`RECOGNITION_MIN_SIMILARITY`](../../../backend/faces/app/config.py:57) to the calibrated value and add a guard test asserting every clustering floor stays at or above it. Verify: `backend/faces/.venv/bin/python -m pytest backend/faces/tests -q` passes and the health test reports the new threshold through `GET /faces/health`.
- [x] 2.4 Write `docs/faces/verification-calibration.md` with the protocol, commands, environment, per-fold and A/B tables, selected threshold and measured accuracy. Verify: the documented command reproduces the recorded numbers and the document contains no images or embeddings.
- [x] 2.5 Confirm automatic cleanup after a successful run without `--keep`. Verify: the printed work directory no longer exists and `git status --porcelain` lists no LFW files.

## 3. Fallback candidates when the baseline misses the floor (conditional)

- [ ] 3.1 Implement the `flip` TTA candidate (average the embedding of the original image and its mirrored copy, L2-normalise) inside the harness and score it on the identical pairs; adopt it only if threshold-only misses the floor. Verify: the A/B table includes the candidate with the same pair count and hermetic tests cover the averaging and normalisation.
- [ ] 3.2 Score an alternative InsightFace pack (for example `antelopev2`) through the same harness; if adopted, update the model configuration, confirm the embedding dimension stays 512, and document the re-embed/recluster runbook. Verify: `backend/faces/.venv/bin/python -m pytest backend/faces/tests -q` passes and the report records the dimension and the migration step.
- [ ] 3.3 When a fallback candidate is adopted, update the calibration report and `backend/faces/README.md` with the final configuration. Verify: the documented commands reproduce the newly recorded numbers.

## 4. Integration checks

- [x] 4.1 Run the full faces suite. Verify: `backend/faces/.venv/bin/python -m pytest backend/faces/tests -q` exits 0.
- [x] 4.2 Run the core suite that consumes the reported threshold. Verify: `cd backend/core && go test ./internal/application/... -count=1` exits 0.
- [x] 4.3 Validate the change artifacts. Verify: `openspec validate calibrate-face-recognition-lfw --strict` reports no errors.
- [x] 4.4 Confirm the repository is dataset-free. Verify: `git status --porcelain` shows only the planned source, test and documentation files, and no LFW work directory remains.

## Workflow follow-up

- Sync the delta spec into `openspec/specs/face-recognition/spec.md` before archiving.
- Archive the change after review and verification.
