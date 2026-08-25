## Context

The Trash today is filesystem-only: [`handlers_trash.go`](../../backend/api-service/internal/interfaces/handler/handlers_trash.go) reads the configured `trashDir` with `os.ReadDir` for every request and returns `fileName/size/sizeHuman/modTime` — no persisted deletion date, no original path, no thumbnail. Deletion flows run through [`helpers.FileMover`](../../backend/api-service/internal/interfaces/handler/helpers/fileops.go), which moves a file into the trash directory (suffixing on name collision) and then removes the `image_files` row via `DeleteFromDB` — the original location is discarded at that moment. The frontend [`TrashTab.tsx`](../../webapp/src/components/tabs/TrashTab.tsx) renders this as a plain table with restore/delete icons and a "clean all" action.

The Gallery is the reference implementation: [`GalleryAllImagesView.tsx`](../../webapp/src/components/gallery/GalleryAllImagesView.tsx) drives [`useGalleryImages`](../../webapp/src/hooks/useGalleryImages.ts) → [`useInfiniteScroll`](../../webapp/src/hooks/useInfiniteScroll.ts) → [`GalleryImageGrid`](../../webapp/src/components/gallery/GalleryImageGrid.tsx) → [`ImageTile`](../../webapp/src/components/gallery/ImageTile.tsx) over [`TileFrame`/`TileThumbnail`](../../webapp/src/components/gallery/TileFrame.tsx) + [`TileOverlay`](../../webapp/src/components/gallery/TileOverlay.tsx); the calendar view ([`GalleryCalendarView`](../../webapp/src/components/gallery/GalleryCalendarView.tsx)) additionally demonstrates date-grouping with cursor pagination via [`useCursorInfiniteScroll`](../../webapp/src/hooks/useCursorInfiniteScroll.ts). Thumbnails are produced server-side and embedded into DTOs as data URLs.

## Goals / Non-Goals

**Goals:**

- Persist deleted-file metadata (deletion date + restore path) so the Trash can be rendered like the Gallery and restore is reliable.
- Rebuild the Trash frontend to match the Gallery: thumbnail grid, date grouping (latest → earliest), infinite scroll, hover overlay with exactly View and Restore.
- View opens a lightbox showing the full image, the original pre-deletion path, and the deletion date.
- Restore moves the file back to its original location and makes it reappear in the gallery.

**Non-Goals:**

- No changes to how files are deleted into trash (only the recording step is added).
- No multi-select/search/folder grouping in Trash in this change.
- No redesign of the Gallery primitives themselves.

## Decisions

### D1 — `TrashItem` domain model and table

Add to [`domain/media.go`](../../backend/api-service/internal/domain/media.go):

```go
type TrashItem struct {
    ID              uint      `gorm:"primaryKey" json:"id"`
    FileName        string    `gorm:"not null" json:"fileName"`
    TrashPath       string    `gorm:"uniqueIndex;not null" json:"trashPath"`
    OriginalPath    string    `gorm:"not null" json:"originalPath"`
    Size            int64     `gorm:"not null" json:"size"`
    DeletedAt       time.Time `gorm:"index;not null" json:"deletedAt"`
    OriginalHash    string    `json:"-"` // internal: re-index gallery on restore
    OriginalModTime time.Time `json:"-"` // internal: re-index gallery on restore
    CreatedAt       time.Time `json:"createdAt"`
}
```

`OriginalHash`/`OriginalModTime` are internal columns (not in the public DTO) captured at delete time so restore can re-insert the `image_files` row without recomputing the hash. Register the model in [`database.go`](../../backend/api-service/internal/infrastructure/database/database.go) `AutoMigrate` and add an index on `deleted_at` (an explicit composite index on `(deleted_at, id)` mirrors the calendar pagination index if needed).

*Alternatives considered:* keeping the filesystem as the source of truth and only adding a sidecar metadata file per trash file (rejected — no queryable ordering/grouping and no reliable restore path); a fully separate trash service (rejected — unnecessary; trash is a small surface in api-service).

