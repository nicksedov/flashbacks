# Spec Delta

## ADDED Requirements

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
