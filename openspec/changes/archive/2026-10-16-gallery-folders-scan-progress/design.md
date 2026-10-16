# Design

## Context

See [`proposal.md`](proposal.md) for motivation.

Current state that shapes the approach:

- The Gallery Folders card in
  [`AdminGeneralTab.tsx`](../../../webapp/src/components/tabs/AdminGeneralTab.tsx:366)
  renders the `AddFolderForm` trigger in the `CardHeader` (filled primary
  `Button`) and the two scan buttons in a separate row (`variant="outline"`).
- [`ScanProgressBanner.tsx`](../../../webapp/src/components/ScanProgressBanner.tsx:10)
  hand-rolls a bordered blue panel (`border-blue-200 bg-blue-50 …
  dark:border-blue-800 dark:bg-blue-950`) with a Radix `Progress` bar and renders
  `status.progress` (a free-text English sentence) plus
  `scanProgress.filesProcessed`.
- [`AdminAnalysisTab.tsx`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx:639)
  renders the Automatic Image Tagging progress as ad-hoc `<div>`s and shows
  `{scanned} of {total} images`.
- [`ScanManager`](../../../backend/core/internal/application/imaging/scan_manager.go:49)
  exposes `ScanStatusResponse{Scanning, Progress string, FilesProcessed int}`.
  `Progress` is English free text (`"Cleaning up missing files..."`,
  `"Scanning: <dir>"`, `"Scan complete"`); `FilesProcessed` counts
  progress-channel **messages** (including `"Skipped (cached): …"` and error
  lines), not files. Real counts exist in `ScanBatchResult`
  (`Created`/`Updated`/`Deleted`) and `FastScanResult`
  (`Unchanged`/`Modified`/`Created`/`Deleted`/`TotalChecked`).
- The generated `ScanStatus` schema in
  [`api.ts`](../../../webapp/src/types/api.ts:1688) uses `running`, while the
  backend and the hand-written
  [`ScanStatusResponse`](../../../webapp/src/types/index.ts:46) use `scanning` —
  existing contract drift to fix here.

## Goals / Non-Goals

**Goals:**

- One reusable, token-based progress indicator used by both the gallery scan
  banner and the Automatic Image Tagging progress.
- A single consistent action row for the Gallery Folders add/scan controls.
- A structured, localized scan status (`/api/status`) with real file counters.
- Keep the change additive and low-risk for the API.

**Non-Goals:**

- Changing scan scheduling, hashing, or duplicate-detection logic.
- Restyling the OCR, daily-sync, smart-search, or geolocation progress bars.
- Persisting scan statistics in the database (status stays in-memory).

## Decisions

### D1 — One composed `ProgressIndicator` beside the existing `Progress` primitive

Add a composed component in
[`progress.tsx`](../../../webapp/src/components/ui/progress.tsx:5) that wraps the
Radix `Progress` bar plus an optional label and statistics line. Props:
`value?: number` (undefined ⇒ indeterminate), `label: string`,
`statusText?: string`, and `stats?: { processed, total, added?, deleted? }`.

- *Alternative considered:* keep `ScanProgressBanner` and add a second bespoke
  bar for tagging. Rejected: this is exactly the duplicated-control drift the
  design system forbids.
- The component renders as a borderless `bg-muted` inset fill with
  `text-muted-foreground`; it never uses raw palette literals. This satisfies the
  new design-system requirement and the existing "inset fill, not nested borders"
  rule.

### D2 — Gallery Folders actions in one row

Move the `AddFolderForm` trigger out of `CardHeader` into a single action row
alongside the scan buttons, ordered **Add folder** (primary `default`) → **Scan
Changes** (`outline`) → **Rescan All** (`outline`), all `size="sm"`. The folder
count moves to its own caption line above the list. This applies the existing
design-system rules (single button primitive, consistent primary action
placement); no new requirement is needed, only a backlog row.

### D3 — Structured scan phase, localize on the frontend

Replace the display of the free-text `progress` with a machine `phase`
(`cleanup` | `scanning` | `complete`) plus a `currentDirectory` string. The
frontend maps the phase to an `en`/`ru` i18n key; the directory is data, not
UI text.

- *Alternative considered:* translate `progress` in the Go backend. Rejected: the
  backend cannot know the webapp locale, and free text cannot be counted or
  asserted on.

