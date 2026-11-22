 # Tasks

## 1. Change bookkeeping

- [x] 1.1 Create the feature branch (`git switch -c person-name-suggestions`) before the first code edit and verify `git branch --show-current` reports it.
- [x] 1.2 Run `node_modules/.bin/focused-spec instructions --protocol 1 --topic workflow --json` from the repository root and confirm the response reports `protocolVersion: 1` and package `focused-spec` before any evidence binding.

## 2. Suggestion helper

- [x] 2.1 Add `PERSON_NAME_SUGGESTION_LIMIT = 7` and `personNameSuggestions(people, query, options)` to [`webapp/src/lib/person-name.ts`](../../../webapp/src/lib/person-name.ts:1), reusing `normalizePersonName` for both the query and each name, dropping unnamed and excluded people, sorting by `photoCount` descending with a `localeCompare` tie-break, filtering by normalized substring when the query is non-empty, and slicing to the limit; verify with `cd webapp && npx vitest run src/lib/person-name.test.ts`.
- [x] 2.2 Extend [`webapp/src/lib/person-name.test.ts`](../../../webapp/src/lib/person-name.test.ts:1) with one case per spec scenario — top-ranked seven, equal-count tie-break, fragment filter, case/space-insensitive match, no-match result, exclusion of the renamed person, unnamed people skipped — named exactly as the selectors in [`design.md`](design.md) (“ranks named people by photo count”, “breaks equal photo counts by name”, “keeps only names containing the fragment”, “ignores case and surrounding spaces”, “returns nothing when no name contains the fragment”, “excludes the person being renamed”, “skips unnamed people”), and verify all seven pass.
- [x] 2.3 Run `cd webapp && npm run lint && npx tsc -b` and confirm both finish without errors.

## 3. Rename dialog

- [x] 3.1 In [`webapp/src/components/people/PeopleDialogs.tsx`](../../../webapp/src/components/people/PeopleDialogs.tsx:64) add optional `people?: PersonDTO[]` and `excludePersonId?: number` to `RenamePersonDialog`, and render, directly under the name input and only when the helper returns entries, a labelled list (`data-slot="rename-suggestions"`, one `data-slot="rename-suggestion"` button per entry showing the name and `people.photoCount`) whose click calls `setName` and nothing else; verify the dialog tests in 3.2 pass.
- [x] 3.2 Extend [`webapp/src/components/people/PeopleDialogs.test.tsx`](../../../webapp/src/components/people/PeopleDialogs.test.tsx:1) with `RenamePersonDialog > fills the name from a suggestion without submitting` (clicking fills the input, `onSubmit` is not called, the dialog stays open) and `RenamePersonDialog > labels suggestions in Russian` (label and entry render in Russian under `language="ru"`), plus a case where no suggestion exists and no list is rendered; verify with `cd webapp && npx vitest run src/components/people/PeopleDialogs.test.tsx`.
- [x] 3.3 Run `cd webapp && npm run lint && npx tsc -b` and confirm both finish without errors.

## 4. Lightbox wiring

- [x] 4.1 In [`webapp/src/components/gallery/UnifiedLightbox.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.tsx:696) pass `people={catalog}` and `excludePersonId={renameTarget.personId}` to the `RenamePersonDialog` rendered for `renameTarget`, leaving the `PeopleTab` call site unchanged; verify the lightbox tests in 4.2 pass.
- [x] 4.2 Extend [`webapp/src/components/gallery/UnifiedLightbox.test.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.test.tsx:215) with `UnifiedLightbox > offers the merge when a suggested name is saved` — `fetchPeople` mocked with more than seven named people, opening the rename dialog from the people list shows seven suggestions with the most-photos name first, clicking one fills the input without calling `renamePerson`, and saving it opens the conflict dialog without creating a duplicate — and verify with `cd webapp && npx vitest run src/components/gallery/UnifiedLightbox.test.tsx`.
- [x] 4.3 Run `cd webapp && npm run lint && npx tsc -b` and confirm both finish without errors.

