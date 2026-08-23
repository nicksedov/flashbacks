# Tasks

## 1. Backend — deleted-file metadata model

- [x] 1.1 Add `TrashItem` to [`backend/api-service/internal/domain/media.go`](../../backend/api-service/internal/domain/media.go) with fields `ID`, `FileName`, `TrashPath`, `OriginalPath`, `Size`, `DeletedAt`, internal `OriginalHash`/`OriginalModTime`, `CreatedAt` and the JSON tags `id/fileName/trashPath/originalPath/size/deletedAt`.
- [x] 1.2 Register `&domain.TrashItem{}` in the `AutoMigrate` call in [`backend/api-service/internal/infrastructure/database/database.go`](../../backend/api-service/internal/infrastructure/database/database.go).
- [x] 1.3 Add a composite index on `trash_items (deleted_at, id)` in `database.go` (mirroring the calendar index), guarded with `IF NOT EXISTS`.
- [x] 1.4 Run `cd backend/api-service && go test ./internal/application/... -count=1`.

## 2. Backend — record trash metadata at move time

- [x] 2.1 In [`backend/api-service/internal/interfaces/handler/helpers/fileops.go`](../../backend/api-service/internal/interfaces/handler/helpers/fileops.go), change `moveToTrash` to return the destination path and to look up the source file's `ImageFile` hash/mod time before the move.
- [x] 2.2 In `MoveToTrashOrDelete` and `BatchProcess`/`BatchProcessWithRules`, insert a `TrashItem` row (with `OriginalPath`, `Size`, `DeletedAt`, `OriginalHash`, `OriginalModTime`) only when the file actually moved to trash, and before `DeleteFromDB`.
- [x] 2.3 Add/update a unit test in `backend/api-service/internal/interfaces/handler/helpers` (or the existing fileops test) that moves a file to a temp trash dir on in-memory SQLite and asserts the `TrashItem` row is created with the correct `OriginalPath` and a non-zero `DeletedAt`.
- [x] 2.4 Run `cd backend/api-service && go test ./internal/application/... -count=1`.

## 3. Backend — DTOs and trash handlers

- [x] 3.1 Add `TrashItemDTO`, `TrashDateGroup`, and `TrashListResponse` to [`backend/api-service/internal/interfaces/dto/media.go`](../../backend/api-service/internal/interfaces/dto/media.go) with camelCase JSON tags matching the spec (`id`, `fileName`, `trashPath`, `originalPath`, `size`, `sizeHuman`, `deletedAt`, `thumbnail`, `groups`, `totalItems`, `totalGroups`, `hasMore`, `nextCursor`).
- [x] 3.2 Rewrite [`handlers_trash.go`](../../backend/api-service/internal/interfaces/handler/handlers_trash.go):
  - `handleListTrash` → `handleGetTrash`: grouped by deletion date, newest → oldest, cursor-paginated, with boundary-date overflow handling and filesystem↔DB reconciliation (backfill missing rows with `originalPath=""` and `deletedAt=mod time`; prune rows whose file is gone).
  - `handleRestoreTrashFile`: `{id}`-based, restores to `originalPath`, re-inserts the `image_files` row, deletes the `TrashItem` row.
  - `handleDeleteTrashFile`: `{id}`-based, removes the file and the row.
  - `handleCleanTrash`: also delete all `TrashItem` rows.
  - Add `handleServeTrashImage` for `GET /api/trash/image` with inline trash-directory verification.
- [x] 3.3 Generate thumbnails for the listed items (via `s.thumbnailBatch`) and embed them into the `TrashItemDTO.Thumbnail` data URL.
- [x] 3.4 Add/update handler tests using in-memory SQLite and a temp trash dir for list grouping/order, restore-to-original-path, permanent delete, and clean.
- [x] 3.5 Run `cd backend/api-service && go test ./internal/application/... -count=1`.

## 4. Backend — i18n messages

- [x] 4.1 Add any new `Msg*` constants (e.g. `MsgTrashItemNotFound`, `MsgTrashRestoreNoOriginalPath`) in [`backend/api-service/internal/interfaces/i18n/messages.go`](../../backend/api-service/internal/interfaces/i18n/messages.go).
- [x] 4.2 Add the matching entries to `backend/api-service/internal/interfaces/i18n/locales/en.json` AND `ru.json` together.
- [x] 4.3 Run `cd backend/api-service && go test ./internal/application/... -count=1`.

## 5. Backend — routes and API contract

