# Fix OCR Tab Image Fit in UnifiedLightbox

## Why

On the OCR tab of the lightbox the image is rendered at a larger scale than on
all other tabs and often does not fit entirely within the visible area. On
every other tab the image always fits (it is constrained with
`max-w-full`/`max-h-full`/`object-contain`). This makes the OCR tab hard to use:
the image, the detected-text highlights and the overlay UI extend beyond the
viewport.

## What Changes

- Frontend (webapp): change the OCR tab image rendering so the image always
  fits entirely within the visible area, in all cases:
  - whether or not the image has recognized text / bounding boxes;
  - whether or not the image is displayed rotated relative to the original.
- Preserve the currently correct positioning of OCR bounding boxes: after the
  change, the box overlay must still align exactly with the underlying image.
- Keep the existing dimension measurement approach (`useImageDimensions`,
  `clientWidth`/`clientHeight` of the rendered `<img>`) and the existing box
  scaling formula (`displayDimensions` ÷ `boundingBoxWidth`/`boundingBoxHeight`).
- **No backend or database change** is required: the OCR classification response
  already returns `boundingBoxWidth`/`boundingBoxHeight` (the preprocessed image
  dimensions), which together with the measured rendered dimensions is
  sufficient to scale the boxes correctly once the image is constrained to fit.
  The note about optionally extending the OCR endpoint / DB table is therefore
  not exercised.

## Non-goals

- No changes to the OCR microservice (`backend/ocr`), the OCR/LLM recognition
  pipeline, the api-service OCR endpoints, or the database schema.
- No changes to the other lightbox tabs (their behavior already matches the
  expected result).
- No rework of the bounding-box scaling math or the OCR data contract.

## Capabilities

### New Capabilities

- `ocr-lightbox`: Behavior of the OCR tab in the unified lightbox — the image
  must be displayed fully within the visible area and the OCR bounding-box
  overlay must remain aligned with the image in all states (text/no text,
  rotated/non-rotated).

### Modified Capabilities

- None.

## Impact

- `webapp/src/components/gallery/UnifiedLightbox.tsx` — OCR panel wiring
  (container classNames passed to `OcrImagePanel`, if needed).
- `webapp/src/components/gallery/lightbox/OcrImagePanel.tsx` — image and
  bounding-box overlay layout (primary change).
- `webapp/src/hooks/useImageDimensions.ts` — reused as-is; no API change
  expected.
- No OpenAPI / MCP contract changes; no i18n changes expected.