## 5. Localization

- [x] 5.1 Add `people.suggestions` (Russian and English label of the list) and `people.suggestionOption` (interpolating the offered name) to both [`webapp/src/i18n/translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:1) and [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:1) in the same edit, reusing the existing `people.photoCount` for the count; verify with `cd webapp && npm test -- i18n` that parity and strict key typing pass.

## 6. Documentation

- [x] 6.1 Add a short bullet to [`docs/agent/webapp.md`](../../../docs/agent/webapp.md:1) stating that the lightbox rename dialog ranks catalog-name suggestions through `personNameSuggestions` (client-side, no extra request) and that the new props stay optional so other call sites are unaffected; verify the sentence matches the shipped helper signature after 2.1.

## 7. Focused-spec evidence

- [x] 7.1 Add one entry per new focused-spec ID from the delta to [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) using the exact runner selectors listed in [`design.md`](design.md), then verify with `node_modules/.bin/focused-spec validate --scope current --strict`.
- [x] 7.2 Run each new scenario and confirm `PASS`: `node_modules/.bin/focused-spec run --scope current --scenario face-recognition.lightbox-rename-suggestions-before-typing` (repeat for `…-ties`, `…-filter`, `…-ignores-case`, `…-no-match`, `…-exclude-renamed`, `…-skip-unnamed`, `face-recognition.lightbox-rename-suggestion-fills-input`, `face-recognition.lightbox-rename-suggestion-opens-conflict`, `face-recognition.lightbox-rename-suggestions-localized`).
- [x] 7.3 Prove sensitivity for one representative new evidence reference: remove the descending `photoCount` sort in [`webapp/src/lib/person-name.ts`](../../../webapp/src/lib/person-name.ts:1), rerun `face-recognition.lightbox-rename-suggestions-before-typing` and observe the assertion fail, restore the sort and observe `PASS` again.

## 8. Cross-cutting checks

- [x] 8.1 Run `cd webapp && npm run lint && npx tsc -b && npm test` and confirm the whole webapp suite is green.
- [x] 8.2 Run `openspec validate person-name-suggestions --strict` and confirm the change is valid.

## 9. Backend: named-people suggestion catalog

- [x] 9.1 Add `ListNamed() ([]domain.Person, error)` to [`person_repo.go`](../../../backend/core/internal/domain/repository/person_repo.go:9) selecting only `name <> ''` ordered by id, and cover it in [`person_repo_test.go`](../../../backend/core/internal/domain/repository/person_repo_test.go:1) (named rows returned, unnamed skipped, id order); verify `cd backend/core && go test ./internal/domain/repository/... -count=1`.
- [x] 9.2 Extract the DTO-building loop of `handleGetPeople` ([`handlers_faces.go`](../../../backend/core/internal/interfaces/handler/handlers_faces.go:20) ) into `personDTOs`, add `handleGetPersonSuggestions` using `ListNamed`, and register `protected.GET("/people/suggestions", …)` in [`router.go`](../../../backend/core/internal/interfaces/handler/router.go:149); cover the new handler in [`handlers_faces_test.go`](../../../backend/core/internal/interfaces/handler/handlers_faces_test.go:1) (named only, people without photos skipped), verify `cd backend/core && go test ./internal/interfaces/... -count=1`.
- [x] 9.3 Document `GET /api/people/suggestions` (reusing `PeopleResponse`) in [`docs/api-contracts/core.yaml`](../../../docs/api-contracts/core.yaml:1), run `make generate-types`, and verify `cd webapp && npx tsc -b`.
- [x] 9.4 Run `cd backend/core && go build ./... && go vet ./... && go test ./... -count=1`.

## 10. Webapp: suggestions at open

- [x] 10.1 Add `fetchPersonSuggestions(): Promise<PeopleResponse>` next to `fetchPeople` in [`webapp/src/api/endpoints.ts`](../../../webapp/src/api/endpoints.ts:573) and verify `cd webapp && npx tsc -b`.
- [x] 10.2 In [`UnifiedLightbox.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.tsx:82) load `fetchPersonSuggestions()` in the rename handler into a `suggestionCatalog` state and pass it to `RenamePersonDialog`; add a lightbox test that opens the rename dialog while `fetchPeople` is still pending and sees chips, verify with `cd webapp && npx vitest run src/components/gallery/UnifiedLightbox.test.tsx`.
- [x] 10.3 In [`PeopleDialogs.tsx`](../../../webapp/src/components/people/PeopleDialogs.tsx:77) keep the input "pristine" until the user edits it, feeding `personNameSuggestions` an empty query until then; add `RenamePersonDialog > opens with the seven most-photographed names`, `> shows the top names when the field already holds a name`, and `> hides the chips when the edited fragment matches nothing` to [`PeopleDialogs.test.tsx`](../../../webapp/src/components/people/PeopleDialogs.test.tsx:140) and verify they pass.
- [x] 10.4 Run `cd webapp && npm run lint && npx tsc -b` and confirm both finish without errors.

