## 1. OCR tab image container fix (OcrImagePanel)

- [x] 1.1 Rework the image container in [`OcrImagePanel.tsx`](../../webapp/src/components/gallery/lightbox/OcrImagePanel.tsx:45) so the `<img>` is bounded by the visible panel: replace the `inline-block` wrapper with a block-level `relative max-w-full max-h-full` wrapper (add `min-w-0`/`min-h-0` where needed) so the `<img>`'s `max-w-full max-h-full object-contain` scales it to fit; verify with `npm run lint && npx tsc -b`
- [x] 1.2 Confirm the bounding-box overlay remains `absolute` inside the same wrapper and sized from `displayDimensions` (not from the panel), so boxes stay aligned with the rendered image; verify with `npm run lint && npx tsc -b`

## 2. OCR panel wiring in UnifiedLightbox

- [x] 2.1 Adjust the OCR panel container className passed to `OcrImagePanel` in [`UnifiedLightbox.tsx`](../../webapp/src/components/gallery/UnifiedLightbox.tsx:283) (add `min-w-0`/`min-h-0` as needed) so the flex panel can shrink and the image fits the visible area; verify with `npm run lint && npx tsc -b`

## 3. Verification

- [x] 3.1 Manually verify in dev that the OCR-tab image fits entirely within the visible area for all three cases — text document with bounding boxes, image without recognized text, and rotated image — and that boxes stay aligned with the image; repeat after resizing the dialog
- [x] 3.2 Run the webapp test suite (`npm test`) and confirm no regressions
