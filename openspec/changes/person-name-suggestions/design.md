# Design

## Context

See [`proposal.md`](proposal.md) for motivation.

Current state that shapes the approach:

- Production holds ~1230 people with photos but only ~20 of them have a name.
- `GET /api/people` (`handleGetPeople`) lists every person and counts its photos
  with one repository call per person, so the response is large and slow; the
  lightbox loads it for the assignment picker, and a rename dialog opened early
  has no catalog to suggest from — the first reported defect.
- `RenamePersonDialog` is shared by the lightbox and the workspace People tab and
  keeps the typed name in local state, seeded from the person's current name.
- `personNameSuggestions` in `webapp/src/lib/person-name.ts` already ranks by
  photo count, filters by normalized substring, skips unnamed and excluded
  people, and caps at seven; its behaviour is unchanged by this revision.
- The AI assistant renders its prompt suggestions in `ChatPanel` as inline pills
  with `rounded-full border-border/70 text-xs text-muted-foreground
  hover:bg-muted`; the design system records raw `<button>` usage as drift.

## Goals / Non-Goals

**Goals:**

- The suggestion chips are on screen the moment the rename dialog opens,
  independently of how long the full people listing takes.
- One chip appearance for every suggestion surface.
- Keep the existing ranking/filtering rules and their unit tests.

**Non-Goals:**

- No change to `GET /api/people`, the People grid, or the merge/conflict flows.
- No server-side substring search: the endpoint returns the named set, the client
  ranks and filters it.
- No translations for suggestion entries: a chip shows a person's name, which is
  data, not UI copy.
- No suggestions in the workspace People tab, create-person or merge dialogs.

## Decisions

### A dedicated named-people read instead of the full catalog

Add `GET /api/people/suggestions` returning `PeopleResponse`: every person with a
non-empty name that still has at least one photo, ordered by id. The handler
reuses the existing DTO-building loop (extracted as `personDTOs`), so it counts
photos only for named people — tens of rows in production instead of 1230.

Repository: `PersonRepository.ListNamed() ([]domain.Person, error)` →
`WHERE name <> '' ORDER BY id`. Route: `protected.GET("/people/suggestions", …)`,
a static sibling of the existing `/people/merge` and `/people/:id/photos`, which
gin already resolves.

Alternatives considered:

- **Reuse the full catalog and fix its N+1 count:** fixes the grid too, but turns
  this suggestion change into a listing-performance change with a wider blast
  radius; recorded as a follow-up instead.
- **Server-side `?query=&limit=7` search:** would need a request per keystroke
  and would move the ranking rules (and their tests) to the backend for no
  benefit on a set of this size.

### The lightbox fetches suggestions when the dialog opens

`UnifiedLightbox` gains `suggestionCatalog` state and loads it in the rename
handler before opening the dialog, passing it as `RenamePersonDialog`'s existing
`people` prop. The prop stays the same, so the workspace People tab keeps showing
no suggestions and no prop churn spreads through the other call sites. A failed
suggestion read leaves the list empty rather than blocking the rename — the input
stays usable, and the server remains authoritative for name uniqueness.

### The field is "pristine" until the user edits it

`RenamePersonDialog` tracks whether the input still holds its seeded value and
feeds `personNameSuggestions` an empty query while it does. Opening the dialog on
a person already named "Анна" therefore shows the top seven names instead of the
(matching-nothing) list filtered by "Анна"; the first edit switches the list to
fragment filtering. This makes "suggestions on open" independent of whether the
person is named.

### One `SuggestionChip` primitive

New `webapp/src/components/ui/suggestion-chip.tsx` renders the AI assistant's
pill: `inline-flex items-center gap-1.5 rounded-full border border-border/70
px-2.5 py-1.5 text-xs text-muted-foreground transition-colors hover:bg-muted
hover:text-foreground`, with `data-slot="suggestion-chip"`, an optional leading
icon and a click handler. `ChatPanel` replaces its raw `<button>` chips with it,
and the rename dialog renders each suggestion as a chip labelled with the name —
no group heading, no photo count. The primitive is the sanctioned chip control
(the design system keeps `Button`/`IconButton` for buttons and calls this out as
the one suggestion control).