## 11. Uniform suggestion chips

- [x] 11.1 Create [`webapp/src/components/ui/suggestion-chip.tsx`](../../../webapp/src/components/ui/suggestion-chip.tsx:1) with the AI assistant's pill classes, an optional icon, `data-slot="suggestion-chip"`, plus `suggestion-chip.test.tsx` asserting `SuggestionChip > renders the shared pill styles`; verify `cd webapp && npx vitest run src/components/ui/suggestion-chip.test.tsx`.
- [x] 11.2 Replace the raw suggestion buttons in [`ChatPanel.tsx`](../../../webapp/src/components/gallery/lightbox/ChatPanel.tsx:535) with `SuggestionChip`, render the rename dialog's suggestions as chips labelled only with the name (no heading, no photo count, no `aria-label`), and delete `people.suggestions`/`people.suggestionOption` from [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts:1) and [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts:1) together.
- [x] 11.3 Update the dialog tests to the chip markup and add `RenamePersonDialog > shows only the name on each suggestion chip` (the entry exposes no heading and no count); verify `cd webapp && npm test -- i18n && npx vitest run src/components/people src/components/gallery`.
- [x] 11.4 Run `cd webapp && npm run lint && npx tsc -b` and confirm both finish without errors.

## 12. Design-system documentation

- [x] 12.1 Update the suggestion bullet in [`docs/agent/webapp.md`](../../../docs/agent/webapp.md:99) to name `fetchPersonSuggestions`, the pristine-query rule and the shared `SuggestionChip`, and verify the text matches the shipped props and endpoint.
- [x] 12.2 Record in [`docs/design-system.md`](../../../docs/design-system.md:1) that the ChatPanel and rename suggestions now go through `SuggestionChip`, and verify no remaining suggestion drift is listed.

## 13. Evidence and final checks

- [x] 13.1 Update [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1): drop `face-recognition.lightbox-rename-suggestions-localized`, rebind the changed scenarios and add the two `design-system.suggestion-chips-*` entries from [`design.md`](design.md); verify `node_modules/.bin/focused-spec validate --scope current --strict`.
- [x] 13.2 Run every affected scenario with `node_modules/.bin/focused-spec run --scope current --scenario <id>` and confirm all report `PASS`.
- [x] 13.3 Prove sensitivity for `design-system.suggestion-chips-uniform`: remove the pill classes from [`suggestion-chip.tsx`](../../../webapp/src/components/ui/suggestion-chip.tsx:1), observe the scenario fail, restore and observe `PASS`.
- [x] 13.4 Run `cd webapp && npm run lint && npx tsc -b && npm test && npm run build` and `openspec validate person-name-suggestions --strict`.

## Workflow follow-up

- Sync the delta into [`openspec/specs/face-recognition/spec.md`](../../../openspec/specs/face-recognition/spec.md:1) after the reviewer approves the implementation, then re-run `focused-spec validate --scope current --strict`.
- Archive the change once the review requirements are satisfied and verify the archived result.