### D4 — `totalFiles` from a per-directory pre-count

Before scanning each gallery directory, walk it once to count image files and sum
the result into `totalFiles`. This gives a real denominator for "scanned X of Y".

- *Alternative considered:* count files as they are processed and set the total
  only at the end. Rejected: the denominator would be unknown for the whole scan.
- *Trade-off:* one extra directory walk per scan. The walk is metadata-only
  (no hashing) and scans are already I/O bound, so the cost is acceptable.

### D5 — Counters from batch results, not messages

Derive `filesProcessed`, `filesAdded`, `filesDeleted`, and `filesUpdated` from
the `ScanBatchResult`/`FastScanResult` counts the manager already receives, plus
the per-file processed total. Stop using the progress-message counter for
`filesProcessed`.

- *Alternative considered:* keep counting progress messages. Rejected: it is the
  root cause of the "0 files processed" bug.

### D6 — Additive API change

Keep `progress` (deprecated for display) and add `phase`, `currentDirectory`,
`totalFiles`, `filesAdded`, `filesDeleted`, `filesUpdated`; correct the semantics
of `filesProcessed` (same field name and type). Update the `ScanStatus` schema in
[`core.yaml`](../../../docs/api-contracts/core.yaml) and run
`make generate-types`. While there, fix the generated `running` → `scanning`
drift so the contract matches the backend and
[`types/index.ts`](../../../webapp/src/types/index.ts:46).

### D7 — No DI or schema migration

`ScanManager` is already wired; no new dependency is added, so `wire_gen.go` does
not change. Scan status remains in-memory (no database change).

## Intended evidence (recorded for the apply phase)

| Scenario ID | Runner | Intended selector |
|---|---|---|
| `design-system.progress-indicator-shared` | `vitest` | `webapp/src/components/ui/progress.test.tsx::ProgressIndicator` — both consumers render the shared primitive |
| `design-system.progress-indicator-indeterminate` | `vitest` | `webapp/src/components/ui/progress.test.tsx::ProgressIndicator` — no value ⇒ indeterminate bar |
| `design-system.progress-indicator-localized` | `vitest` | `webapp/src/components/ui/progress.test.tsx::ProgressIndicator` — ru locale shows the Russian label |
| `design-system.progress-indicator-token-based` | `vitest` | `webapp/src/components/ui/progress.test.tsx::ProgressIndicator` — semantic tokens, no raw palette |
| `design-system.progress-indicator-inset-fill` | `vitest` | `webapp/src/components/ui/progress.test.tsx::ProgressIndicator` — no border utility inside a Card |
| `gallery-browsing.scan-status-phase` | `go` | `./internal/application/imaging::TestScanManager_StatusPhase` |
| `gallery-browsing.scan-status-total-files` | `go` | `./internal/application/imaging::TestScanManager_StatusTotalFiles` |
| `gallery-browsing.scan-status-file-counters` | `go` | `./internal/application/imaging::TestScanManager_StatusFileCounters` |
| `gallery-browsing.scan-status-files-processed` | `go` | `./internal/application/imaging::TestScanManager_StatusFilesProcessed` |

Evidence registry entries (`*ID*: evidence: [...]`) are added to
[`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) during
apply, not at propose time.

## Risks / Trade-offs

- [Extra directory walk per scan] → Pre-count is metadata-only and bounded by the
  gallery size; it runs once per scan, not per file.
- [Deterministic Go tests around async scanning] → Test the counters through the
  synchronous helpers (`scanDirectory`, `processBatchResult`) or await scan
  completion via the existing `OnScanComplete` hook rather than sleeping.
- [`filesProcessed` semantics change could surprise a consumer] → It is only read
  by the webapp, which is updated in the same change; the field name and type are
  unchanged.
- [Raw-color regression in a new component] → A unit test asserts the absence of
  raw palette classes (e.g. `bg-blue-`), matching the design-system requirement.

## Migration Plan

Frontend and backend ship together on the same feature branch. The API change is
additive, so an older frontend keeps working (it simply ignores the new fields).
No database migration and no rollback-specific steps are required; reverting the
branch restores the previous banner.

## Open Questions

- None that affect the specs or tasks. The exact statistics-line wording is a
  copy detail settled during implementation with both `en` and `ru` present.
