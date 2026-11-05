# Proposal

## Why

The `faces` service ships recognition and clustering thresholds that were tuned
against a private fixture, which has since been removed with its harness. Nothing
in the repository now measures 1:1 face verification on a public, reproducible
benchmark, so a threshold tweak, a model swap or an engine change cannot be shown
to keep recognition quality acceptable.

LFW is the standard 1:1 verification benchmark: 13,233 images of 5,749 people and
an official 6,000-pair, 10-fold protocol, freely downloadable with no credentials.
Calibrating against it gives the service a public quality floor and an auditable
A/B procedure for every future threshold, model or algorithm change.

## What Changes

- Add an offline calibration harness under `backend/faces/tools/` that
  dynamically downloads the canonical LFW archive and `pairs.txt` from
  `https://vis-www.cs.umass.edu/lfw/` (the same LFW corpus as the linked Kaggle
  mirror), embeds each image once through the service's own `FaceEngine`, and
  scores the official 6,000-pair, 10-fold verification protocol.
- Sweep the cosine decision threshold and print an A/B table that scores every
  candidate (current shipped threshold, swept candidate, and — when needed — an
  alternative embedding variant or model) on the identical pair set.
- Acceptance gate: mean 1:1 verification accuracy on LFW is at least 90% at the
  shipped recognition threshold; the calibration exits non-zero and leaves the
  configuration untouched otherwise.
- Recalibrate the recognition threshold in [`app/config.py`](../../../backend/faces/app/config.py:57)
  when the sweep shows a better value; `GET /faces/health` keeps reporting the
  effective thresholds, so core keeps consuming one source of truth.
- Dataset lifecycle: LFW is fetched on demand into a temporary work directory and
  the archive, extracted images and derived embedding cache are deleted after a
  successful run. No LFW data is committed or required to be preinstalled.
- If the baseline cannot reach 90%, the change may replace the recognition model
  (for example `antelopev2`) or add a verification-time enhancement such as flip
  TTA. Every such option is A/B-scored with the same harness before it ships, and
  embedding-space implications are handled explicitly.
- Record the protocol, the A/B table, the selected threshold and the measured
  accuracy in a tracked calibration report so the result is reproducible from the
  download command alone.

**BREAKING** (conditional, internal): if the calibration adopts a different
embedding model, existing stored face embeddings become invalid and the gallery
must be re-embedded and re-clustered. A threshold-only outcome requires no data
migration.

## Capabilities

### New Capabilities

None. The calibration is part of the existing face-recognition behaviour, not a
separate capability.

### Modified Capabilities

- `face-recognition`: adds requirements that 1:1 verification is calibrated
  against LFW with at least 90% accuracy at the shipped threshold, that every
  candidate configuration is A/B-scored on the same pairs, and that the
  downloaded dataset is removed after a successful run.

## Impact

- `backend/faces/tools/lfw_calibration.py`, `backend/faces/tools/__init__.py` and
  `backend/faces/tests/test_lfw_calibration.py` — new harness and hermetic tests.
- [`backend/faces/app/config.py`](../../../backend/faces/app/config.py:57) — the
  calibrated recognition threshold (and any dependent floor that must stay above
  it).
- [`backend/faces/README.md`](../../../backend/faces/README.md:1) and a new
  `docs/faces/verification-calibration.md` — protocol, commands and recorded
  result.
- `.gitignore` — the temporary work directory when it is placed inside the
  repository (the default lives in the system temp directory).
- No OpenAPI or MCP contract change: `/faces/embed`, `/faces/analyze`,
  `/faces/cluster*` and `/faces/health` keep their shapes. If a model change is
  adopted, the embedding dimension stays 512 but the vector space changes, which
  requires a re-embed/recluster run in core.
- Operators need outbound HTTPS to `vis-www.cs.umass.edu` (about 173 MB) to run a
  calibration; the test suite stays hermetic and never downloads LFW.

## Non-goals

- No 1:N clustering benchmark on LFW; the acceptance metric is 1:1 verification
  accuracy.
- No runtime download by the running service — only the offline calibration tool
  fetches LFW.
- No webapp/UI change and no new user-facing text.
- No change to the recognition job schedule or core's person lifecycle beyond
  consuming the calibrated threshold.
- No accuracy claim about the production gallery from LFW alone; gallery-specific
  recall is checked separately against the existing `tests/faces` photo sets.
