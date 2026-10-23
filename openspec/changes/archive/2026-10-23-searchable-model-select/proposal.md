# Proposal

## Why

The Analysis tab of Settings assigns a model to every LLM instrument (chat, VL,
OCR, image edit, embedding). Each instrument renders a plain dropdown that lists
every compatible model a provider catalog returned, and provider catalogs
routinely expose dozens or hundreds of models. Finding one model means scrolling
a long, unsorted list with no way to narrow it, which makes the most common
administrative action — picking a model — slow and error-prone.

## What Changes

- Model dropdowns in the Analysis tab gain an integrated search field: once the
  dropdown is open, typing a substring filters the visible options to models
  whose label contains that substring (case-insensitive).
- The filter is applied on top of the existing instrument-compatibility filter, so
  it can only narrow the already-eligible model set (it never reintroduces a model
  that the instrument type excludes).
- A reusable searchable-select primitive is introduced in the webapp UI layer and
  used by all five instrument model dropdowns (chat, VL, OCR, image edit,
  embedding), replacing the plain `Select` in those five places.
- An explicit empty state is shown when no option matches the typed query, in both
  locales (`en`, `ru`).
- Selecting an option, the currently saved value, and the existing save-on-change
  behaviour are unchanged.

## Non-goals

- Provider dropdowns in the same cards are not changed; only model dropdowns are
  in scope.
- No server-side search, pagination, or catalog API change: filtering happens
  entirely on the already-loaded model list.
- No change to which models are compatible with an instrument
  (`filterModelsForInstrument` stays the authority).
- No change to manual model entry, the catalog refresh action, or the stale
  catalog notice.
- Other dropdowns in the app (tag-scan hour/minute pickers, theme select, language
  select) are not converted.
- No new third-party dependency: the primitive is built on the already-vendored
  Radix primitives and existing Tailwind/semantic tokens.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `llm-settings`: the Analysis-tab instrument model dropdowns become searchable —
  a typed substring filters the listed models, and a no-match state is shown.

## Impact

- Webapp UI: new shared primitive (searchable select) plus its unit test; the five
  instrument model dropdowns in
  [`AdminAnalysisTab.tsx`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx)
  consume it.
- i18n: new user-facing strings for the search placeholder and the no-match state
  in [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts) and
  [`translations.ru.ts`](../../../webapp/src/i18n/translations.ru.ts), kept in sync.
- Tests: primitive filter unit tests plus an Admin Analysis tab interaction test;
  existing [`AdminAnalysisTab.test.tsx`](../../../webapp/src/components/tabs/AdminAnalysisTab.test.tsx)
  must keep passing.
- No API contract change: `docs/api-contracts/core.yaml` and the generated
  `webapp/src/types/api.ts` are untouched; no backend change.
- Design system: the new primitive must satisfy the existing accessibility and
  single-primitive rules in
  [`openspec/specs/design-system/spec.md`](../../../openspec/specs/design-system/spec.md)
  (visible focus, correct ARIA, no duplicated `Select` implementation).
