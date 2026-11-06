# Tasks

## 1. Domain and data access

- [x] 1.1 Add `Person.Manual` (`gorm:"not null;default:false"`) and `Face.ManualAssignment` (`gorm:"not null;default:false;index"`) to [`backend/core/internal/domain/faces.go`](../../../backend/core/internal/domain/faces.go:1); verify with `cd backend/core && go test ./internal/domain/... ./internal/testutil/... -count=1` and a migration assertion that both columns exist after AutoMigrate
- [x] 1.2 Add `PersonRepository.Delete(id uint) error` and `FaceRepository.ListUnassigned(offset, limit int)` / `CountUnassigned() (int64, error)`; verify with `cd backend/core && go test ./internal/domain/repository/... -count=1` covering deletion, pagination order and the exclusion of assigned faces
- [x] 1.3 Add `PersonReferenceRepository` coverage for the new flows (list+replace filtering by face, delete by person) via a repository test; verify `cd backend/core && go test ./internal/domain/repository/... -count=1`

## 2. Catalog application service

- [x] 2.1 Create [`backend/core/internal/application/people/catalog.go`](../../../backend/core/internal/application/people/catalog.go:1) with the `Catalog` constructor (db, person/face/reference/image-file repositories) and sentinel errors for unknown person/face and invalid input; verify `cd backend/core && go build ./...`
- [x] 2.2 Implement `Rename(personID, name)` (trim, 200-char limit, empty clears, sets `manual`); verify `TestCatalogRename` covers trimmed, cleared, over-long-name rejection and unknown-id cases and prints PASS
- [x] 2.3 Implement `Merge(target, sources, name)` in one transaction: reassign faces, lock them, delete sources, union references capped at 5, keep/fallback the cover, reject target in sources, empty sources and more than 50 sources; verify `TestCatalogMerge` asserts shared photos count once, sources disappear and references point at the target
- [x] 2.4 Implement `CreateFromFaces(name, faceIDs)`: new manual person, faces assigned and locked, references seeded from their embeddings; verify `TestCatalogCreateFromFaces` covers named and unnamed creation plus empty `faceIds` and unknown/assigned face ids
- [x] 2.5 Implement `Disband(personID)`: faces unassigned and locked, references deleted, person removed, photos untouched; verify `TestCatalogDisband` asserts the photos and face rows survive and the person is gone
- [x] 2.6 Implement `AssignFace(faceID, personID)` including `personID == 0` detach: locks the face, drops/replaces the references of the previous and new person, and promotes a new cover when the cover face leaves; verify `TestCatalogAssignFace` covers assign, detach, cover promotion and unknown person
- [x] 2.7 Implement `ListUnassigned(page, pageSize)` joining `image_files` and excluding orphaned faces (detached user-locked faces stay listed, as the spec requires); verify `TestCatalogListUnassigned` covers pagination, `hasNextPage`, empty result and confidence/box passthrough
- [x] 2.8 Run the application suite: `cd backend/core && go test ./internal/application/... -count=1`

## 3. HTTP API and contract

- [x] 3.1 Add DTOs `UnassignedFaceDTO` / `UnassignedFacesResponse` and request structs for rename, merge, create and face assignment in [`backend/core/internal/interfaces/dto/media.go`](../../../backend/core/internal/interfaces/dto/media.go:1), and add `Msg*` constants with en/ru locale entries for every new error and success message; verify the core locale parity test passes with `cd backend/core && go test ./internal/interfaces/i18n/... -count=1`
- [x] 3.2 Add handlers for `PATCH /api/people/:id`, `POST /api/people`, `POST /api/people/merge`, `DELETE /api/people/:id`, `PATCH /api/faces/:id`, `GET /api/faces/unassigned` and register them in the authenticated face block of [`backend/core/internal/interfaces/handler/router.go`](../../../backend/core/internal/interfaces/handler/router.go:1); verify handler tests cover 200/201, 400 validation, 404 unknown ids and the unassigned pagination envelope with `cd backend/core && go test ./internal/interfaces/handler/... -count=1`
- [x] 3.3 Document the six paths and their schemas in [`docs/api-contracts/core.yaml`](../../../docs/api-contracts/core.yaml:1) and run `make generate-types`; verify `cd webapp && npx tsc -b` and that [`webapp/src/types/api.ts`](../../webapp/src/types/api.ts:1) contains the new operations and schemas
- [x] 3.4 Provide the catalog service through Wire (`di` provider, `handler.ServerDeps`, `Server` field) in [`backend/core/cmd/server/wire.go`](../../../backend/core/cmd/server/wire.go:1); run `cd backend/core && go generate ./...` and verify `go build ./...` plus a clean `git diff` of `wire_gen.go`

## 4. Recognition job respects manual curation

- [x] 4.1 Restrict `clusterAssign` to `person_id = 0 AND manual_assignment = false`; verify `TestFaceRecognitionManager_DetachedFaceStaysUnassigned` and `TestFaceRecognitionManager_ManualFaceKeepsPerson` pass
- [x] 4.2 Skip `manual = true` people in `reconcileRound` (neither merged away nor merged into); verify `TestFaceRecognitionManager_ManualPersonSurvivesReconcile`
- [x] 4.3 Key `resetGeneratedPeople` on `manual = false` instead of `name = ''`; verify `TestFaceRecognitionManager_ReclusterKeepsRenamedPerson` and that an unnamed user-created person survives `ReclusterAll`
- [x] 4.4 Verify a merged person's references recognise a source identity in a later pass with `TestFaceRecognitionManager_MergedPersonRecognisedLater`
- [x] 4.5 Verify a person created from unassigned faces keeps references that recognise the same identity in a later pass with `TestFaceRecognitionManager_CreatedPersonRecognisedLater`
- [x] 4.6 Run the job suite: `cd backend/core && go test ./internal/application/faces/... -count=1`

