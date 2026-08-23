## Why

The Trash (Корзина) section is currently a plain table of file names, sizes, and two small action icons. It does not render image thumbnails, does not group deleted files by deletion date, does not support infinite scroll, and cannot tell the user where a file lived before it was deleted — the backend only scans the trash directory on demand and does not persist any metadata about deleted files.

This makes the Trash feel disconnected from the rest of the product and makes "restore to original location" unreliable: the current restore handler falls back to the process working directory when no target path is supplied, because the original location is never stored.

This change rebuilds the Trash section to be visually and structurally uniform with the Gallery (thumbnail grid, date grouping, infinite scroll, tile hover overlay), and adds persistent backend storage for deleted-file metadata (deletion date and the restore path) plus the endpoints that the new UI needs.

## What Changes

- **New `trash` capability** defining the required behavior of the Trash section and the backend trash operations.
- **Backend — deleted-file metadata storage**: add a `TrashItem` domain model (table `trash_items`) storing, per deleted file: the file name, the current path inside the trash directory, the original path before deletion (the restore path), the size, and the deletion timestamp. Record a row when a file is moved to trash, and remove the row when the file is restored or permanently deleted.
- **Backend — endpoint changes**: replace the ungrouped `GET /api/trash-list` with a grouped, cursor-paginated `GET /api/trash` that returns trash items grouped by deletion date and sorted from the latest date to the earliest; switch `POST /api/trash-restore` and `POST /api/trash-delete` from `fileName`-based to `id`-based operations; keep `GET /api/trash-info` and `POST /api/trash-clean` (now also clearing `trash_items` rows); add `GET /api/trash/image` to serve a full-size preview of a trashed file for the lightbox (verified against the configured trash directory, mirroring gallery access checks).
- **Backend — thumbnails**: generate and embed thumbnails into trash list items (as data URLs), exactly like the gallery views do, so the grid shows image thumbnails.
- **Frontend — Trash UI**: rewrite [`TrashTab.tsx`](../../webapp/src/components/tabs/TrashTab.tsx) as a gallery-style view: a responsive thumbnail grid grouped by deletion date (latest first), infinite scroll (cursor-based), and a two-button hover overlay — **View** (opens a lightbox showing the full image plus the original location before deletion and the deletion date) and **Restore** (restores the file to its original location). A view-level "Empty Trash" action remains available in the header.
- **API contract & types**: update the OpenAPI contract at [`docs/api-contracts/api-service.yaml`](../../docs/api-contracts/api-service.yaml) for the trash endpoints, refresh the webapp TS types, and add en/ru translations for all new strings.

## Capabilities

### New Capabilities

- `trash`: The deleted-files (Trash) capability — persistent backend storage of deleted-file metadata (deletion date, restore path), the trash REST endpoints, and the frontend Trash view (thumbnail grid grouped by deletion date, infinite scroll, view/restore overlay, restore-to-original-location).

### Modified Capabilities

- `design-system`: The Trash view is rebuilt to consume the design-system primitives (tile overlay, view header, empty state, pagination footer, grid/responsive breakpoints) already specified in the design-system capability.

## Impact

- Backend api-service: new [`TrashItem`](../../backend/api-service/internal/domain/media.go) model, migration registration in [`database.go`](../../backend/api-service/internal/infrastructure/database/database.go), trash handler rework in [`handlers_trash.go`](../../backend/api-service/internal/interfaces/handler/handlers_trash.go), trash recording in [`fileops.go`](../../backend/api-service/internal/interfaces/handler/helpers/fileops.go), a trash access verifier, DTOs in [`media.go`](../../backend/api-service/internal/interfaces/dto/media.go), routes in [`router.go`](../../backend/api-service/internal/interfaces/handler/router.go), and i18n `Msg*` constants with en/ru entries.
- Webapp: [`TrashTab.tsx`](../../webapp/src/components/tabs/TrashTab.tsx) rewritten; new trash API/hook/types; a trash lightbox component; a trash tile grid component reusing gallery primitives; new i18n keys in en/ru.
- Documentation/contracts: [`docs/api-contracts/api-service.yaml`](../../docs/api-contracts/api-service.yaml) updated; TS types refreshed via `make generate-types`.
- No changes to the exif/ocr services, and no new third-party dependencies.

## Non-goals

- Not changing the delete flows that move files into trash (Gallery, Smart Search, duplicates, batch dedup) beyond recording the `TrashItem` row at move time.
- Not changing how the trash directory itself is configured (the existing `trashDir` setting stays).
- Not a redesign of the Gallery; the Trash view reuses existing gallery primitives rather than introducing new ones.
- Not adding multi-select, search, or folder grouping to Trash in this change — only deletion-date grouping, infinite scroll, view, and restore.
- Not migrating existing trash files' original locations (impossible to recover); existing files are reconciled with fallback metadata where the original path is unknown.
