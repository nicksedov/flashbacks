# Enforce Unique Person Names — Tasks

## 0) Change bookkeeping
- [x] Write `.openspec.yaml` (`schema: spec-driven`, `created: 2026-11-20`) next to this file, because the scaffold was authored by hand (no shell in the planning session), then run `openspec validate enforce-unique-person-names --strict` and fix anything it reports.
- [x] Run `node_modules/.bin/focused-spec instructions --protocol 1 --topic workflow --json` from the repository root and follow the returned guidance for binding each new `focused-spec:id` scenario in `specs/face-recognition/spec.md` to its evidence; record the evidence in `.focused-spec/evidence.yaml`.
- [x] Create the feature branch for the change (`git switch -c enforce-unique-person-names`) before the first code edit.

## 1) Domain: canonical name and persisted key
- [x] Add `NameKey string \`gorm:"default:'';index" json:"-"\`` to `domain.Person` in [`backend/core/internal/domain/faces.go`](../../../backend/core/internal/domain/faces.go:24) with a comment stating it is the unique identity of the name and is never exposed.
- [x] Add `domain.NormalizePersonName(name string) (display, key string)` (collapse whitespace with `strings.Fields`/`Join`, then `strings.ToLower`) and the comparable `domain.PersonNameKey(name string) string` helper in the same package; an empty result stays empty.
- [x] Keep display name and key in sync through `Person.SetName` instead of GORM hooks: the `domain` package deliberately imports no GORM, and the catalog (the only writer of a non-empty name) sets `name_key` explicitly in its bulk updates, which hooks would not see anyway. Documented in the field comment.
- [x] Add `backend/core/internal/domain/faces_test.go` cases: `""`, `"  Anna  "`, `"Anna   Petrova"`, `"ИВАН"`, `"Иван"`, and a `TestPersonNameKeyMigrates` guard that `AutoMigrate` created the `name_key` column.

## 2) Catalog: uniqueness on every write path
- [x] In [`backend/core/internal/application/people/catalog.go`](../../../backend/core/internal/application/people/catalog.go:26) add the `ErrNameTaken` sentinel plus the exported `ErrNameTakenError` struct carrying `Existing *domain.Person`, with `Error()` and `Unwrap()`.
- [x] Add `(*Catalog).findNameHolder(tx *gorm.DB, key string, excludeIDs []uint) (*domain.Person, error)` querying `people` by `name_key` (skip when `key == ""`, skip ids in `excludeIDs`).
- [x] `Rename` ([`catalog.go`](../../../backend/core/internal/application/people/catalog.go:104)): compute `display`/`key` with `NormalizePersonName`, check the holder with `excludeIDs = []uint{personID}`, and write `name` **and** `name_key` in the same update map.
- [x] `Merge` ([`catalog.go`](../../../backend/core/internal/application/people/catalog.go:130)): when a name is supplied, check the holder with `excludeIDs = target + sources` and write `name_key` with the name; without a name leave both untouched.
- [x] `CreateFromFaces` ([`catalog.go`](../../../backend/core/internal/application/people/catalog.go:217)): check the holder with no exclusions before creating anything; set `NameKey` on the new person.
- [x] Map a driver unique-violation on `people.name_key` to `ErrNameTakenError` (concurrent request lost the race) instead of returning a 500.
- [x] Extend [`backend/core/internal/application/people/catalog_test.go`](../../../backend/core/internal/application/people/catalog_test.go:1) with table-driven cases: duplicate on rename, duplicate on create, duplicate on merge, merge name equal to the target's name, merge name equal to a source's name, self-rename with different case, whitespace-variant duplicate, unnamed exemption.
- [x] Run `cd backend/core && go test ./internal/application/people/... ./internal/domain/... -count=1`.

## 3) Startup repair and the database guarantee
- [x] Add `backend/core/internal/infrastructure/database/people_names.go` with `migratePersonNames`: backfill `name_key` for rows whose stored key does not match the computed one, group duplicates by `name_key`, merge each group into its lowest id (faces move with `manual_assignment = true`, cover adopted, exemplars unioned and capped at five, losers deleted, survivor marked `manual = true`), and log every merge and every skipped group.
- [x] Call `migratePersonNames` from [`database.go`](../../../backend/core/internal/infrastructure/database/database.go:52) right after `AutoMigrate`, then create the partial index: `CREATE UNIQUE INDEX IF NOT EXISTS idx_people_name_key_unique ON people (name_key) WHERE name_key <> ''`, logging a warning instead of failing startup when unresolved duplicates keep it from being created (same policy as `idx_users_login_lower` at [`database.go`](../../../backend/core/internal/infrastructure/database/database.go:102)).
- [x] Add `backend/core/internal/infrastructure/database/people_names_test.go` (SQLite): two people with the same normalized name become one under the lower id with both faces, a second run changes nothing, unnamed people are untouched, and a direct insert of a duplicate `name_key` is rejected by the index.
- [x] Run `cd backend/core && go test ./internal/infrastructure/database/... -count=1`.

