# Proposal

## Why

Face recognition groups faces into people, but the catalog it produces is
read-only: every cluster keeps its generated label, a person split over several
clusters cannot be joined back together, and a face the algorithm put on the
wrong person can only be fixed by re-running the whole job. On a real family
gallery that means the People view stays a list of "Person 12345" tiles, and one
misclustered face silently pollutes an otherwise correct person.

The catalog is only useful when the user can curate it: name the people, merge
the clusters that are one person, attach the faces the algorithm left
unassigned, and correct an individual photo without discarding the rest of the
work.

## What Changes

- Add catalog mutations to the core API, all under the existing authenticated
  `/api` group:
  - `PATCH /api/people/:id` renames a person (trimmed name; an empty name makes
    the person unnamed again).
  - `POST /api/people/merge` merges one or more people into a target person,
    optionally setting the resulting name.
  - `POST /api/people` creates a person from a set of previously unassigned
    faces, optionally with a name.
  - `DELETE /api/people/:id` disbands a cluster: its faces become unassigned
    (`personId` 0) and the person row is removed.
  - `PATCH /api/faces/:id` assigns a face to a person or, with `personId` 0,
    detaches it from its person.
  - `GET /api/faces/unassigned` lists the faces that belong to no person,
    paginated, each with its image path and box.
- Mark user-curated state so the background job stops undoing it: a person and a
  face carry a manual flag, manual people are excluded from automatic
  reconciliation, and manual face assignments are never reassigned by a
  clustering pass. Merges, detachments and deletes keep the person-reference
  exemplars consistent with the resulting people.
- Workspace **People** tab gains the catalog tools:
  - rename and delete actions on a person tile and on the person detail header;
  - multi-select merge with a confirmation that shows the resulting name and
    photo count;
  - a search field that filters the grid by name;
  - an **Unassigned faces** view (paginated) where faces can be selected and
    attached to an existing person or used to create a new named person.
- Unified **lightbox → People** tab becomes editable: rename a person from the
  people list, select a face box and either assign it to another person or
  detach it ("this is not …"), and attach an unnumbered face to a person. The
  boxes and the people list refresh after each correction.
- Add the new OpenAPI paths and schemas to
  [`docs/api-contracts/core.yaml`](../../docs/api-contracts/core.yaml:1) and
  regenerate [`webapp/src/types/api.ts`](../../webapp/src/types/api.ts:1) with
  `make generate-types`.
- Add all new interface strings in English and Russian, plus Vitest/Go unit
  tests for the new behavior.

## Capabilities

### New Capabilities

None — the work extends the existing face-recognition and face-clustering
capabilities.

### Modified Capabilities

- `face-recognition`: adds the catalog-management API (rename, merge, create
  from unassigned faces, delete/disband, assign/detach a face), the unassigned
  faces listing, and the People-tab and lightbox scenarios that expose each
  management action on its screen.
- `face-clustering`: adds the requirement that manual curation is authoritative
  — clustering passes leave manually created/merged people and manually assigned
  faces alone — and that merge, detach and disband keep person-reference
  exemplars consistent.

## Non-goals

- No change to face detection, embedding or the similarity thresholds.
- No automatic suggestion of merges ("these two people look alike"); merging is
  always an explicit user action.
- No face-crop thumbnail pipeline: unassigned faces are shown through the
  existing photo thumbnail with their box, not as new crops on disk.
- No undo/history of catalog edits and no audit trail of who renamed a person.
- No new role model: every authenticated user may curate the shared catalog,
  like every other gallery mutation today.
- No split of an automatically clustered person into two people beyond
  detaching individual faces in the lightbox.
- No server-side sort or search of the people listing; the grid filters the
  already-loaded list.
- No change to the existing People tab grid/empty-state/localization
  requirements beyond the tools added here.

## Impact

- **Backend (`backend/core`)**: new DTOs, handlers and routes in
  `handlers_faces.go` / `router.go`; `PersonRepository` gains delete and
  merge-supporting updates; `FaceRepository` gains listing of unassigned faces
  and person reassignment in bulk; `domain.Person`/`domain.Face` gain manual
  flags (migration on both Postgres and the in-memory SQLite test database);
  the recognition manager's reconcile step skips manual people and manual
  faces; `PersonReferenceRepository` gains moves/deletes used by merge, detach
  and disband; core i18n message constants and en/ru locale entries.
- **API contract**: `docs/api-contracts/core.yaml` gains the new paths and
  schemas; `make generate-types` refreshes `webapp/src/types/api.ts`.
- **Webapp**: `PeopleTab.tsx` (grid tools, search, unassigned view, person
  detail actions), new catalog dialogs (rename, merge, person picker, delete
  confirmation) and an unassigned-faces grid, `PeoplePanel.tsx` and
  `PeopleImagePanel.tsx` (editable list entries and selectable face boxes),
  `useImageFaces.ts` (refresh after a correction), `endpoints.ts`, `types/index.ts`,
  `translations.en.ts` + `translations.ru.ts`, Vitest tests.
- **Design system**: existing dialog, touch-target (44×44), destructive-action
  confirmation and empty-state rules apply to the new surfaces.