- [x] 5.1 Update [`router.go`](../../backend/api-service/internal/interfaces/handler/router.go): register `GET /api/trash`, `GET /api/trash/image`, keep `GET /api/trash-info`, `POST /api/trash-clean`, `POST /api/trash-restore`, `POST /api/trash-delete`; remove `GET /api/trash-list`.
- [ ] 5.2 Update [`docs/api-contracts/api-service.yaml`](../../docs/api-contracts/api-service.yaml) trash endpoints and schemas to match the new contract (grouped response, `{id}` requests, `GET /api/trash/image`).
- [x] 5.3 Run `cd backend/api-service && go build ./... && go test ./internal/application/... -count=1`.

## 6. Webapp — types and API client

- [x] 6.1 Regenerate types with `make generate-types` (or update [`webapp/src/types/index.ts`](../../webapp/src/types/index.ts)): add `TrashItemDTO`, `TrashDateGroup`, `TrashListResponse`; remove obsolete `TrashFileDTO`/`RestoreTrashFileRequest`/`DeleteTrashFileRequest`.
- [x] 6.2 Update [`webapp/src/api/endpoints.ts`](../../webapp/src/api/endpoints.ts): replace `fetchTrashList` with `fetchTrash(cursor?)`, update `restoreTrashFile`/`deleteTrashFile` to send `{id}`, add `buildTrashImageUrl` (or equivalent).
- [x] 6.3 Run `cd webapp && npm run lint && npx tsc -b`.

## 7. Webapp — trash infinite scroll hook

- [x] 7.1 Create [`webapp/src/hooks/useTrashItems.ts`](../../webapp/src/hooks/useTrashItems.ts) wrapping `useCursorInfiniteScroll` with `fetchFn = (cursor) => fetchTrash(cursor)`, `transform = (r) => r.groups`, `responseNextCursor = (r) => r.nextCursor`, `responseTotal = (r) => r.totalItems`, and a `mergeFn` that merges a re-opened boundary date group.
- [x] 7.2 Run `cd webapp && npm run lint && npx tsc -b`.

## 8. Webapp — trash grid, tile, overlay, and lightbox

- [x] 8.1 Create [`webapp/src/components/trash/TrashTile.tsx`](../../webapp/src/components/trash/TrashTile.tsx) reusing `TileFrame`/`TileThumbnail` and a new `TrashTileOverlay` with exactly two buttons: View (`Eye`) and Restore (`RotateCcw`), both `stopPropagation`.
- [x] 8.2 Create [`webapp/src/components/trash/TrashTileGrid.tsx`](../../webapp/src/components/trash/TrashTileGrid.tsx) rendering date-group headers (label + item count) and a responsive grid (same breakpoints as the gallery grid).
- [x] 8.3 Create [`webapp/src/components/trash/TrashLightbox.tsx`](../../webapp/src/components/trash/TrashLightbox.tsx): a `Dialog`-based lightbox showing the full image via `GET /api/trash/image`, the original location (`originalPath`), and the deletion date.
- [x] 8.4 Run `cd webapp && npm run lint && npx tsc -b`.

## 9. Webapp — rewrite TrashTab

- [x] 9.1 Rewrite [`webapp/src/components/tabs/TrashTab.tsx`](../../webapp/src/components/tabs/TrashTab.tsx): `ViewHeader` (item count), "Empty Trash" action with `ConfirmDialog`, `TrashTileGrid` + intersection-observer sentinel + `PaginationFooter`, lightbox state, and per-tile restore with optimistic removal.
- [x] 9.2 Run `cd webapp && npm run lint && npx tsc -b`.

## 10. Webapp — i18n

- [x] 10.1 Add the new Trash strings (view, restore, original location, deleted date, empty state, toasts) to [`webapp/src/i18n/translations.en.ts`](../../webapp/src/i18n/translations.en.ts) AND [`webapp/src/i18n/translations.ru.ts`](../../webapp/src/i18n/translations.ru.ts) together.
- [x] 10.2 Run `cd webapp && npm run lint && npx tsc -b && npm test` (the i18n parity test must pass).

## 11. Final verification

- [x] 11.1 Run `cd backend/api-service && go test ./internal/application/... -count=1`.
- [x] 11.2 Run `cd webapp && npm run lint && npx tsc -b && npm test`.
- [ ] 11.3 Manually verify: trash shows thumbnails grouped by deletion date (latest first), infinite scroll loads more, View opens the lightbox with original path + deletion date, Restore returns the file to its original location and the file reappears in the gallery.