## 5. Webapp API layer and localization

- [x] 5.1 Add `UnassignedFaceDTO`/`UnassignedFacesResponse` and request payload types to [`webapp/src/types/index.ts`](../../webapp/src/types/index.ts:1) matching the Go JSON tags; verify `cd webapp && npm run lint && npx tsc -b`
- [x] 5.2 Add `renamePerson`, `mergePeople`, `createPerson`, `disbandPerson`, `assignFaceToPerson` and `fetchUnassignedFaces` to [`webapp/src/api/endpoints.ts`](../../webapp/src/api/endpoints.ts:1); verify `cd webapp && npm run lint && npx tsc -b`
- [x] 5.3 Add all catalog labels, dialog copy, confirmations and empty states to [`webapp/src/i18n/translations.en.ts`](../../webapp/src/i18n/translations.en.ts:1) and [`webapp/src/i18n/translations.ru.ts`](../../webapp/src/i18n/translations.ru.ts:1) together; verify `cd webapp && npm test -- translations`
- [x] 5.4 Add `refresh()` to [`webapp/src/hooks/useImageFaces.ts`](../../webapp/src/hooks/useImageFaces.ts:1) driven by a nonce that re-issues the request for the active path; verify the new test in `useImageFaces.test.tsx` observes a second fetch and the updated faces
- [x] 5.5 Run `cd webapp && npm run lint && npx tsc -b && npm test -- useImageFaces`

## 6. Workspace People tab — catalog tools

- [x] 6.1 Add the name search field with case-insensitive filtering (localized placeholder included) and a no-matches message; verify the PeopleTab tests for filtering and no-match states pass
- [x] 6.2 Add tile selection with `aria-pressed`, the merge action enabled only for two or more people, and `MergePeopleDialog` showing the resulting name and distinct photo count; verify the merge tests assert the single merged tile and the disabled single-selection state
- [x] 6.3 Add `RenamePersonDialog` to the tile action and the person detail header, refreshing the name in place and sending no request on cancel; verify the rename and cancel tests pass
- [x] 6.4 Add `DisbandPersonDialog` with the photos-are-kept copy, destructive confirmation and cancel path; verify the disband tests assert the tile disappears only after confirmation
- [x] 6.5 Surface failed rename/merge/disband requests through the toast helper and keep the previous grid state; verify the PeopleTab failure tests assert the view is unchanged and the error is reported
- [x] 6.6 Run `cd webapp && npm run lint && npx tsc -b && npm test -- PeopleTab`

## 7. Workspace People tab — unassigned faces

- [x] 7.1 Add `UnassignedFacesView` as a People-tab mode: paginated thumbnail grid with the face box scaled like `PeopleImagePanel`, multi-select and the empty state; verify its tests render one entry per face, scale the box and page forward
- [x] 7.2 Add the attach flow through `PersonPickerDialog` (searchable people list) and refresh the grid and the person's count afterwards; verify the attach test asserts the face leaves the view and the person gains the photo
- [x] 7.3 Add the "create person from selected faces" path with a name field; verify the test asserts the new tile appears in the People grid and the faces leave the unassigned view
- [x] 7.4 Run `cd webapp && npm run lint && npx tsc -b && npm test -- UnassignedFaces`

## 8. Unified lightbox People corrections

- [x] 8.1 Turn each face box into a focusable button with an accessible label and visible selected state (pointer events enabled only in People mode); verify `PeopleImagePanel.test.tsx` asserts one button per face and the selected state
- [x] 8.2 Add the face action dialog with "assign to person…" and "this is not <person>" (assigned faces) and "assign to person…" (unassigned), where the picker lists the whole catalog (not only the photo's people), calling `assignFaceToPerson` then `refresh()`; verify the tests assert renumbered boxes, the muted detached box and the compacted people list
- [x] 8.3 Add the rename action to each [`PeoplePanel.tsx`](../../webapp/src/components/gallery/lightbox/PeoplePanel.tsx:1) entry and refresh the panel and the People grid afterwards; verify `PeoplePanel.test.tsx` asserts the updated name and that the rename callback fires with the person id
- [x] 8.4 Add the "create new person" path for an unassigned face in the lightbox face action dialog and refresh the People grid afterwards; verify the test asserts the box gains the new person's number and the grid shows the new person
- [x] 8.5 Run `cd webapp && npm run lint && npx tsc -b && npm test -- PeoplePanel PeopleImagePanel UnifiedLightbox`

## 9. Evidence binding and integration verification

- [x] 9.1 Bind every new `focused-spec:id` in the change's spec deltas to its exact test in [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) (`go::` for core, `vitest::` for webapp); verify `./node_modules/.bin/focused-spec validate --scope current --strict`
- [x] 9.2 Prove each new evidence binding detects failure with a controlled product regression (not a test edit), then restore it; verify `./node_modules/.bin/focused-spec run --scope current` reports PASS for every new id. Controlled regressions were run per product layer (catalog service, recognition job, PeopleTab grid, unassigned view, lightbox list/panels, localization), each observed FAIL, restored, and all 54 new bindings then reported PASS.
- [x] 9.3 Run the full component suites: `cd backend/core && go test ./... -count=1` and `cd webapp && npm run lint && npx tsc -b && npm test`
- [ ] 9.4 Manually walk the four screens (grid rename/merge/disband/search, unassigned attach/create, lightbox rename/reassign/detach/attach) against a seeded gallery and confirm the spec scenarios hold

## Workflow follow-up

- Run the SDD sync step to publish the delta requirements into `openspec/specs/face-recognition/spec.md` and `openspec/specs/face-clustering/spec.md`.
- Archive the change once the project review requirements are satisfied, and verify the archived result.
