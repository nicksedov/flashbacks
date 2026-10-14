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
