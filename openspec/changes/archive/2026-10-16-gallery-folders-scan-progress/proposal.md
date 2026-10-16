# Proposal

## Why

The Gallery → Folders settings area and the two background-scan progress bars are
visually inconsistent and under-informative. "Add folder" renders as a filled
primary button while "Scan Changes" and "Rescan All" are outlined and sit in a
separate row. The scan banner uses raw blue palette colors (so it looks light in
dark themes) inside a bordered box, its "Cleaning up missing files…" text is
hardcoded English regardless of the interface locale, and its counter reports
progress-channel *messages* rather than files — so it perpetually reads "0 files
processed". The Automatic Image Tagging section renders a second, independently
hand-rolled progress bar. Operators need one themed, localized progress
indicator that reports real scan statistics.

## What Changes

- Group "Add folder", "Scan Changes" and "Rescan All" into a single action row in
  the Gallery Folders card, with one consistent button treatment (primary add
  action + outlined scan actions).
- Introduce one shared, token-based progress indicator (determinate and
  indeterminate) and use it for both the gallery scan banner and the Automatic
  Image Tagging progress.
- Replace the free-text English scan progress string with a structured,
  localized scan phase (`cleanup` | `scanning` | `complete`) plus the directory
  currently being scanned.
- Extend `GET /api/status` additively (non-breaking) with `phase`,
  `currentDirectory`, `totalFiles`, `filesAdded`, `filesDeleted`, and
  `filesUpdated`, and correct `filesProcessed` to count files rather than
  progress messages.
- Add English and Russian localization for every new scan-phase and statistics
  string.
- Document the progress-bar requirement in the design system and record/close the
  drift rows in the reconciliation backlog.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `design-system`: add a requirement that background-progress indicators share a
  single primitive, use semantic tokens (no raw palette literals), render as a
  borderless inset fill when nested inside a `Card`, support both determinate and
  indeterminate states, and show a localized label together with optional
  statistics.
- `gallery-browsing`: add a requirement that `GET /api/status` reports a
  structured, localized scan phase and real progress statistics (files scanned
  out of a known total, plus added, deleted, and updated counts) while a gallery
  scan runs and when it finishes.

## Impact

- **Webapp**: [`AdminGeneralTab.tsx`](../../../webapp/src/components/tabs/AdminGeneralTab.tsx:366)
  (action row + banner), [`AddFolderForm.tsx`](../../../webapp/src/components/settings/AddFolderForm.tsx:20),
  [`ScanProgressBanner.tsx`](../../../webapp/src/components/ScanProgressBanner.tsx:10),
  [`AdminAnalysisTab.tsx`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx:618),
  [`progress.tsx`](../../../webapp/src/components/ui/progress.tsx:5),
  [`useScanStatus.ts`](../../../webapp/src/hooks/useScanStatus.ts:6),
  [`types/index.ts`](../../../webapp/src/types/index.ts:46),
  [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:310) and
  [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:310).
- **Core (Go)**: [`scan_manager.go`](../../../backend/core/internal/application/imaging/scan_manager.go:12)
  and the per-directory scan counting in
  [`scanner.go`](../../../backend/core/internal/application/imaging/scanner.go:114);
  the `GET /api/status` handler and DTO.
- **API contract**: [`core.yaml`](../../../docs/api-contracts/core.yaml) `ScanStatus`
  schema, then `make generate-types` to refresh
  [`api.ts`](../../../webapp/src/types/api.ts). The change is additive — the
  existing `progress` field is retained, so it is **not** a breaking change.
- **Docs**: [`docs/design-system.md`](../../../docs/design-system.md:1) reconciliation
  backlog; [`openspec/specs/design-system/spec.md`](../../../openspec/specs/design-system/spec.md:1)
  is updated through sync.
- **Dependencies / schema**: none added; no database schema change.

## Non-goals

- Restyling the OCR-classification, daily-sync, smart-search backfill, or
  geolocation batch progress bars. They remain as-is and are not part of this
  change; only the gallery scan and Automatic Image Tagging bars are unified.
- Removing or renaming the existing `progress` string field on `/api/status`
  (kept for compatibility; the frontend simply stops rendering it).
- Changing scan triggering, scheduling, hashing, or duplicate-detection behavior.
- Generating the third-party component list or any unrelated settings work.
