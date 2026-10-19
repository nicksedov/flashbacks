# gallery-browsing Specification

## Purpose

Defines how gallery folders are configured, how a configured gallery folder is scanned, and how its indexed images are browsed through the paginated gallery endpoint, so the base "configure, scan, then browse" journey is specified and regression-tested.

## Requirements

### Requirement: Folder scan indexes images

The core service SHALL index image files found under configured gallery folders so that a completed scan makes them visible to the gallery listing endpoint.

<!-- focused-spec:id=gallery-browsing.scan-indexes-images -->
#### Scenario: Scan makes images browsable

- **WHEN** a gallery folder containing image files is configured and a scan is triggered and completes
- **THEN** `GET /api/gallery` lists the indexed images of that folder

### Requirement: Gallery folder configuration

`GET /api/folders` SHALL list the configured gallery root folders, `POST /api/folders` SHALL add an existing directory on disk as a gallery root, and `DELETE /api/folders/:id` SHALL remove a configured root. Adding a folder SHALL reject a path that does not exist and a path that is already configured.

<!-- focused-spec:id=gallery-browsing.folders-initially-empty -->
#### Scenario: No folders are configured initially

- **WHEN** no gallery root folder has been added
- **THEN** `GET /api/folders` reports `totalFolders: 0` and an empty `folders` array

<!-- focused-spec:id=gallery-browsing.folder-add -->
#### Scenario: Adding an existing directory configures a gallery root

- **WHEN** an admin posts an existing directory path to `POST /api/folders`
- **THEN** the response is HTTP 200 with message `folder.added`, a folder record whose `path` equals the submitted path, and a boolean `scanStarted`

<!-- focused-spec:id=gallery-browsing.folder-list -->
#### Scenario: Configured folders are listed with metadata

- **WHEN** `GET /api/folders` is called after adding folders
- **THEN** `totalFolders` equals the number of configured roots and each folder carries its `path`, `fileCount`, and `createdAt`

<!-- focused-spec:id=gallery-browsing.folder-invalid-path -->
#### Scenario: A missing directory is rejected

- **WHEN** an admin posts a path that does not exist on disk to `POST /api/folders`
- **THEN** the response is HTTP 400 with error `folder.invalid_path` and no folder is created

<!-- focused-spec:id=gallery-browsing.folder-duplicate -->
#### Scenario: A duplicate folder is rejected

- **WHEN** an admin posts a path that is already configured
- **THEN** the response is HTTP 409 with error `folder.already_in_gallery`

<!-- focused-spec:id=gallery-browsing.folder-remove -->
#### Scenario: Removing a folder drops it from the listing

- **WHEN** an admin deletes a configured folder by id
- **THEN** the response is HTTP 200 with message `folder.removed` and `GET /api/folders` no longer contains that path

### Requirement: Paginated gallery listing

`GET /api/gallery` SHALL return one page of indexed images together with the total image count and pagination metadata (`totalImages`, `currentPage`, `pageSize`, `totalPages`, `hasNextPage`).

<!-- focused-spec:id=gallery-browsing.gallery-paginated -->
#### Scenario: Gallery listing paginates

- **WHEN** the gallery contains more indexed images than a single page holds
- **THEN** the response reports `totalImages` larger than `pageSize` and `hasNextPage: true`, and requesting the next page returns a different, non-overlapping set of images

### Requirement: Scan status reports structured progress statistics

`GET /api/status` SHALL report a structured scan phase (`cleanup`, `scanning`,
or `complete`), the directory currently being scanned, a `totalFiles` count of
the work items the scan will process, and real `filesProcessed`, `filesAdded`,
`filesDeleted`, and `filesUpdated` counters that count items rather than progress
messages. A scan processes two kinds of item: existing records verified (and
removed when missing) during the `cleanup` phase, and image files indexed during
the `scanning` phase. `totalFiles` is reported before the cleanup phase starts and
`filesProcessed` grows monotonically across both phases, so progress is
determinate from the first poll.

