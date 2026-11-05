# Spec Delta — face-recognition

## ADDED Requirements

### Requirement: 1:1 verification on LFW reaches the 90% floor

The faces service SHALL provide an offline calibration that measures 1:1 face
verification on the Labelled Faces in the Wild (LFW) benchmark using the official
6,000-pair, 10-fold protocol, and the mean fold accuracy at the shipped
recognition threshold SHALL be at least 0.90.

<!-- focused-spec:id=face-recognition.lfw-verification-accuracy -->
#### Scenario: Mean LFW accuracy meets the floor

- **WHEN** the calibration scores the official LFW pair protocol at the shipped recognition threshold
- **THEN** it reports the ten per-fold accuracies, their mean, and that mean is at least 0.90

<!-- focused-spec:id=face-recognition.lfw-verification-below-floor -->
#### Scenario: Accuracy below the floor fails the run

- **WHEN** the mean LFW accuracy at the shipped threshold is below 0.90
- **THEN** the calibration exits with a non-zero status, prints the measured accuracy and the best candidate threshold, and leaves the configuration unchanged

### Requirement: Verification accuracy follows the official LFW protocol

A pair SHALL be accepted as the same person when the cosine similarity of its two
embeddings is at least the shipped recognition threshold. Accuracy per fold SHALL
be the share of its 600 pairs decided correctly, and the reported accuracy SHALL
be the unweighted mean of the ten folds.

<!-- focused-spec:id=face-recognition.lfw-undetected-pair -->
#### Scenario: A pair with an undetectable face counts as a non-match

- **WHEN** either image of a pair yields no detectable face
- **THEN** the pair is scored as a non-match instead of being excluded from the accuracy denominator

<!-- focused-spec:id=face-recognition.lfw-fold-mean -->
#### Scenario: Every fold contributes equally

- **WHEN** the ten folds are scored
- **THEN** the reported accuracy is the arithmetic mean of the ten fold accuracies, not the pooled share of all 6,000 pairs

### Requirement: Every calibration candidate is scored on the same pairs

The calibration SHALL score every candidate configuration it compares — the
currently shipped threshold, the threshold sweep and any embedding or model
variant — against the identical LFW pair set and fold split, and SHALL report the
scores together.

<!-- focused-spec:id=face-recognition.lfw-ab-table -->
#### Scenario: A/B table compares candidates on identical pairs

- **WHEN** the calibration finishes with more than one candidate configuration
- **THEN** it prints one row per candidate with the same pair count and fold count, and the accuracy of each candidate

<!-- focused-spec:id=face-recognition.lfw-threshold-selected -->
#### Scenario: The best-scoring candidate is selected

- **WHEN** the threshold sweep finds a value with higher accuracy than the currently shipped threshold
- **THEN** the calibration reports that value as the selected threshold and reports the accuracy it achieved

### Requirement: The shipped recognition threshold is the calibrated one

The recognition similarity floor consumed by the core recognition job SHALL equal
the threshold selected by the LFW calibration and SHALL be the value reported by
`GET /faces/health` as the recognition threshold.

<!-- focused-spec:id=face-recognition.lfw-threshold-reported -->
#### Scenario: Health reports the calibrated recognition threshold

- **WHEN** the calibration has selected a recognition threshold and the faces service starts
- **THEN** `GET /faces/health` reports that threshold and the core recognition job uses it

<!-- focused-spec:id=face-recognition.lfw-floors-ordered -->
#### Scenario: No clustering floor sits below the recognition threshold

- **WHEN** the calibrated recognition threshold is adopted
- **THEN** every clustering similarity floor remains greater than or equal to it

<!-- focused-spec:id=face-recognition.lfw-threshold-unavailable -->
#### Scenario: A service without the threshold is never guessed

- **WHEN** the faces service is unreachable or does not report a recognition threshold
- **THEN** the recognition job substitutes no value of its own: it queries no candidates at a guessed radius, skips merges that would use a guessed floor, and clusters in memory or retries the pass

### Requirement: The LFW dataset is fetched on demand and removed after success

The calibration SHALL download LFW into a temporary work directory at run time,
require no dataset in the repository or on the host, and delete the archive,
images and derived embeddings after a successful run.

<!-- focused-spec:id=face-recognition.lfw-downloaded-on-demand -->
#### Scenario: A run without a local copy downloads LFW

- **WHEN** calibration starts and no local LFW copy exists
- **THEN** it downloads the archive and pairs file, and validates the pairs file contains 6,000 pairs across 10 folds before scoring

<!-- focused-spec:id=face-recognition.lfw-dataset-removed -->
#### Scenario: A successful run leaves no dataset behind

- **WHEN** a calibration run completes successfully
- **THEN** the temporary work directory no longer contains LFW images, the archive or derived embeddings

<!-- focused-spec:id=face-recognition.lfw-dataset-retained-on-failure -->
#### Scenario: A failed run keeps the workspace for diagnosis

- **WHEN** a calibration run fails
- **THEN** the work directory is retained and its path is printed, unless cleanup is explicitly requested
