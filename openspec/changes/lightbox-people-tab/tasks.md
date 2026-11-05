# Tasks

## 1. Core API — image faces endpoint

- [ ] 1.1 Add `ImageFaceDTO` and `ImageFacesResponse` to [`backend/core/internal/interfaces/dto/media.go`](../../backend/core/internal/interfaces/dto/media.go:1) (fields `id`, `personId`, `personName`, `x`, `y`, `width`, `height`, `confidence`, camelCase JSON tags) and describe `GET /api/image-faces` plus both schemas in [`docs/api-contracts/core.yaml`](../../docs/api-contracts/core.yaml:1); verify `cd backend/core && go build ./...` succeeds and `make generate-types` regenerates `webapp/src/types/api.ts` with the new schemas
- [ ] 1.2 Implement `handleGetImageFaces` in [`backend/core/internal/interfaces/handler/handlers_faces.go`](../../backend/core/internal/interfaces/handler/handlers_faces.go:1) — gallery-access check on `path`, image lookup by path, faces ordered by id, memoized person-name lookup — and register the route in [`router.go`](../../backend/core/internal/interfaces/handler/router.go:149); verify `cd backend/core && go build ./... && go vet ./...` succeed
- [ ] 1.3 Cover the four API scenarios in [`handlers_faces_test.go`](../../backend/core/internal/interfaces/handler/handlers_faces_test.go:1): `TestHandleGetImageFaces_ListsDetectedFaces`, `TestHandleGetImageFaces_ReportsPersonName`, `TestHandleGetImageFaces_UnassignedFaceHasNoPerson`, `TestHandleGetImageFaces_EmptyWithoutFaces`; verify `cd backend/core && go test ./internal/interfaces/handler/ -count=1 && go test ./internal/application/... -count=1` pass

## 2. Webapp data layer

- [ ] 2.1 Add `fetchImageFaces` to [`webapp/src/api/endpoints.ts`](../../webapp/src/api/endpoints.ts:563) and the `ImageFaceDTO` / `ImageFacesResponse` types to [`webapp/src/types/index.ts`](../../webapp/src/types/index.ts:673); verify `cd webapp && npm run lint && npx tsc -b` are clean
- [ ] 2.2 Add the `useImageFaces` hook (fetch on mode activation, loading/error flags, and the derived numbered people list — distinct `personId` in face-id order, `personId 0` excluded) under [`webapp/src/hooks/`](../../webapp/src/hooks:1) with a unit test that asserts the numbering and that unassigned faces are excluded; verify `cd webapp && npm test -- useImageFaces && npm run lint && npx tsc -b`

## 3. Webapp lightbox People mode

- [ ] 3.1 Add the People mode strings — `lightbox.tab.people`, `lightbox.people.emptyTitle`, `lightbox.people.emptyHint`, `lightbox.people.loadFailed` — to [`translations.en.ts`](../../webapp/src/i18n/translations.en.ts:45) and [`translations.ru.ts`](../../webapp/src/i18n/translations.ru.ts:45) in the same edit; verify `cd webapp && npm test -- translations && npm run lint && npx tsc -b`
- [ ] 3.2 Extend `LightboxMode` with `"people"` and insert the People tab (lucide `Users`) immediately before Tags in [`lightboxChrome.tsx`](../../webapp/src/components/gallery/lightbox/lightboxChrome.tsx:13), then add a `UnifiedLightbox` test asserting the tab order and extend the existing stage-consistency test's mode list with `people`; verify `cd webapp && npm test -- UnifiedLightbox && npm run lint && npx tsc -b`
- [ ] 3.3 Add `PeopleImagePanel` (image stage + numbered/muted boxes scaled by rendered/natural size) next to the other lightbox panels, with tests for box-per-face, numbered person face, unnumbered unassigned face and scaling at a different rendered size; verify `cd webapp && npm test -- PeopleImagePanel && npm run lint && npx tsc -b`
- [ ] 3.4 Add `PeoplePanel` (numbered people list, `people.unnamed` fallback, shared `EmptyState`, error banner) with tests for the list, the fallback, the empty state and the Russian labels; verify `cd webapp && npm test -- PeoplePanel && npm run lint && npx tsc -b`
- [ ] 3.5 Wire both components into [`UnifiedLightbox.tsx`](../../webapp/src/components/gallery/UnifiedLightbox.tsx:280) — the People image branch before the `enhancedPath` branch, the panel content for `activeMode === "people"`, and reset behaviour when the image changes; verify `cd webapp && npm test -- UnifiedLightbox && npm run lint && npx tsc -b`

## 4. Evidence and full checks

- [ ] 4.1 Register the thirteen scenario ids from [`specs/face-recognition/spec.md`](specs/face-recognition/spec.md) in [`.focused-spec/evidence.yaml`](../../.focused-spec/evidence.yaml:1) using the runner/selector table from [`design.md`](design.md); verify `node_modules/.bin/focused-spec validate --scope current --strict` reports no errors
- [ ] 4.2 Run `node_modules/.bin/focused-spec run --scenario <id>` for every new scenario id and record the observed result; for each new evidence entry prove sensitivity by reverting the behaviour (not the test), observing a failing assertion, then restoring and observing `PASS`
- [ ] 4.3 Run the full component suites: `cd backend/core && go test ./internal/application/... -count=1` plus `go test ./internal/interfaces/handler/ -count=1`, and `cd webapp && npm run lint && npx tsc -b && npm test && npm run build`

## Workflow follow-up

- Sync the delta spec into `openspec/specs/face-recognition/spec.md` after implementation passes.
- Review the branch diff against `main` (§7 of the SDD workflow) before merging.
- Archive the change once the sync and review are complete.
