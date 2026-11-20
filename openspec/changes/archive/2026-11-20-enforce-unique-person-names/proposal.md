# Proposal

## Why

The face catalog lets the user name people, but it never checks whether a name is
already in use. In the unified lightbox (People tab → an unnamed person →
Rename) typing a name that another person already carries creates **a second
person with the same name**: the grid and every person picker then show two
identical «Иван» entries, the user cannot tell which cluster holds which photos,
and the very cluster the user meant to fold into an existing person stays apart.
The shipped spec even asserts this as intended
([`face-recognition.spec.md`](../../../openspec/specs/face-recognition/spec.md:493), scenario
`person-name-duplicate-allowed`).

A person's name is the only identity the catalog exposes, so two people sharing
one name is not a cosmetic defect — it makes every name-based decision
(reassignment pickers, attach-to-person, search) ambiguous. The duplicate is
almost always one of two intents, and the user must be asked which one:

- it is the same human being, and the new photos should join the existing person
  under that one name (**merge**);
- it is a different human being, and the name has to be different
  (**choose another name**).

## What Changes

- Introduce a canonical form of a person's name — trimmed, internal whitespace
  collapsed, Unicode case-folded («Иван» = «иван» = «ИВАН» = « Иван ») — and
  persist it next to the display name in a new `people.name_key` column.
- Make the name unique across the catalog on **every** write path:
  - `PATCH /api/people/:id` (rename, also the lightbox rename),
  - `POST /api/people` (create a person from unassigned faces),
  - `POST /api/people/merge` (the merge result name).
  A collision answers **409** with the error key `people.name_taken` and a
  `conflict` object holding the existing person (`id`, `name`, `photoCount`,
  `coverThumbnail`). Renaming a person to its own name, clearing a name, and
  leaving people unnamed stay allowed; unnamed people never collide.
- Guarantee the invariant in the database as well: a partial `UNIQUE` index on
  `name_key` where the key is not empty, plus a one-shot startup repair that
  backfills the keys and merges pre-existing duplicate names into the oldest
  person (lowest id) so the index can be created.
- Surface the conflict in the UI instead of saving it. When the typed name is
  taken, the client shows a conflict dialog with the existing person and the two
  alternatives the user asked for:
  - **merge** — the current cluster (or the selected faces, or the whole
    selection) joins the existing person, reusing `POST /api/people/merge` and
    the existing attach-faces path;
  - **choose another name** — back to the input, nothing saved.
  Wired into the lightbox People tab (rename, create-from-face) and the
  workspace People tab (rename, create from faces, merge with a name).
- Add the `ApiError` type (`status` + response body) to the webapp API client so
  a 409 can be told apart from a generic failure, and a shared
  `findPersonNameConflict` helper so the loaded catalog is checked before the
  request is sent (the server remains authoritative).
- Document the 409 responses and the `PersonNameConflict` schema in
  [`docs/api-contracts/core.yaml`](../../../docs/api-contracts/core.yaml:1) and
  regenerate [`webapp/src/types/api.ts`](../../../webapp/src/types/api.ts:1).
- Add every new string in English and Russian on both sides (core locale JSON and
  webapp translations) and cover the new behavior with Go and Vitest tests.

## Capabilities

### New Capabilities

None — the change extends the existing `face-recognition` capability.

### Modified Capabilities

- `face-recognition`: the rename, merge and create-from-faces requirements gain
  the uniqueness and 409 conflict contract; the `person-name-duplicate-allowed`
  scenario is replaced by uniqueness scenarios; a new requirement states the
  catalog invariant and the startup repair of legacy duplicates; new
  requirements describe the conflict dialog and its two alternatives on the
  lightbox and on the People tab.

## Non-goals

- No fuzzy matching, transliteration or nickname awareness («Иван» and «Иван
  Петров» stay different names; «Vanya» and «Иван» are not compared).
- No merge suggestion based on appearance: the user still decides between merge
  and a new name.
- No automatic renaming or deletion of people to resolve a conflict — the only
  automatic action is the startup merge of **identically named** legacy
  duplicates into the oldest person.
- No change to face detection, embeddings, thresholds or clustering.
- No history/audit trail of renames and merges.
- No new role or permission: every authenticated user may curate the shared
  catalog, as today.
- No server-side name search or sort; the catalog is already loaded on both
  surfaces that resolve a conflict.

## Impact

- **Backend (`backend/core`)**: `domain.Person` gains `NameKey` with a
  `BeforeCreate`/`BeforeSave` hook and a `NormalizePersonName` helper;
  `application/people/catalog.go` enforces uniqueness in `Rename`,
  `CreateFromFaces` and `Merge` and returns a typed `ErrNameTaken` carrying the
  conflicting person; `interfaces/handler/handlers_faces.go` maps it to 409 and
  builds the `conflict` payload; `interfaces/dto/media.go` gains the conflict
  schema; `interfaces/i18n/{messages.go,locales/en.json,locales/ru.json}` gain
  `people.name_taken`; `infrastructure/database/database.go` and a new
  `people_names.go` run the backfill, the duplicate merge and the partial unique
  index.
- **API contract**: `docs/api-contracts/core.yaml` gains the 409 responses and
  the `PersonNameConflict` schema; `make generate-types` refreshes
  `webapp/src/types/api.ts`.
- **Webapp**: `api/client.ts` gains `ApiError` (and must stop wrapping it into a
  plain `Error`); new `lib/person-name.ts`; `components/people/PeopleDialogs.tsx`
  gains `PersonNameConflictDialog`; `components/tabs/PeopleTab.tsx` and
  `components/gallery/UnifiedLightbox.tsx` resolve conflicts on their name-entry
  paths; `api/endpoints.ts`, `types/index.ts`,
  `i18n/translations.en.ts` + `i18n/translations.ru.ts`; Vitest tests.
- **Design system**: the conflict dialog is an ordinary dialog following the
  existing rules (44×44 touch targets, no destructive styling for merge,
  localized copy) — no new design-system requirement.
- **Compatibility**: `RenamePersonRequest`/`CreatePersonRequest`/
  `MergePeopleRequest` keep their shape; only a new 409 response and a new
  response field appear, so older clients keep working but will show the
  localized 409 message instead of a duplicate name.