### D2 — Record at move time, inside `FileMover`

`FileMover` already holds `*gorm.DB` and is injected via `ServerDeps`, so recording needs no DI change. Change `moveToTrash` to capture the source path, size, and the pre-deletion `ImageFile` hash/mod time (read from DB before `DeleteFromDB`), perform the move, and insert a `TrashItem{FileName, TrashPath, OriginalPath, Size, DeletedAt, OriginalHash, OriginalModTime}`. `MoveToTrashOrDelete` and `BatchProcess`/`BatchProcessWithRules` record the row only when the file actually moved to trash (not when permanently deleted).

*Alternatives considered:* recording in each handler that calls the mover (rejected — scattered and easy to miss a call site); recording in a domain service wired via Wire (rejected — `FileMover` is the single choke point already used by all delete flows).

### D3 — Endpoint contract

- `GET /api/trash` (new, replaces `GET /api/trash-list`) returns `TrashListResponse{groups, totalItems, totalGroups, hasMore, nextCursor}` grouped by deletion date, ordered newest → oldest.
- `POST /api/trash-restore` request changes from `{fileName, targetPath}` to `{id}`; restores to `originalPath` and deletes the row.
- `POST /api/trash-delete` request changes from `{fileName}` to `{id}`.
- `GET /api/trash-info` and `POST /api/trash-clean` remain; clean now also deletes all `TrashItem` rows.
- `GET /api/trash/image?path=...` serves the full-size image for the lightbox, verified against the configured trash directory.

New DTOs live in [`dto/media.go`](../../backend/api-service/internal/interfaces/dto/media.go): `TrashItemDTO{id, fileName, trashPath, originalPath, size, sizeHuman, deletedAt, thumbnail}`, `TrashDateGroup{date, label, itemCount, items}`, `TrashListResponse{groups, totalItems, totalGroups, hasMore, nextCursor}`.

*Alternatives considered:* keeping `fileName` keys for restore/delete (rejected — trash file names are suffixed on collision, so `fileName` is ambiguous; `id` is stable and unique).

### D4 — Date grouping and cursor pagination

Reuse the calendar view's approach. Sort by `deleted_at DESC, id DESC`; group rows by `deleted_at` calendar day into date groups in the order encountered; paginate with a cursor over `(deletedAt, id)` and, when a page boundary splits a date, fetch the remaining items of that date so a group is never partially rendered (the same overflow handling the calendar handler performs). `nextCursor` is base64-encoded `deletedAt` + `id`.

### D5 — Thumbnails embedded as data URLs

Trash thumbnails are generated server-side with the existing `thumbnailProvider`/`thumbnailBatch` and embedded into each `TrashItemDTO` as a data URL, exactly like the gallery grid (which embeds thumbnails in DTOs). No new thumbnail serving endpoint is needed. A failed thumbnail yields an empty string and the tile shows a placeholder.

### D6 — Trash image serving with inline verification

`GET /api/trash/image` normalizes the requested path, resolves the configured `trashDir` from `AppSettings`, and verifies the path is inside the trash directory (same `pathConflict` logic used for folder/trash checks) before calling `c.File`. Verification is done inline in the handler using `settingsLoader` — no new DI type, so `wire_gen.go` regeneration is not required.

### D7 — Restore re-indexes the gallery

Restore moves `trashPath` → `originalPath` (recreating directories and suffixing on conflict), deletes the `TrashItem` row, and re-inserts the `image_files` row using `OriginalHash`/`OriginalModTime` (falling back to a fast scan of the restored directory if those are empty, e.g. for legacy entries). This makes the restored image reappear in the gallery immediately.

*Alternatives considered:* only moving the file and waiting for a background/manual rescan (rejected — restored files would silently disappear from the gallery); storing the full `ImageFile` as JSON in the trash row (rejected — coupling).

