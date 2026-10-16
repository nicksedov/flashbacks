# Plan — Gallery Folders settings polish + unified scan progress

SDD change covering: (1) the Gallery Folders settings action row, (2) the scan
progress banner, (3) the Automatic Image Tagging progress bar, and (4) progress
bar requirements in the design system.

## Findings (current state)

| Concern | Location | Problem |
|---|---|---|
| Add-folder button style | [`AddFolderForm.tsx`](webapp/src/components/settings/AddFolderForm.tsx:43) | `Button` with default (filled) variant, `Plus` icon, rendered top-right in the card header |
| Scan buttons style | [`AdminGeneralTab.tsx`](webapp/src/components/tabs/AdminGeneralTab.tsx:386) | `Button variant="outline" size="sm"`, rendered in a separate row next to the folder-count caption |
| Scan progress banner | [`ScanProgressBanner.tsx`](webapp/src/components/ScanProgressBanner.tsx:16) | Raw blue palette (`bg-blue-50`, `border-blue-200`, `text-blue-800`…) + bordered container inside a `Card`; violates the token rule and the inset-fill rule |
| Scan progress text | [`scan_manager.go`](backend/core/internal/application/imaging/scan_manager.go:93) | `progress` is a free-text English string (`"Cleaning up missing files..."`, `"Scanning: <dir>"`, `"Processed: <path>"`) — not localized |
| Scan counter | [`scan_manager.go`](backend/core/internal/application/imaging/scan_manager.go:101) | `filesProcessed` counts progress-channel **messages**, not files → always misleading ("0 files processed") |
| Tag-scan progress | [`AdminAnalysisTab.tsx`](webapp/src/components/tabs/AdminAnalysisTab.tsx:639) | Ad-hoc raw `<div>` bar (`bg-muted`/`bg-primary`), duplicated design |
| Progress primitive | [`progress.tsx`](webapp/src/components/ui/progress.tsx:5) | Radix `Progress` exists but has no label/stats wrapper; `value=undefined` = indeterminate |

## Design decisions

### 1. Gallery Folders action row (one row, consistent styles)

- Move the `AddFolderForm` trigger out of the card header and into a single
  action row together with "Scan Changes" and "Rescan All".
- Order: **Add folder** (primary, `default` variant) → **Scan Changes**
  (`outline`) → **Rescan All** (`outline`), all `size="sm"`.
- Keep the folder-count caption (`{n} folder(s) in gallery`) above the folder
  list, not in the action row.
- This applies the existing design-system rules ("Single button primitive",
  "Consistent primary action placement") — no new spec requirement needed, only
  a drift row in `docs/design-system.md`.

### 2. Shared progress component

- Add a composed component (e.g. `ProgressIndicator` in
  [`progress.tsx`](webapp/src/components/ui/progress.tsx)) that wraps the Radix
  `Progress` bar plus a label and an optional status/stats line.
- Props: `value?: number` (undefined = indeterminate), `label: string`,
  `statusText: string`, optional `stats?: { processed: number; total: number; added?: number; deleted?: number }`.
- Styling: borderless `bg-muted` inset fill (shared radius token), semantic
  tokens only (`bg-primary`, `text-muted-foreground`) — no raw palette, no
  `border`. This satisfies the "Nested areas use inset fill" rule.
- Consumers in this change: `ScanProgressBanner` (gallery scan) and the
  tag-scan progress in [`AdminAnalysisTab.tsx`](webapp/src/components/tabs/AdminAnalysisTab.tsx:639).
- Non-goal (backlog follow-up): OCR classification, daily-sync, smart-search
  backfill, and geo batch progress bars remain as-is for now.

### 3. Scan status — real, localized statistics

`GET /api/status` response extended (additive, non-breaking — keep `progress`
for compatibility but stop rendering it):

```
scanning: bool
phase: "cleanup" | "scanning" | "complete"   // NEW — localized on the frontend
currentDirectory: string                     // NEW — informational
progress: string                             // kept, deprecated for display
totalFiles: int                              // NEW — total image files in gallery dirs
filesProcessed: int                          // semantics changed → actual files scanned
filesAdded: int                              // NEW
filesDeleted: int                            // NEW
filesUpdated: int                            // NEW
```

Backend work in [`scan_manager.go`](backend/core/internal/application/imaging/scan_manager.go):
- Replace the string-only progress channel with structured counters, or
  accumulate real counts from `ScanBatchResult` (`Created`/`Updated`/`Deleted`)
  and `FastScanResult` (`Unchanged`/`Modified`/`Created`/`Deleted`).
- Compute `totalFiles` via a per-directory image-file pre-count before scanning
  (one extra `filepath.Walk`) so the frontend can render
  "Scanned: X of Y".
- Set `phase` and `currentDirectory` as the scan moves through
  cleanup → per-directory scan → complete.

### 4. Localization

New i18n keys (en + ru together) in
[`translations.en.ts`](webapp/src/i18n/translations.en.ts) and
[`translations.ru.ts`](webapp/src/i18n/translations.ru.ts):
- scan phases: cleanup / scanning / complete.
- stats line: `Scanned: {processed} of {total} · added {added} · deleted {deleted}`
  (ru: `Просканировано: {processed} из {total} · добавлено {added} · удалено {deleted}`).

### 5. API contract + type alignment

- Update `ScanStatus` schema in [`core.yaml`](docs/api-contracts/core.yaml),
  regenerate [`api.ts`](webapp/src/types/api.ts) via `make generate-types`.
- Align [`ScanStatusResponse`](webapp/src/types/index.ts:46) and fix the
  `running` vs `scanning` drift (the generated `ScanStatus` currently has
  `running`, the backend actually returns `scanning`).
- Update `useScanStatus` default status accordingly.

### 6. Design system

- Add a new `design-system` requirement "Unified progress indicator" to
  [`openspec/specs/design-system/spec.md`](openspec/specs/design-system/spec.md)
  (delta spec, merged at sync).
- Record drift rows in [`docs/design-system.md`](docs/design-system.md) for the
  progress bars and the gallery-folders action-row style mismatch; mark them
  done during apply.

## Capability mapping for the delta specs

- `design-system` → ADDED requirement: unified progress indicator (token-based,
  inset fill, determinate + indeterminate, localized label + stats).
- `gallery-browsing` → MODIFIED/ADDED requirement: scan status reports
  structured, localized progress statistics.

## Flow (simplified)

```mermaid
flowchart LR
  A[Scan triggered] --> B[ScanManager sets phase cleanup]
  B --> C[Pre-count total files per dir]
  C --> D[Scan each dir, accumulate added/updated/deleted]
  D --> E[phase complete, isScanning false]
  D --> F[/api/status returns structured counters]
  F --> G[ProgressIndicator renders localized stats]
```
