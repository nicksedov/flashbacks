# Trash Specification

## Purpose

Defines the deleted-files (Trash) capability: persistent backend storage of deleted-file metadata, the trash REST endpoints, and the frontend Trash view. The Trash view SHALL be structurally uniform with the Gallery: thumbnail grid, grouping by deletion date, infinite scroll, and a tile hover overlay with view/restore actions.

## Requirements

### Requirement: Persistent deleted-file metadata

The api-service SHALL persist a `TrashItem` record for every file moved to the trash directory. The record SHALL store:

- `id` — unique identifier
- `fileName` — the file name inside the trash directory
- `trashPath` — the current absolute path inside the trash directory (unique)
- `originalPath` — the file's absolute path before deletion (the restore path)
- `size` — file size in bytes
- `deletedAt` — the deletion timestamp

The JSON representation SHALL use camelCase field names exactly: `id`, `fileName`, `trashPath`, `originalPath`, `size`, `sizeHuman`, `deletedAt`.

#### Scenario: File moved to trash is recorded

- **WHEN** a file is moved to the trash directory via any delete flow (Gallery, Smart Search, duplicates, batch dedup)
- **THEN** a `TrashItem` row is created with `originalPath` equal to the file's path before deletion and `deletedAt` set to the current time

#### Scenario: Restored file is unrecorded

- **WHEN** a trashed file is restored
- **THEN** its `TrashItem` row is deleted

#### Scenario: Permanently deleted file is unrecorded

- **WHEN** a trashed file is permanently deleted
- **THEN** its `TrashItem` row is deleted

### Requirement: Trash list grouped by deletion date

`GET /api/trash` SHALL return trash items grouped by deletion date (calendar day of `deletedAt`), ordered from the latest date to the earliest date. Within a group, items SHALL be ordered by `deletedAt` descending.

The response SHALL be:

```json
{
  "groups": [
    {
      "date": "YYYY-MM-DD",
      "label": "human-readable label",
      "itemCount": 3,
      "items": [
        {
          "id": 1,
          "fileName": "photo.jpg",
          "trashPath": "/trash/photo.jpg",
          "originalPath": "/gallery/2024/photo.jpg",
          "size": 1024,
          "sizeHuman": "1 KB",
          "deletedAt": "2026-08-22T18:00:00+03:00",
          "thumbnail": "data:image/webp;base64,..."
        }
      ]
    }
  ],
  "totalItems": 3,
  "totalGroups": 1,
  "hasMore": false,
  "nextCursor": null
}
```

#### Scenario: Latest deletion date appears first

- **WHEN** the trash contains files deleted on different days
- **THEN** the response groups are ordered from the most recent deletion date to the oldest, and each group's items are ordered by deletion time descending

#### Scenario: Cursor pagination over date groups

- **WHEN** there are more groups than fit in one page
- **THEN** the response includes a non-null `nextCursor` and `hasMore: true`; passing the cursor to the next request returns the following date groups without duplicating the boundary date

#### Scenario: Empty trash

- **WHEN** the trash is empty
- **THEN** `GET /api/trash` returns an empty `groups` array, `totalItems: 0`, `totalGroups: 0`, `hasMore: false`, and `nextCursor: null`

### Requirement: Trash thumbnails

The trash list endpoint SHALL include a server-generated thumbnail (data URL) for each item, generated from the file's current trash path, matching the gallery thumbnail approach. Items whose thumbnail cannot be generated SHALL have an empty `thumbnail` and still be listed.

#### Scenario: Trash grid shows image thumbnails

- **WHEN** the Trash view renders
- **THEN** each tile displays the image thumbnail when available, and a placeholder when the thumbnail is empty

### Requirement: Restore to original location

`POST /api/trash-restore` SHALL accept `{ "id": number }` and restore the identified file to its `originalPath`, recreating the directory if needed and handling name conflicts by suffixing. On success it SHALL delete the `TrashItem` row and return `{ "success": true, "restoredPath": "..." }`.

#### Scenario: Restore returns file to pre-deletion path