## 4) HTTP layer: DTO, 409 mapping, messages
- [x] Add the conflict DTO to [`backend/core/internal/interfaces/dto/media.go`](../../../backend/core/internal/interfaces/dto/media.go:934): `PersonNameConflictResponse` (`error` + `conflict PersonDTO`) used by the three endpoints.
- [x] Map the new error in `respondCatalogError` ([`handlers_faces.go`](../../../backend/core/internal/interfaces/handler/handlers_faces.go:410)): `errors.As` for `*people.ErrNameTakenError` → `409` with the `conflict` payload rendered through the existing `personDTO` helper; keep 404/400 branches untouched.
- [x] Add `MsgPeopleNameTaken MessageKey = "people.name_taken"` to [`backend/core/internal/interfaces/i18n/messages.go`](../../../backend/core/internal/interfaces/i18n/messages.go:210) and the matching entry to `locales/en.json` and `locales/ru.json` (both files, same change).
- [x] Extend [`backend/core/internal/interfaces/handler/handlers_people_test.go`](../../../backend/core/internal/interfaces/handler/handlers_people_test.go:34) with: rename → 409 and unchanged row, create → 409 with faces still unassigned, merge → 409 with nothing merged, and the `conflict` field content (id, name, photoCount, cover).
- [x] Run `cd backend/core && go test ./internal/interfaces/... -count=1 && go vet ./... && go build ./...`.

## 5) API contract
- [x] Document the `409` response and the `PersonNameConflictResponse` schema for `PATCH /api/people/{id}`, `POST /api/people` and `POST /api/people/merge` in [`docs/api-contracts/core.yaml`](../../../docs/api-contracts/core.yaml:1491).
- [x] Run `make generate-types` and commit the refreshed [`webapp/src/types/api.ts`](../../../webapp/src/types/api.ts:1); verify `cd webapp && npx tsc -b`.

## 6) Webapp API client and name helper
- [x] Add `export class ApiError extends Error { readonly status: number; readonly body: unknown }` in [`webapp/src/api/client.ts`](../../../webapp/src/api/client.ts:54); throw it from `handleResponse` for every non-ok response with the translated message unchanged.
- [x] Make `apiPost`, `apiPatch`, `apiPut`, `apiDelete` and `apiUpload` re-throw an `ApiError` unchanged instead of wrapping it in `translateNetworkError`, so `status` and `body` survive.
- [x] Add `webapp/src/lib/person-name.ts` with `normalizePersonName(name: string): string` (trim, collapse inner whitespace, lower-case) and `findPersonNameConflict(name, people, excludeIds)`, plus `nameConflictFromError` reading the 409 body.
- [x] Add `webapp/src/lib/person-name.test.ts` covering the same normalization table as the Go test, the exclusion of the person being renamed, the empty-name exemption and the 409 body.
- [x] Run `cd webapp && npx vitest run src/lib/person-name.test.ts`.

## 7) Conflict dialog
- [x] Add `PersonNameConflictDialog` to [`webapp/src/components/people/PeopleDialogs.tsx`](../../../webapp/src/components/people/PeopleDialogs.tsx:1) with props `pendingName`, `conflict`, `mode` (`merge-person` | `merge-selection` | `attach-faces`), `loading`, `onMerge`, `onChangeName`, `onCancel`; show the existing person's name and photo count, and the two alternatives as the only actions.
- [x] Add `data-slot` hooks (`name-conflict`, `name-conflict-merge`, `name-conflict-rename`, `name-conflict-cancel`) for the tests, matching the existing dialog conventions.
- [x] Add `webapp/src/components/people/PeopleDialogs.test.tsx`: renders both alternatives, merge calls `onMerge`, change-name calls `onChangeName`, cancel calls `onCancel`, the words follow the mode, and the dialog is localized in Russian.

