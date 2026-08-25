# Design — Fix OCR Tab Image Fit in UnifiedLightbox

## Context

See `proposal.md` — Why for motivation and the spec `specs/ocr-lightbox/spec.md`
for the behavior contract.

Current state of the OCR tab rendering:

- [`UnifiedLightbox.tsx`](../../webapp/src/components/gallery/UnifiedLightbox.tsx:283) renders
  `OcrImagePanel` in a flex panel; the image URL is either the rotated OCR
  image (`/api/ocr-image?angle=N`) or the original (`/api/image`).
- [`useImageDimensions()`](../../webapp/src/hooks/useImageDimensions.ts:25) measures the rendered
  `<img>` element (`clientWidth`/`clientHeight`) after load and on resize,
  storing the result in `displayDimensions`.
- [`OcrImagePanel.tsx`](../../webapp/src/components/gallery/lightbox/OcrImagePanel.tsx:45) wraps the
  `<img>` in `<div className="relative inline-block">`. Because the wrapper is
  `inline-block`, it shrink-wraps to the image's intrinsic size and does not
  bound the image; the `<img>`'s `max-w-full`/`max-h-full`/`object-contain`
  resolve against that content-sized containing block and are effectively
  unbounded. The image therefore renders at (near) its natural preprocessed
  size and overflows the visible area — unlike the other tabs, where the
  `<img>` is a direct flex child of a constrained container
  ([`UnifiedLightbox.tsx`](../../webapp/src/components/gallery/UnifiedLightbox.tsx:315)).
- The bounding-box overlay is anchored to the same `relative` wrapper and sized
  to `displayDimensions`; each box is scaled by
  `scaleX = displayDimensions.width / boundingBoxWidth` and
  `scaleY = displayDimensions.height / boundingBoxHeight`
  ([`OcrImagePanel.tsx`](../../webapp/src/components/gallery/lightbox/OcrImagePanel.tsx:30)).

Because the overlay origin already matches the rendered image (both derive from
the same `displayDimensions`), the only required fix is to make the image
actually fit. No part of the measurement/scaling math needs to change.

## Goals / Non-Goals

**Goals:**

- Make the OCR-tab image fit entirely within the visible area in all cases
  (with/without boxes, rotated/non-rotated).
- Keep the bounding-box overlay exactly aligned with the rendered image.
- Keep the change frontend-only and small.

**Non-Goals:**

- No OCR service, api-service, DB, or OpenAPI changes (see Decision 3).
- No changes to the other lightbox tabs.
- No rework of the OCR data contract or the box-scaling formula.

## Decisions

### 1. Make the `<img>` a direct flex child of the panel and center the overlay

Mirror the other lightbox tabs, where the `<img>` is a direct flex child of a
constrained, definite-height panel and is sized with
`max-w-full max-h-full object-contain`:

- Keep the outer panel as the flex centering context
  (`flex items-center justify-center relative h-full`) and add
  `min-w-0 min-h-0` so the panel (a flex item) can shrink below the image's
  intrinsic size. No padding is applied so the image fills the panel like the
  other tabs.
- Remove the intermediate `relative inline-block` wrapper entirely and render
  the `<img>` as a direct flex child. Its `max-w-full` / `max-h-full` then
  resolve against the panel's definite width/height, so the image scales to fit
  exactly like the other tabs. The `<img>` keeps auto dimensions, so
  `useImageDimensions` measures the true fitted size (not a letterboxed box).
- Anchor the bounding-box overlay to the centered image instead of to a wrapper:
  render it `absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2`,
  sized to `displayDimensions`. Because the panel padding is symmetric, the
  content-box center coincides with the padding-box center, so the overlay's
  origin lands exactly on the rendered image.

**Why not the originally proposed bounded wrapper:** a wrapper with
`max-w-full max-h-full` (plus `min-w-0`/`min-h-0`) still fails to bound the
`<img>`: the `<img>`'s percentage `max-height` resolves against the wrapper's
auto height (an indefinite containing block), which either leaves the image
overflowing or collapses it to a fragment in some browser/layout combinations.
The direct-flex-child pattern is the same one the other tabs already use and is
known to fit correctly.

**Alternatives considered:**

- *Bounded `relative max-w-full max-h-full` wrapper* — rejected after a dev
  regression: percentage-height resolution against the auto-height wrapper
  collapses/overflows the image.
- *Overlay with `absolute inset-0` over the whole panel* — rejected: the image
  is centered, so a panel-sized overlay's origin would not match the image
  origin and boxes would be offset. Centering the `displayDimensions`-sized
  overlay avoids this without manual offset math.

### 2. Keep the existing measurement and scaling pipeline unchanged

`useImageDimensions` reads `clientWidth`/`clientHeight` of the rendered `<img>`,
so `displayDimensions` automatically reflects whatever fitted size CSS produces.
The box scaling (`displayDimensions ÷ boundingBoxWidth|Height`) therefore stays
correct with no change to the hook or the formula.

### 3. No backend or database change

`boundingBoxWidth` / `boundingBoxHeight` already describe the preprocessed image
(the space the box coordinates live in), and the frontend measures the rendered
size at runtime. These two values are sufficient to fit the image via CSS and to
scale the boxes; the OCR endpoint/table does not need to return anything extra.
The user permitted extending the OCR endpoint and its DB table *if necessary*;
it is not necessary, so this change stays frontend-only and avoids a
schema/OpenAPI migration.

## Risks / Trade-offs

- [A bounded wrapper still fails to constrain height in some layout/browser
  combination, leaving the overflow in edge cases] → Mitigation: verify in dev
  with (a) a text document with boxes, (b) an image without recognized text, and
  (c) a rotated image; confirm the image and boxes remain in sync on dialog
  resize.
- [Overlay misalignment if the wrapper no longer exactly matches the rendered
  image] → Mitigation: keep the overlay inside the same wrapper that wraps the
  image and keep it sized to `displayDimensions`; do not size the overlay to the
  panel.
- [Regression in the non-text case if the image element is still rendered at
  natural size while loading] → Mitigation: the loading placeholder already has
  a fixed size; the real `<img>` only measures after `onLoad`, and the fit
  constraints apply to the element itself.

## Migration Plan

Frontend-only change shipped with the webapp bundle. Rollback is a revert of the
webapp change; there is no data or backend dependency to migrate.

## Open Questions

None — the exact Tailwind class combinations are apply-time implementation
details and do not affect the specs, approach, or task breakdown.