- **WHEN** a user restores a trashed file that had `originalPath` `/gallery/2024/photo.jpg`
- **THEN** the file is moved back to `/gallery/2024/photo.jpg` and the `TrashItem` row is removed

#### Scenario: Restore with unknown original path

- **WHEN** a trashed file has an empty `originalPath` (legacy entry)
- **THEN** the endpoint returns a clear error and does not move the file to an arbitrary location

### Requirement: Permanent delete by id

`POST /api/trash-delete` SHALL accept `{ "id": number }`, permanently remove the identified file from the trash directory, and delete its `TrashItem` row.

#### Scenario: Permanent delete removes file and record

- **WHEN** a user permanently deletes a trashed file
- **THEN** the file is removed from disk and its `TrashItem` row is deleted

### Requirement: Clean trash clears metadata

`POST /api/trash-clean` SHALL remove all files from the trash directory and delete all `TrashItem` rows, returning `{ "deleted": n, "failed": m }`.

#### Scenario: Clean trash removes everything

- **WHEN** a user empties the trash
- **THEN** all trash files are removed and the `trash_items` table is emptied

### Requirement: Trash image serving for the lightbox

`GET /api/trash/image` SHALL serve the full-size image for a trashed file. The endpoint SHALL verify that the requested path resolves inside the configured trash directory and is not outside it (mirroring gallery access verification), and SHALL reject paths outside the trash directory.

#### Scenario: View opens full-size trashed image

- **WHEN** a user opens a trashed file in the lightbox
- **THEN** the full-size image is served from the trash directory

#### Scenario: Path outside trash is rejected

- **WHEN** `GET /api/trash/image` is called with a path outside the configured trash directory
- **THEN** the request is rejected with an error

### Requirement: Trash view uniformity with the gallery

The frontend Trash view SHALL render a responsive thumbnail grid that follows the gallery layout: date-group headers with a human-readable label and item count, followed by a responsive grid of image tiles. Each tile SHALL use the shared gallery tile primitives (`TileFrame`, `TileThumbnail`, `TileOverlay`).

#### Scenario: Trash renders a thumbnail grid

- **WHEN** the Trash tab is opened
- **THEN** deleted files are shown as a grid of image thumbnails grouped by deletion date, not as a table

### Requirement: Trash tile overlay has view and restore actions

The hover overlay on each trash tile SHALL contain exactly two buttons:

- **View** — opens a lightbox showing the full image with the original location before deletion and the deletion date.
- **Restore** — restores the file to its original location.

#### Scenario: Overlay shows two buttons

- **WHEN** the user hovers over a trash tile
- **THEN** the overlay shows exactly a View button and a Restore button

#### Scenario: View opens the lightbox with metadata

- **WHEN** the user clicks View
- **THEN** a lightbox opens showing the image, the original location (pre-deletion path), and the deletion date

#### Scenario: Restore returns the file

- **WHEN** the user clicks Restore
- **THEN** the file is restored to its original location and the tile disappears from the grid

### Requirement: Trash infinite scroll

The Trash view SHALL load items incrementally via cursor-based pagination and use an intersection observer sentinel to fetch the next page when the user scrolls near the end, consistent with the gallery views.

#### Scenario: Scrolling loads more trash items

- **WHEN** the user scrolls toward the end of the loaded trash groups
- **THEN** the next page of date groups is fetched and appended

### Requirement: Trash empty and loading states

The Trash view SHALL use the shared `EmptyState` primitive when empty and the shared `Skeleton` primitive for initial loading, consistent with the gallery.

#### Scenario: Empty trash uses the shared empty state

- **WHEN** the trash is empty
- **THEN** the shared `EmptyState` primitive is shown

### Requirement: Localization

All new user-facing text (view, restore, original location, deletion date, empty state, error toasts) SHALL be defined in both English (`translations.en.ts`) and Russian (`translations.ru.ts`) under strict `TranslationKey` typing, and backend error messages SHALL use `Msg*` constants with matching en/ru locale entries.

#### Scenario: English and Russian strings in sync

- **WHEN** the Trash view renders in English or Russian
- **THEN** every string is localized and the en/ru translation parity test passes
