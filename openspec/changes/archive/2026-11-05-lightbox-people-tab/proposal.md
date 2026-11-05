# Proposal

## Why

Face recognition already stores detected faces and clusters them into people, and
the People tab lists those people with their photos — but the photo itself carries
no face information. Opening a photo in the unified lightbox shows nothing about
who is on it or where those faces are, so a group photo is still an anonymous grid
of pixels. The lightbox is the one place where the stored face boxes and the
person they belong to can be seen together.

## What Changes

- Add a read-only endpoint `GET /api/image-faces?path=<imagePath>` that returns
  the faces detected on one gallery image, each with its bounding box, its
  detection confidence and the person it was clustered into (name included).
  Boxes are returned in the analysed image's pixel frame, which the lightbox scales
  to whatever size the image is rendered at. The endpoint is added to the OpenAPI contract
  [`docs/api-contracts/core.yaml`](../../docs/api-contracts/core.yaml:1) and
  `make generate-types` regenerates [`webapp/src/types/api.ts`](../../webapp/src/types/api.ts:1).
- Add a **People** mode to the unified lightbox tab bar, placed immediately
  before the **Tags** tab (`AI`, `EXIF`, `OCR`, `People`, `Tags`).
- Render the image on the existing dark image stage with a bounding-box overlay:
  every detected face gets a box; a face clustered into a person additionally
  gets its number, and a face without a person gets a muted, unnumbered box.
- Render a numbered list of the people present in the photo in the right-hand
  panel; each entry shows the number drawn on the image and the person's display
  name (the localized "Person N" fallback when the person has no name yet).
- Add the People tab's loading, empty and error states plus the new English and
  Russian strings.

## Capabilities

### New Capabilities

None — the change extends the existing face-recognition capability.

### Modified Capabilities

- `face-recognition`: adds the requirement that a single image's detected faces
  are exposed by the API with bounding boxes and their person, and the requirement
  that the unified lightbox renders a People mode with numbered bounding boxes and
  a matching people list.

## Non-goals

- No naming, merging, splitting or deleting of people from the lightbox; person
  management stays in the People view.
- No navigation from a face box or a people-list entry to that person's photo
  grid, and no click/hover coupling between the boxes and the list.
- No face crop thumbnails in the people list and no highlighting of a selected
  face.
- No bounding-box overlay in the other lightbox modes (AI, EXIF, OCR, Tags).
- No change to how faces are detected, clustered or stored.
- No correction of EXIF orientation for the overlay; boxes are drawn in the
  pixel frame of the stored image, like the stored coordinates.

## Impact

- **Backend (`backend/core`)**: new DTO, handler and route; reuse of the existing
  `FaceRepository`/`PersonRepository` and gallery path verification; unit tests
  for the new handler.
- **API contract**: `docs/api-contracts/core.yaml` gains the endpoint and schema;
  `make generate-types` refreshes `webapp/src/types/api.ts`.
- **Webapp**: `lightboxChrome.tsx` (mode + tab definitions), `UnifiedLightbox.tsx`
  (People mode wiring and image-stage overlay), a new people panel component, a
  new data hook, an API client function, `translations.en.ts` +
  `translations.ru.ts`, Vitest tests.
- **Design system**: the existing lightbox requirements (single dark image stage,
  one tab primitive, accessible names, 44×44 touch targets) apply unchanged to the
  new mode.