## 8) Lightbox wiring
- [x] In [`webapp/src/components/gallery/UnifiedLightbox.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.tsx:225) run `findPersonNameConflict` against the catalog (loaded while the People tab is open) before `renamePerson`, and on a 409 `ApiError` open `PersonNameConflictDialog` with `conflict` from the body.
- [x] Implement the merge alternative as `mergePeople({ targetPersonId: conflict.id, sourcePersonIds: [renameTarget.personId] })`, then `refreshFaces()` + `onPeopleChanged?.()` and close both dialogs.
- [x] Implement change-name by reopening `RenamePersonDialog` with the typed name and nothing saved; cancelling abandons the flow.
- [x] Apply the same treatment to `handleCreateFromFace` ([`UnifiedLightbox.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.tsx:204)) with `mode="attach-faces"`: merge means `assignFaceToPerson(selectedFace.id, conflict.id)`.
- [x] Extend [`webapp/src/components/gallery/UnifiedLightbox.test.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.test.tsx:213) with: conflicting rename opens the dialog and sends no rename, merge calls `mergePeople` and refreshes faces, create-from-face with a taken name creates nothing.

## 9) People tab wiring
- [x] `handleRenameSubmit` ([`PeopleTab.tsx`](../../../webapp/src/components/tabs/PeopleTab.tsx:150)): pre-check against `people`, handle the 409 with the conflict dialog, and on merge fold the renamed person into the holder (drop it from `people`, replace the holder with the returned person, clear the detail selection).
- [x] `handleMergeSubmit` ([`PeopleTab.tsx`](../../../webapp/src/components/tabs/PeopleTab.tsx:170)): when the typed name belongs to a third person, offer the conflict dialog and on merge re-issue the request with `targetPersonId = conflict.id` and the former target added to the sources; an explicit override target skips the pre-check so the dialog cannot loop.
- [x] `handleCreate` in [`UnassignedFacesView.tsx`](../../../webapp/src/components/people/UnassignedFacesView.tsx:160): pre-check the name, and on the attach alternative assign every selected face to `conflict.id` with the existing `assignFaceToPerson` loop, then drop them from the unassigned list.
- [x] Extend [`webapp/src/components/tabs/PeopleTab.test.tsx`](../../../webapp/src/components/tabs/PeopleTab.test.tsx:220) (rename conflict, merge into holder, change name, cancel, selection merge) and [`UnassignedFacesView.test.tsx`](../../../webapp/src/components/people/UnassignedFacesView.test.tsx:1) (attach alternative).
- [x] Run `cd webapp && npx vitest run src/components/people src/components/tabs/PeopleTab.test.tsx src/components/gallery/UnifiedLightbox.test.tsx`.

## 10) Localization
- [x] Add the conflict strings to [`webapp/src/i18n/translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:58) and [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:58) in the `people.*` namespace: title, description with the pending name and the holder's photo count, hint, merge action (three wordings by mode), change-name action.
- [x] Verify the strict `TranslationKey` type in [`webapp/src/i18n/types.ts`](../../../webapp/src/i18n/types.ts:1) picks the keys up and that the existing parity tests (`translations.test.ts`, `activeLanguage.test.ts`) pass.
- [x] Run `cd webapp && npm test -- i18n`.

## 11) Documentation
- [x] Add the uniqueness rule to the face-catalog bullets of [`docs/agent/go-services.md`](../../../docs/agent/go-services.md:61): a person name is unique after normalization; every write path checks it; the database carries the partial unique index; never compare names with SQL `lower()`.
- [x] Add a line to the API-integration section of [`docs/agent/webapp.md`](../../../docs/agent/webapp.md:1) stating that a taken name is resolved through the conflict dialog, never saved, and that `ApiError` must be re-thrown intact.

## 12) Cross-cutting checks and completion
- [x] Run `cd backend/core && go build ./... && go vet ./... && go test ./... -count=1`.
- [x] Run `cd webapp && npm run lint && npx tsc -b && npm test`.
- [x] Re-run the focused-spec evidence binding for every new scenario (`focused-spec run --scope current --scenario face-recognition.<id>` — 27 bindings, all PASS) and prove sensitivity with two controlled regressions: dropping the rename pre-check fails `TestCatalogRenameRejectsTakenName`, dropping the case-fold fails `TestPersonNameKey`; both restored and green again.
- [x] Sync the delta into [`openspec/specs/face-recognition/spec.md`](../../../openspec/specs/face-recognition/spec.md:1) (`openspec validate --specs`: 22 passed, 0 failed) and re-run `focused-spec validate --scope current --strict`; the remaining duplicate-marker reports are the active delta restating the synced main spec and disappear when the change is archived.
- [ ] Verify the reported defect end to end in the running app: UnifiedLightbox → People → unnamed person → Rename → an existing name shows the conflict dialog, merge folds the cluster into the existing person, and the People grid lists exactly one person with that name. Left for the user: this session has no running app/browser session, and the behavior is covered by the Vitest and Go suites listed above.
