# Tasks

## 1. Runner and harness setup

- [x] 1.1 Extended `tests/webapp/integration/support.ts` with the helpers the base
      suites need (scan polling, gallery/duplicates reads, `deleteFiles`,
      `moveFiles`, settings reads, a deterministic `resetLibraryState`) and
      verified `cd webapp && npx tsc -p ../tests/webapp` passes.
- [x] 1.2 Made the integration suites reachable by the existing `vitest` runner by
      splitting the root `vitest.config.ts` into `webapp-unit` and
      `webapp-integration` projects (design D2), and verified the CLI resolves an
      integration selector (`vitest::tests/webapp/integration/...`) while existing
      `vitest::webapp/src/...` evidence still resolves.

## 2. gallery-browsing suite

- [x] 2.1 Added `tests/webapp/integration/gallery-browsing.test.ts` asserting
      `gallery-browsing.scan-indexes-images`; verified it passes and fails when the
      gallery builder is reverted.
- [x] 2.2 Added the `gallery-browsing.gallery-paginated` assertion; verified it
      passes and fails when the page parameter is ignored.

## 3. duplicate-detection suite

- [x] 3.1 Added `tests/webapp/integration/duplicate-detection.test.ts` asserting
      `duplicate-detection.groups-identical-content`; verified pass and
      fail-when-grouping-broken.
- [x] 3.2 Added the `duplicate-detection.no-duplicates-empty` assertion; verified
      pass and fail-when-distinct-files-grouped.
- [x] 3.3 Added the `duplicate-detection.resolve-removes-group` assertion; verified
      pass and fail-when-DB-removal-skipped.

## 4. file-move suite

- [x] 4.1 Added `tests/webapp/integration/file-move.test.ts` asserting
      `file-move.move-to-target-dir`; verified pass and fail-when-move-is-a-no-op.
- [x] 4.2 Added the `file-move.move-name-conflict` assertion; verified pass and
      fail-when-conflict-handling-is-removed.

## 5. Evidence binding and completion checks

- [x] 5.1 Added the eight `current`-scope evidence entries to
      `.focused-spec/evidence.yaml` with exact
      `vitest::tests/webapp/integration/<file>::<describe> > <it>` selectors;
      `focused-spec validate --scope current` passes.
- [x] 5.2 Ran each new scenario via `focused-spec run --scenario <id>`: all seven
      report `PASS`.
- [x] 5.3 Performed the controlled product-regression check on all seven new
      evidence references (revert the protected behavior → `FAIL` → restore →
      `PASS`); every product file was restored and `git status` is clean for them.
- [x] 5.4 Ran the full integration suite (`make test-webapp-integration`: 6 files,
      31 tests pass), `npx tsc -p ../tests/webapp`, and
      `openspec validate base-scenario-integration-tests` — all green.

## Workflow follow-up

- Sync the delta specs into `openspec/specs/**` (`openspec-sync-specs`), moving the
  focused-spec IDs to their main-spec homes.
- Archive the change into `openspec/changes/archive/YYYY-MM-DD-base-scenario-integration-tests/`
  after sync, then re-run `focused-spec validate --scope current`.
