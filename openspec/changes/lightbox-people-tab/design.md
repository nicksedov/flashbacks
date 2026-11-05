# Design

## Context

Stored face data already carries everything the tab needs — [`domain.Face`](../../backend/core/internal/domain/faces.go:36)
holds `ImageFileID`, `PersonID`, `X`, `Y`, `Width`, `Height` and `Confidence`, and
[`domain.Person`](../../backend/core/internal/domain/faces.go:20) holds the optional
name — but no endpoint exposes a single image's faces, and the unified lightbox has
no mode that shows them. See [`proposal.md`](proposal.md) for motivation.

Constraints that shape the approach:

- The analysis job reads the file bytes verbatim and sends them to the faces
  service, which returns boxes in that same pixel frame
  ([`manager.go`](../../backend/core/internal/application/faces/manager.go:420)).
  `/api/image` also serves the file verbatim, so the displayed image and the stored
  boxes share one coordinate frame whenever the browser does not re-orient the image
  (EXIF orientation caveat, see Risks).
- The gallery supports JPEG, PNG, GIF, BMP, TIFF and WEBP, while core registers Go
  decoders only for JPEG, PNG and GIF — so the server cannot reliably measure the
  analysed frame for every supported file.
- Every lightbox mode must render the image on the same dark stage
  ([`design-system` requirement](../../openspec/specs/design-system/spec.md:496)),
  and the tab bar is one shared tab primitive
  ([`lightboxChrome.tsx`](../../webapp/src/components/gallery/lightbox/lightboxChrome.tsx:13)).

## Goals / Non-Goals

**Goals**

- One read-only request answers "who is on this photo and where".
- Box geometry survives any rendered image size, on desktop and in the mobile
  full-screen lightbox, without a server-side image decode.
- The People mode inherits the existing lightbox stage, tab, accessibility and
  touch-target rules instead of introducing a parallel layout.

**Non-Goals**

- No interaction model beyond reading: no selection, no navigation to a person,
  no rename from the lightbox (see proposal Non-goals).
- No new persistence: the feature is a projection of columns that already exist, so
  there is no migration and no backfill.
- No orientation normalization of either the image or the boxes.

## Decisions

### D1 — A dedicated read-only endpoint `GET /api/image-faces?path=`

The response is `{ faces: ImageFaceDTO[] }`, each entry carrying `id`, `personId`,
`personName`, `x`, `y`, `width`, `height` and `confidence`, ordered by face id
ascending.

- **Alternatives rejected**: extending `/api/people` (it answers a gallery-wide
  listing, not "this photo"); reusing `/api/image-metadata` (EXIF domain, and it
  would couple two unrelated payloads); letting the client fetch `/api/people` and
  then `/api/people/{id}/photos` per person (O(people) requests to answer one
  question).
- **Naming**: `/api/faces/*` already means the recognition *job* (`status`, `start`,
  `stop`), so the per-image read lives at `/api/image-faces`, matching the existing
  `/api/image-tags` naming for a per-image read.
- **Where it lives**: a handler in
  [`handlers_faces.go`](../../backend/core/internal/interfaces/handler/handlers_faces.go:1)
  over the already-injected `FaceRepository`, `PersonRepository`,
  `ImageFileRepository` and gallery-access check. No domain or application layer
  change, so the Wire graph (and `wire_gen.go`) is untouched.
- **Person names**: resolved server-side (one lookup per distinct `personId` in the
  photo, memoized) so the client needs exactly one request and never has to
  reconcile two listings.
- **Errors**: missing/blank `path` → 400; a path outside the gallery folders →
  403 (existing `galleryAccess` behaviour); an image with no stored faces → 200 with
  an empty list, not 404, because "no faces" is a normal answer for the tab.

### D2 — Boxes stay in the analysed pixel frame; the client scales them

The endpoint returns pixel coordinates; the People mode scales a box by
`renderedWidth / naturalWidth` and `renderedHeight / naturalHeight` of the very
`<img>` element that displays `/api/image`.

- **Why not normalize server-side**: normalization needs the analysed frame's pixel
  size, which core can only get by decoding image headers — and core cannot decode
  BMP, TIFF or WEBP at all (Context). Normalizing would turn "a photo in a supported
  format" into a 500 for those files.
- **Why the client can trust `naturalWidth`**: the box frame is the file's raw
  pixel frame, and `/api/image` serves the file unchanged, so the browser's natural
  size *is* the analysed size — for exactly the formats the browser can display,
  which is the only set that can show a box at all.
- The overlay is rendered only once the image has loaded and its natural size is
  known; before that the panel shows its loading state rather than mis-scaled boxes.
- This mirrors how [`OcrImagePanel`](../../webapp/src/components/gallery/lightbox/OcrImagePanel.tsx:35)
  already scales OCR boxes to the rendered image.

### D3 — One derived "people in this photo" list, numbered by first appearance

The People mode derives its list from the fetched faces: distinct `personId`s in
face-id order, numbered 1..N; every face of a numbered person carries that number,
and a face with `personId === 0` is drawn muted and unnumbered (the behaviour chosen
for unassigned faces).

- Putting the derivation in one hook (`useImageFaces`) keeps the overlay and the
  list from computing the numbering twice and drifting apart.
- Numbering is per photo on purpose: the list answers "who is in this picture",
  and a gallery-wide stable person number would need a persisted ordering that does
  not exist.

### D4 — People mode renders its own image branch, like the OCR mode

`UnifiedLightbox` gains an `activeMode === "people"` branch that renders a
`PeopleImagePanel` (stage + `<img>` + overlay) and a `PeoplePanel` in the right-hand
detail panel, placed before the `enhancedPath` branch so the compare slider never
hides the boxes.

