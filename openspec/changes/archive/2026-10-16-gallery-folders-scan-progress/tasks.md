# Tasks

## 1. Backend — structured scan status (core / imaging)

- [x] 1.1 Extend `ScanStatusResponse` in
      [`scan_manager.go`](../../../backend/core/internal/application/imaging/scan_manager.go:12)
      with `Phase`, `CurrentDirectory`, `TotalFiles`, `FilesAdded`,
      `FilesDeleted`, `FilesUpdated` (keep `Progress`); verify
      `cd backend/core && go build ./...` succeeds.
- [x] 1.2 Add a metadata-only per-directory image pre-count and accumulate it
      into `TotalFiles` in `StartScan`/`FastScanGallery`; verify
      `TestScanManager_StatusTotalFiles` passes.
- [x] 1.3 Track phase transitions (`cleanup` → `scanning` → `complete`) and the
      current directory during a scan; verify `TestScanManager_StatusPhase`
      passes.
- [x] 1.4 Derive `FilesProcessed`, `FilesAdded`, `FilesDeleted`, `FilesUpdated`
      from the `ScanBatchResult`/`FastScanResult` counts instead of progress
      messages; verify `TestScanManager_StatusFileCounters` and
      `TestScanManager_StatusFilesProcessed` pass.
- [x] 1.5 Confirm `GET /api/status` serializes the new fields (handler/DTO) and
      run `cd backend/core && go test ./internal/application/... -count=1` green.

## 2. API contract and TypeScript types

- [x] 2.1 Update the `ScanStatus` schema in
      [`core.yaml`](../../../docs/api-contracts/core.yaml) with `phase`,
      `currentDirectory`, `totalFiles`, `filesAdded`, `filesDeleted`,
      `filesUpdated`, and correct `running` → `scanning`; verify
      `make generate-types` completes and `webapp/src/types/api.ts` shows the new
      fields.
- [x] 2.2 Align [`ScanStatusResponse`](../../../webapp/src/types/index.ts:46) and
      the [`useScanStatus`](../../../webapp/src/hooks/useScanStatus.ts:13) default
      with the new contract; verify `cd webapp && npm run lint && npx tsc -b`
      passes.

## 3. Shared progress indicator (webapp)

- [x] 3.1 Add a composed `ProgressIndicator` to
      [`progress.tsx`](../../../webapp/src/components/ui/progress.tsx:5) with
      `value` (undefined ⇒ indeterminate), `label`, `statusText`, and optional
      `stats`, rendering as a borderless `bg-muted` inset fill with semantic
      tokens only; verify it renders determinate and indeterminate in a unit test.
- [x] 3.2 Add `webapp/src/components/ui/progress.test.tsx` asserting the shared
      primitive is used by both scan displays, the indeterminate state, the
      Russian label, token-only classes (no `bg-blue-`/raw palette), and the
      absence of a `border` utility inside a `Card`; verify
      `cd webapp && npx vitest run src/components/ui/progress.test.tsx` passes.

## 4. Progress-bar consumers and localization

- [x] 4.1 Add English and Russian i18n keys for the scan phases
      (`cleanup`/`scanning`/`complete`) and the statistics line to
      [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:310) and
      [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:310);
      verify `webapp/src/i18n/translations.test.ts` passes.
- [x] 4.2 Refactor
      [`ScanProgressBanner.tsx`](../../../webapp/src/components/ScanProgressBanner.tsx:10)
      to use `ProgressIndicator`, a localized phase label, and the real
      statistics line; verify a `ScanProgressBanner.test.tsx` asserts the
      localized label and the "scanned X of Y / added / deleted" text.
- [x] 4.3 Refactor the Automatic Image Tagging progress in
      [`AdminAnalysisTab.tsx`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx:639)
      to use `ProgressIndicator`; verify `AdminSettingsTab.test.tsx` still passes.
- [x] 4.4 Run `cd webapp && npm run lint && npx tsc -b` green.

## 5. Gallery Folders action row

- [x] 5.1 Group "Add folder" (primary), "Scan Changes", and "Rescan All"
      (`outline`) into one action row in
      [`AdminGeneralTab.tsx`](../../../webapp/src/components/tabs/AdminGeneralTab.tsx:366),
      moving the `AddFolderForm` trigger out of the card header; verify a
      component test asserts all three controls render in a single row and
      `cd webapp && npm run lint && npx tsc -b` passes.

## 6. Design-system documentation

- [x] 6.1 Add reconciliation-backlog rows in
      [`docs/design-system.md`](../../../docs/design-system.md:7) for the raw-palette
      scan banner, the message-based counter, the ad-hoc tag-scan bar, and the
      inconsistent action-row button styles — each marked done and referencing the
      new `design-system` requirement; verify the table rows resolve their links
      and the file renders.

## 7. Evidence and integration checks

- [x] 7.1 Bind the nine scenario IDs in
      [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) with
      their exact selectors; verify
      `node_modules/.bin/focused-spec validate --scope current` passes.
- [x] 7.2 Run `focused-spec run --scope current` (or per-scenario) so every new
      scenario reports `PASS`.
- [x] 7.3 For each new/changed evidence reference, perform the controlled
      regression check (revert behavior → `FAIL` → restore → `PASS`); verify all
      product files are restored.
- [x] 7.4 Run `cd backend/core && go test ./internal/application/... -count=1`,
      `cd webapp && npm run lint && npx tsc -b && npm test`, and
      `openspec validate gallery-folders-scan-progress` — all green.

## Workflow follow-up

- Sync the `design-system` and `gallery-browsing` deltas into `openspec/specs/**`
  via `openspec-sync-specs`, moving the focused-spec IDs to their main-spec homes,
  then run `openspec validate --specs`.
- Archive the change into
  `openspec/changes/archive/YYYY-MM-DD-gallery-folders-scan-progress/` after sync,
  then re-run `focused-spec validate --scope current`.