<!-- focused-spec:id=gallery-browsing.scan-status-phase -->
#### Scenario: Running scan reports a machine phase value

- **WHEN** a gallery scan is running
- **THEN** the response's `phase` is `cleanup` or `scanning` (a machine value, not a free-form English sentence)

<!-- focused-spec:id=gallery-browsing.scan-status-total-files -->
#### Scenario: Status reports the total work the scan will process

- **WHEN** a scan starts with D existing records and the gallery folders contain N image files
- **THEN** `GET /api/status` reports `totalFiles` equal to D + N

<!-- focused-spec:id=gallery-browsing.scan-status-file-counters -->
#### Scenario: Status reports added and deleted file counts

- **WHEN** a scan adds A new records and deletes D missing records
- **THEN** `GET /api/status` reports `filesAdded` equal to A and `filesDeleted` equal to D

<!-- focused-spec:id=gallery-browsing.scan-status-files-processed -->
#### Scenario: Processed count reflects items examined, not progress messages

- **WHEN** a scan has examined the cleanup records and the image files
- **THEN** `GET /api/status` reports `filesProcessed` equal to the number of items examined, not the number of progress messages

<!-- focused-spec:id=gallery-browsing.scan-status-cleanup-completes -->
#### Scenario: Cleanup phase completes for an index larger than one batch

- **WHEN** the index holds more records than the cleanup batch size and their files still exist on disk
- **THEN** the cleanup phase finishes without re-reading the same records and the scan advances past it

### Requirement: Scan result equivalence is preserved

The folder scan SHALL classify each image file (created, updated with a
content-change flag, deleted, or unchanged) and write the same database rows as
before the batching optimisation, regardless of the internal batch size, with one
deliberate correction: the update path no longer overwrites `created_at` with the
zero time (see the creation-time scenario below).

<!-- focused-spec:id=gallery-browsing.scan-result-equivalence -->
#### Scenario: Scan produces identical classification across batch sizes

- **WHEN** a directory containing unchanged, size-changed,
  content-changed-at-same-size, new, and deleted image files is scanned
- **THEN** the created, updated, deleted and unchanged sets and the
  `ContentChanged` flag of each updated record are identical whether writes are
  flushed one row at a time or in larger batches

<!-- focused-spec:id=gallery-browsing.scan-preserves-created-at -->
#### Scenario: Updating a record preserves its creation time

- **WHEN** an existing image record is updated by a scan
- **THEN** its `created_at` still holds the timestamp from when the record was
  first created, instead of being overwritten with the zero time as it was before
  the batching change

### Requirement: File hashing digest is stable

Hashing a file SHALL return the same MD5 digest regardless of internal buffer
reuse or concurrent hashing.

<!-- focused-spec:id=gallery-browsing.hash-digest-stable -->
#### Scenario: Hash digest does not depend on buffering

- **WHEN** the same file content is hashed
- **THEN** the returned digest equals the canonical MD5 for that content, and it
  does not change when the hashing routine reuses a pooled read buffer

### Requirement: Scan database writes are batched

A scan SHALL write changed rows in batches rather than one SQL statement per
row, so the number of executed write statements grows with the number of
batches, not the number of files.

<!-- focused-spec:id=gallery-browsing.scan-batched-writes -->
#### Scenario: Write statement count is bounded by the batch size

- **WHEN** a scan updates or inserts N changed records
- **THEN** the number of SQL write statements executed is at most
  ceil(N / batch), not N per-row statements

### Requirement: Hashing memory is bounded

Hashing a file SHALL use a fixed-size reusable read buffer rather than
allocating a fresh full-copy buffer per file.

<!-- focused-spec:id=gallery-browsing.hash-bounded-allocations -->
#### Scenario: Hashing allocates a constant, small amount

- **WHEN** a file is hashed
- **THEN** the allocations per hash are constant and small (a pooled read
  buffer), independent of the file size and of the number of concurrent hash
  workers
