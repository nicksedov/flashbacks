# gallery-browsing Specification

## Purpose

Defines how a configured gallery folder is scanned and how its indexed images are browsed through the paginated gallery endpoint, so the base "scan then browse" journey is specified and regression-tested.

## Requirements

### Requirement: Folder scan indexes images

The core service SHALL index image files found under configured gallery folders so that a completed scan makes them visible to the gallery listing endpoint.

<!-- focused-spec:id=gallery-browsing.scan-indexes-images -->
#### Scenario: Scan makes images browsable

- **WHEN** a gallery folder containing image files is configured and a scan is triggered and completes
- **THEN** `GET /api/gallery` lists the indexed images of that folder

### Requirement: Paginated gallery listing

`GET /api/gallery` SHALL return one page of indexed images together with the total image count and pagination metadata (`totalImages`, `currentPage`, `pageSize`, `totalPages`, `hasNextPage`).

<!-- focused-spec:id=gallery-browsing.gallery-paginated -->
#### Scenario: Gallery listing paginates

- **WHEN** the gallery contains more indexed images than a single page holds
- **THEN** the response reports `totalImages` larger than `pageSize` and `hasNextPage: true`, and requesting the next page returns a different, non-overlapping set of images

### Requirement: Scan status reports structured progress statistics

`GET /api/status` SHALL report a structured scan phase (`cleanup`, `scanning`,
or `complete`), the directory currently being scanned, a `totalFiles` count of
the image files under the gallery folders, and real `filesProcessed`,
`filesAdded`, `filesDeleted`, and `filesUpdated` counters that count files rather
than progress messages.

<!-- focused-spec:id=gallery-browsing.scan-status-phase -->
#### Scenario: Running scan reports a machine phase value

- **WHEN** a gallery scan is running
- **THEN** the response's `phase` is `cleanup` or `scanning` (a machine value, not a free-form English sentence)

<!-- focused-spec:id=gallery-browsing.scan-status-total-files -->
#### Scenario: Status reports the total number of image files

- **WHEN** the gallery folders contain N image files and a scan runs
- **THEN** `GET /api/status` reports `totalFiles` equal to N

<!-- focused-spec:id=gallery-browsing.scan-status-file-counters -->
#### Scenario: Status reports added and deleted file counts

- **WHEN** a scan adds A new records and deletes D missing records
- **THEN** `GET /api/status` reports `filesAdded` equal to A and `filesDeleted` equal to D

<!-- focused-spec:id=gallery-browsing.scan-status-files-processed -->
#### Scenario: Processed count reflects files, not progress messages

- **WHEN** a scan has processed P image files
- **THEN** `GET /api/status` reports `filesProcessed` equal to P

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