- **Alternative rejected**: folding the overlay into the shared default branch.
  That branch has no `<img>` ref or dimension state, and adding them there would
  change the markup every other mode shares.
- `PeopleImagePanel` reuses the OCR stage class set
  (`flex-1 flex items-center justify-center bg-black relative h-full min-w-0`), so
  the "image stage is consistent across modes" requirement keeps holding and the
  existing stage test simply gains `people` in its mode list.
- `LightboxMode` becomes `"ai" | "exif" | "ocr" | "people" | "tags"`, and the shared
  tab config inserts `people` (icon `Users`) immediately before `tags`.

### D5 — Token-derived overlay colors

Numbered boxes use the primary tokens (`border-primary`, `bg-primary/10`, badge
`bg-primary text-primary-foreground`); unassigned boxes use muted tokens
(`border-muted-foreground/60`, `bg-muted-foreground/10`, no badge). OCR's yellow is
deliberately not reused: the two overlays can be seen in the same lightbox, and
color alone must not be the only difference — the number badge carries the meaning.

### D6 — i18n keys added together

`lightbox.tab.people`, `lightbox.people.emptyTitle`, `lightbox.people.emptyHint` and
`lightbox.people.loadFailed` are added to `translations.en.ts` and
`translations.ru.ts` in the same commit; the unnamed-person fallback reuses the
existing `people.unnamed` key, and the empty state reuses the shared `EmptyState`
primitive.

### D7 — Evidence bindings (recorded for the apply phase)

Each scenario in [`specs/face-recognition/spec.md`](specs/face-recognition/spec.md)
maps to one test, registered later in `.focused-spec/evidence.yaml` (not edited
during propose):

| Scenario id | Runner | Selector |
|---|---|---|
| `face-recognition.image-faces-returned` | `go` | `./internal/interfaces/handler::TestHandleGetImageFaces_ListsDetectedFaces` |
| `face-recognition.image-face-person` | `go` | `./internal/interfaces/handler::TestHandleGetImageFaces_ReportsPersonName` |
| `face-recognition.image-face-unassigned` | `go` | `./internal/interfaces/handler::TestHandleGetImageFaces_UnassignedFaceHasNoPerson` |
| `face-recognition.image-faces-empty` | `go` | `./internal/interfaces/handler::TestHandleGetImageFaces_EmptyWithoutFaces` |
| `face-recognition.image-face-box-scaled` | `vitest` | `webapp/src/components/gallery/lightbox/PeopleImagePanel.test.tsx::PeopleImagePanel > scales every box with the rendered image` |
| `face-recognition.lightbox-people-tab-order` | `vitest` | `webapp/src/components/gallery/UnifiedLightbox.test.tsx::UnifiedLightbox > places the People tab immediately before the Tags tab` |
| `face-recognition.lightbox-people-box-per-face` | `vitest` | `webapp/src/components/gallery/lightbox/PeopleImagePanel.test.tsx::PeopleImagePanel > draws one box per detected face` |
| `face-recognition.lightbox-people-face-numbered` | `vitest` | `webapp/src/components/gallery/lightbox/PeopleImagePanel.test.tsx::PeopleImagePanel > labels a person's face with the person's number` |
| `face-recognition.lightbox-people-unassigned-unnumbered` | `vitest` | `webapp/src/components/gallery/lightbox/PeopleImagePanel.test.tsx::PeopleImagePanel > draws an unassigned face without a number` |
| `face-recognition.lightbox-people-list` | `vitest` | `webapp/src/components/gallery/lightbox/PeoplePanel.test.tsx::PeoplePanel > lists each person once with the number drawn on the image` |
| `face-recognition.lightbox-people-name-fallback` | `vitest` | `webapp/src/components/gallery/lightbox/PeoplePanel.test.tsx::PeoplePanel > renders the placeholder for an unnamed person` |
| `face-recognition.lightbox-people-empty` | `vitest` | `webapp/src/components/gallery/lightbox/PeoplePanel.test.tsx::PeoplePanel > shows the empty state when the image has no faces` |
| `face-recognition.lightbox-people-localized` | `vitest` | `webapp/src/components/gallery/lightbox/PeoplePanel.test.tsx::PeoplePanel > renders Russian labels under the Russian interface` |

## Risks / Trade-offs

- **EXIF-rotated photos** — the browser auto-orients the displayed image while the
  boxes stay in the raw frame, so boxes can be rotated off their faces. → Documented
  in the proposal's Non-goals; the same limitation already exists for OCR boxes and
  is fixed by a later orientation-normalization change for both overlays.
- **Stale faces** — an image whose `faces_scan_version` is behind the current
  analyser still returns the previously stored boxes. → Acceptable: the job rewrites
  them on its next pass, and the tab never fabricates faces.
- **N+1 person lookups** — one `FindByID` per distinct person on the photo. →
  Bounded by faces per image and memoized per request; no gallery-wide scan.
- **Crowded group photos** — a 27-face photo produces 27 boxes and a long list. →
  The list scrolls inside the detail panel and the number badges stay small
  (`text-xs`); no clustering or collapsing in this change.
- **Compare-slider branch** — with an accepted enhancement the lightbox shows the
  slider; the People branch is evaluated first, so opening People always shows the
  original image and its boxes.

## Migration Plan

No schema, data or configuration change, so there is nothing to migrate and no
backfill. The endpoint is additive and unread by older clients; reverting the branch
removes it without leaving state behind.

## Open Questions

None — the unassigned-face behaviour was decided with the requester (muted,
unnumbered box, no list entry).