The dialog drops its `people.suggestions` and `people.suggestionOption` strings,
which removes both locale entries; the chip's accessible name is its own text.

## Risks / Trade-offs

- **The named set can grow** (every renamed cluster adds a row) → it stays far
  smaller than the catalog, and the client caps rendering at seven; if it grows
  into thousands, add a server-side search parameter.
- **Two catalog reads when the dialog opens** (the people listing may already be
  loaded) → the suggestion read is cheap and bounded; correctness at open matters
  more than one extra small request.
- **`ChatPanel` visual touch** → the chip classes are byte-for-byte the classes
  already used there, so the refactor is a pure extraction; the assistant's tests
  and prompt texts are untouched.
- **Chip click keeps the list filtered by the chosen name** → harmless: the
  chosen name stays visible as the only match, and clicking again is idempotent.

## Spec evidence for the apply phase

The evidence registry (`.focused-spec/evidence.yaml`) is updated during apply,
after approval, with these intended selectors:

| Focused-spec ID | Runner selector |
|---|---|
| `face-recognition.lightbox-rename-suggestions-before-typing` | `vitest::webapp/src/components/people/PeopleDialogs.test.tsx::RenamePersonDialog > opens with the seven most-photographed names` |
| `face-recognition.lightbox-rename-suggestions-prefilled-name` | `vitest::webapp/src/components/people/PeopleDialogs.test.tsx::RenamePersonDialog > shows the top names when the field already holds a name` |
| `face-recognition.lightbox-rename-suggestions-ties` | `vitest::webapp/src/lib/person-name.test.ts::personNameSuggestions > breaks equal photo counts by name` |
| `face-recognition.lightbox-rename-suggestions-filter` | `vitest::webapp/src/lib/person-name.test.ts::personNameSuggestions > keeps only names containing the fragment` |
| `face-recognition.lightbox-rename-suggestions-ignores-case` | `vitest::webapp/src/lib/person-name.test.ts::personNameSuggestions > ignores case and surrounding spaces` |
| `face-recognition.lightbox-rename-suggestions-no-match` | `vitest::webapp/src/components/people/PeopleDialogs.test.tsx::RenamePersonDialog > hides the chips when the edited fragment matches nothing` |
| `face-recognition.lightbox-rename-suggestions-exclude-renamed` | `vitest::webapp/src/lib/person-name.test.ts::personNameSuggestions > excludes the person being renamed` |
| `face-recognition.lightbox-rename-suggestions-skip-unnamed` | `vitest::webapp/src/lib/person-name.test.ts::personNameSuggestions > skips unnamed people` |
| `face-recognition.lightbox-rename-suggestion-fills-input` | `vitest::webapp/src/components/people/PeopleDialogs.test.tsx::RenamePersonDialog > fills the name from a suggestion without submitting` |
| `face-recognition.lightbox-rename-suggestion-opens-conflict` | `vitest::webapp/src/components/gallery/UnifiedLightbox.test.tsx::UnifiedLightbox > offers the merge when a suggested name is saved` |
| `design-system.suggestion-chips-uniform` | `vitest::webapp/src/components/ui/suggestion-chip.test.tsx::SuggestionChip > renders the shared pill styles` |
| `design-system.suggestion-chips-text-only` | `vitest::webapp/src/components/people/PeopleDialogs.test.tsx::RenamePersonDialog > shows only the name on each suggestion chip` |

The `face-recognition.lightbox-rename-suggestions-localized` binding is removed
together with its requirement: suggestion entries now carry only a person's name,
so there is no suggestion copy left to translate.

## Migration Plan

Additive endpoint plus a webapp refactor; no data migration. Rollback is a revert
of the touched files; an older webapp bundle keeps using the full catalog and is
unaffected by the new route.
