# OCR Lightbox Specification

## Purpose

Defines how the image is displayed on the OCR tab of the unified lightbox: the image must fit entirely within the visible area while the OCR bounding-box overlay stays aligned with it.

## Requirements

### Requirement: OCR tab image fits entirely within the visible area

The OCR tab of the unified lightbox SHALL display the image scaled to fit entirely within the visible image area. This SHALL hold in all states, independently of whether the image has recognized text or bounding boxes, and independently of whether the image is displayed rotated relative to the original.

#### Scenario: Image with recognized text fits

- **WHEN** a user opens the OCR tab for an image classified as a text document with bounding boxes
- **THEN** the image is scaled so that it fits entirely within the visible area, with no part of the image extending beyond the viewport

#### Scenario: Image without recognized text fits

- **WHEN** a user opens the OCR tab for an image with no recognized text or an empty set of bounding boxes
- **THEN** the image is scaled so that it fits entirely within the visible area, with no part of the image extending beyond the viewport

#### Scenario: Rotated image fits

- **WHEN** a user opens the OCR tab for an image that is displayed rotated relative to the original
- **THEN** the rotated image is scaled so that it fits entirely within the visible area, with no part of the image extending beyond the viewport

### Requirement: Bounding boxes remain aligned with the image

When a text document is displayed on the OCR tab, the bounding-box overlay SHALL remain aligned with the underlying image after the image is scaled to fit the visible area.

#### Scenario: Boxes cover the same text regions

- **WHEN** a text document with bounding boxes is displayed and the image is scaled to fit the visible area
- **THEN** each bounding box covers the same text region it covered before the image was scaled to fit

#### Scenario: Alignment is preserved after resize

- **WHEN** a user resizes the lightbox while a text document with bounding boxes is displayed
- **THEN** the image remains fully visible within the visible area and the bounding boxes remain aligned with the image
