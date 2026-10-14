# Proposal

## Why

The repository has an HTTP-level integration harness (`tests/webapp/integration/`)
that boots the real `core` service against the dedicated test database, but it only
covers bootstrap/login, account creation, and gallery-folder settings. The base
user journeys that define the product — scanning a folder, browsing the indexed
gallery, detecting duplicates, resolving a duplicate group, and moving files —
have no end-to-end regression coverage, and three of those behaviors (gallery
listing, duplicate grouping, file move) are not specified at all. A regression in
any of them would ship undetected.

## What Changes

- Specify the currently-unspecified base behaviors as new capabilities with
  focused-spec IDs, each bound to an executable HTTP integration test.
- Add integration tests under `tests/webapp/integration/` that drive the real
  service through the base journeys:
  - folder scan indexes images that then appear in `GET /api/gallery`;
  - `GET /api/gallery` paginates the indexed images;
  - `GET /api/duplicates` groups files that share identical content;
  - resolving a duplicate (delete-to-trash via `POST /api/delete-files`) removes
    it from the duplicate groups;
  - `GET /api/duplicates` returns an empty result when nothing is duplicated;
  - `POST /api/move-files` moves files to a target directory and resolves name
    conflicts by suffixing.
- Extend `tests/webapp/integration/support.ts` with the client helpers and
  fixture utilities these tests need (deterministic duplicate fixtures, scan
  polling, app-settings trash configuration).

No production code behavior changes; this change makes existing behavior
regression-tested and specified.

## Capabilities

### New Capabilities

- `gallery-browsing`: a configured gallery folder is scanned and its images are
  listed through the paginated gallery endpoint.
- `duplicate-detection`: files with identical content are grouped by the
  duplicates endpoint, a resolved group no longer appears, and an
  unduplicated library yields an empty result.
- `file-move`: selected files are moved to a target directory, with name
  conflicts resolved deterministically.

### Modified Capabilities

- None. The `trash` and `account-registration` capabilities already specify their
  behavior; the trash lifecycle is exercised transitively through
  `duplicate-detection.resolve-removes-group` rather than re-specified.

## Non-goals

- Re-specifying behavior already owned by `trash` (list/restore/permanent-delete/
  clean) or `account-registration`; those scenarios exist and are not duplicated.
- Covering provider-dependent features (LLM chat/embeddings, OCR recognition,
  geocoding) — they need external services and are not part of the deterministic
  base path.
- Changing production behavior; every assertion is added against the current
  service contract in `docs/api-contracts/core.yaml`.

## Impact

- Tests: `tests/webapp/integration/` (new suite files + `support.ts` additions).
- Evidence tooling: `.focused-spec/evidence.yaml` (new `current`-scope entries).
- Specs: new `openspec/changes/base-scenario-integration-tests/specs/**`.
- Contract reference: `docs/api-contracts/core.yaml` (`/api/scan`, `/api/gallery`,
  `/api/duplicates`, `/api/delete-files`, `/api/move-files`).
- Runtime requirement: the integration suite needs the Go toolchain and a
  reachable `flashbacks-test` PostgreSQL (unchanged from the existing suite).
