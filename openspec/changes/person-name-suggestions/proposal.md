# Proposal

## Why

Renaming in the lightbox People tab starts from an input with no visibility into
the names the catalog already holds. The catalog is unique by name, so typing a
near-duplicate of an existing name either misses the intended person or lands in
the merge-conflict dialog, and long names invite typos. Offering the catalog's
own names as suggestions makes the rename flow faster and keeps the catalog
tidy.

The first implementation shipped with two defects: the list was not visible when
the dialog opened (it waited for the full catalog, which in production holds over
1200 people and only about 20 named ones, so every row costs a separate photo
count query), and its entries did not match the suggestion chips the AI assistant
already uses.

## What Changes

- The rename dialog of the lightbox People tab shows suggestion chips under the
  name input **as soon as it opens**: up to 7 named people with the most photos,
  even when the field starts with the person's current name. The list starts
  filtering only after the user edits the field, so opening the dialog always
  offers the most-used names first.
- Once the user edits the field, it offers up to 7 named people whose name
  contains the typed fragment (case- and whitespace-insensitive, matching how
  names are compared for uniqueness).
- The suggestions come from a new lightweight catalog read that returns **only
  named people with their photo counts**, requested when the dialog opens,
  instead of waiting for the full people listing (which counts photos per person
  and is slow at production scale).
- Suggestion entries render through the same shared chip primitive the AI
  assistant's prompt suggestions use: a name-only pill, no list heading, no photo
  count, no bespoke styling.
- Choosing a suggestion fills the name input; it does not save. The user still
  presses Save, so an existing name reaches the name-conflict dialog, which
  already offers merging into the person that holds it.
- The person being renamed and unnamed people are never suggested.
- The now-unused suggestion labels are removed from both locales.
- The design system gains a requirement that every inline suggestion list uses
  one chip primitive.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `face-recognition`: the lightbox rename dialog shows suggestions at open,
  draws them from the named-people catalog, and renders them as chips.
- `design-system`: new requirement for a single suggestion-chip primitive shared
  by every inline suggestion list.

## Non-goals

- No change to name uniqueness, normalization or the conflict dialog.
- No optimization of `GET /api/people` itself: the People grid keeps its current
  listing and photo counting. The new read is additive and serves suggestions
  only.
- No server-side search or ranking: the endpoint returns the named people, and
  the client ranks and filters the small set it receives.
- No suggestions in the workspace People tab, create-person or merge dialogs.
- No change to the AI assistant's prompt texts or behaviour; only its chip
  rendering moves to the shared primitive.

## Impact

- `backend/core`: a repository read for named people, a suggestions handler and
  route, plus handler/repository tests.
- `docs/api-contracts/core.yaml` and `webapp/src/types/api.ts` (generated).
- `webapp/src/api/endpoints.ts` — suggestion catalog request.
- `webapp/src/components/ui/suggestion-chip.tsx` (new) and
  `webapp/src/components/gallery/lightbox/ChatPanel.tsx` — shared chip.
- `webapp/src/components/people/PeopleDialogs.tsx` — chips, pristine-query rule,
  no heading or counts.
- `webapp/src/components/gallery/UnifiedLightbox.tsx` — loads the suggestion
  catalog when the rename dialog opens.
- `webapp/src/i18n/translations.en.ts` / `.ru.ts` — remove the two suggestion
  labels.
- `docs/agent/webapp.md` and `docs/design-system.md` — the uniform chip rule and
  its drift record.
- Webapp and Go unit tests for all of the above.