### D8 — Frontend architecture

- New `useTrashItems` hook wrapping [`useCursorInfiniteScroll`](../../webapp/src/hooks/useCursorInfiniteScroll.ts) with `fetchFn = (cursor) => fetchTrash(cursor)`, `transform = (r) => r.groups`, and a `mergeFn` that merges a boundary date group if the next page re-opens the same date.
- New `TrashTileGrid` + `TrashTile` components: reuse [`TileFrame`/`TileThumbnail`](../../webapp/src/components/gallery/TileFrame.tsx) for the tile shell and add a dedicated `TrashTileOverlay` with exactly two buttons — View (`Eye` icon) and Restore (`RotateCcw` icon) — replacing the download/delete overlay.
- New `TrashLightbox` (a `Dialog`-based component, not the full AI/EXIF `UnifiedLightbox`) that renders the image via `GET /api/trash/image`, plus an info panel showing the original location and the deletion date.
- [`TrashTab.tsx`](../../webapp/src/components/tabs/TrashTab.tsx) keeps a header with a `ViewHeader` (count) and an "Empty Trash" `Button` (confirmed by a `ConfirmDialog`), then renders `TrashTileGrid` with an intersection-observer sentinel and a `PaginationFooter`, and manages the lightbox and per-tile restore.
- The overlay View/Restore handlers call `stopPropagation` like the existing [`TileOverlay`](../../webapp/src/components/gallery/TileOverlay.tsx).

### D9 — i18n and contract/type refresh

New `TranslationKey` entries in [`translations.en.ts`](../../webapp/src/i18n/translations.en.ts) and [`translations.ru.ts`](../../webapp/src/i18n/translations.ru.ts) (view, restore, original location, deleted date, empty state, toasts). New backend `Msg*` constants plus en/ru locale entries in [`interfaces/i18n/locales`](../../backend/api-service/internal/interfaces/i18n/locales/en.json). The OpenAPI contract at [`docs/api-contracts/api-service.yaml`](../../docs/api-contracts/api-service.yaml) is updated to match the new endpoints (it currently diverges from the Go handlers), and `make generate-types` refreshes the webapp TS types.

## Risks / Trade-offs

- **Restore re-indexing can desync if the directory changed** → Mitigation: re-insert uses the captured hash/mod time and creates the directory; if the path conflicts with a newer file, the conflict suffix is applied and the restored path is returned to the caller.
- **Legacy trash files have no metadata** → Mitigation: `GET /api/trash` reconciles the trash directory with `trash_items` — files present on disk but missing rows are backfilled with `originalPath=""`, `deletedAt=file mod time`, and empty hash/mod time; rows whose file vanished are pruned. Restore for `originalPath==""` returns a clear error.
- **Embedding thumbnails for large pages** → Mitigation: page size stays at the gallery default (~50) and thumbnails are generated in parallel via the existing batch helper.
- **Contract churn on restore/delete** → Mitigation: the only consumer of these endpoints is the webapp, which is updated in the same change; the OpenAPI contract and generated TS types are refreshed together.

## Migration Plan

1. Add `TrashItem` + migration; record rows in `FileMover` at move time.
2. Rework trash handlers (grouped list, id-based restore/delete, clean, image serving) and DTOs; add `Msg*` + en/ru.
3. Update OpenAPI contract and regenerate TS types; add webapp `fetchTrash`/types.
4. Build `useTrashItems` hook and the `TrashTileGrid`/`TrashTile`/`TrashTileOverlay`/`TrashLightbox` components; rewrite `TrashTab`.
5. Add en/ru translations; run backend `go test ./internal/application/... -count=1` and webapp `npm run lint && npx tsc -b` + `npm test` after each step.
6. Rollback: each step is an independent revertable commit; the only schema change is additive (`trash_items`), so rollback is a plain `git revert` (dropping the table via migration is optional cleanup).

## Open Questions

<!-- none -->
