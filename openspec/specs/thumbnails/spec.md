# thumbnails Specification

## Purpose

Server-side thumbnail generation and caching, including EXIF-orientation-aware
rendering that keeps the cached thumbnails consistent with the source photos.

## Requirements

### Requirement: Thumbnails honor EXIF orientation rotation

When generating a thumbnail from an image file, the core service SHALL apply the
same EXIF `Orientation` transform a browser applies when it displays the original
file in the lightbox, before resizing, so the thumbnail depicts the photo at the
angle the user sees: `6` rotates the stored pixels 90° clockwise, `8` rotates them
270° clockwise, `3` rotates them 180°, `2` and `4` mirror them horizontally and
vertically, and `5` and `7` transpose and transverse them. Orientation `1` or an
absent orientation SHALL produce a thumbnail from the image exactly as decoded.

<!-- focused-spec:id=thumbnails.orientation-6-rotated -->
#### Scenario: Orientation 6 turns the thumbnail a quarter turn clockwise

- **WHEN** a source image carries EXIF Orientation `6`
- **THEN** the stored bottom-left corner appears at the thumbnail's top-left corner and the stored top-left corner appears at its top-right corner

<!-- focused-spec:id=thumbnails.orientation-8-rotated -->
#### Scenario: Orientation 8 turns the thumbnail a quarter turn counter-clockwise

- **WHEN** a source image carries EXIF Orientation `8`
- **THEN** the stored top-right corner appears at the thumbnail's top-left corner and the stored top-left corner appears at its bottom-left corner

<!-- focused-spec:id=thumbnails.orientation-3-rotated -->
#### Scenario: Orientation 3 rotates the thumbnail by 180 degrees

- **WHEN** a source image carries EXIF Orientation `3`
- **THEN** the stored top-left corner appears at the thumbnail's bottom-right corner while the thumbnail keeps its width and height

<!-- focused-spec:id=thumbnails.orientation-1-unrotated -->
#### Scenario: Orientation 1 leaves the thumbnail unrotated

- **WHEN** a source image carries EXIF Orientation `1` or no EXIF orientation
- **THEN** the generated thumbnail matches the image decoded without rotation
