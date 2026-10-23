# Design

## Context

See [`proposal.md`](proposal.md) for motivation and
[`specs/llm-settings/spec.md`](specs/llm-settings/spec.md) for the required
behavior.

Current state that constrains the approach:

- The Analysis tab renders five model dropdowns in
  [`AdminAnalysisTab.tsx`](../../../webapp/src/components/tabs/AdminAnalysisTab.tsx):
  four through `renderInstrumentCard` (chat, VL, OCR, image edit) and one inline
  inside the Embeddings card. All five are the shared Radix-based
  [`Select`](../../../webapp/src/components/ui/select.tsx).
- Model options come from `modelCache[providerAlias]`, narrowed by
  [`filterModelsForInstrument()`](../../../webapp/src/lib/llm-model-filter.ts),
  which fails closed per instrument capability.
- The webapp has no combobox/command primitive and no `cmdk` or popover package;
  `@radix-ui/react-select` is the only dropdown primitive in the dependency set.
- Selecting a model immediately calls `saveInstrument(type, "model", id)`, which
  persists through `PUT /api/llm/settings`; there is no draft state.
- i18n is a strict `TranslationKey` union derived from
  [`translations.en.ts`](../../../webapp/src/i18n/translations.en.ts); `ru` must
  stay in sync and `translations.test.ts` enforces it.

## Goals / Non-Goals

**Goals:**

- One searchable dropdown primitive reused by all five instrument model dropdowns.
- Filtering is a pure, unit-testable function over `{ value, label }` options.
- No new runtime dependency; no backend, API-contract, or DI change.
- Compliance with the accessibility rules already required by
  [`design-system/spec.md`](../../../openspec/specs/design-system/spec.md).

**Non-Goals:**

- Replacing or modifying the shared `Select` primitive, or converting other
  dropdowns (provider selectors, tag-scan pickers, theme/language selects).
- Server-side model search or changes to the catalog endpoints.
- Multi-select, grouping, or virtualization of the option list.

## Decisions

### 1. A custom `SearchableSelect` combobox instead of a filtered Radix `Select`

Build a presentational `SearchableSelect` in
`webapp/src/components/ui/searchable-select.tsx` exposing:

```ts
type SearchableOption = { value: string; label: string }
props: {
  value: string
  onValueChange: (value: string) => void
  options: SearchableOption[]
  placeholder: string
  searchPlaceholder: string
  noResultsText: string
  disabled?: boolean
  triggerId?: string
}
```

It renders a trigger button (`role="combobox"`, `aria-expanded`, `aria-controls`,
`aria-haspopup="listbox"`, `aria-label` from the current selection or
placeholder), and, while open, a panel (`role="listbox"`) containing a real
`Input` search field plus `role="option"` rows.

*Why not keep Radix `Select` and inject an `Input` into `SelectContent`:* Radix
`Select` is a listbox whose content owns focus and keydown typeahead, and
`SelectValue` renders only while the selected `SelectItem` is mounted — filtering
the selected item out of the list blanks the trigger text. Working around both
means fighting the primitive.

*Why not `cmdk` + `@radix-ui/react-popover`:* two new dependencies for a single
control, against the project rule to justify every dependency, while the existing
`Input` and semantic tokens already cover the need.

### 2. Pure filter helper in the lib layer

`webapp/src/lib/option-filter.ts`:

```ts
export function filterOptionsByQuery<T extends { label: string }>(
  options: readonly T[],
  query: string,
): readonly T[]
```

Semantics: trim the query; an empty/whitespace query returns every option;
otherwise return options whose `label` contains the query as a case-insensitive
substring, preserving input order. Matching is label-only, exactly as the spec
states; the size suffix part of a model label is therefore searchable too.

Keeping this out of the component makes the substring and case rules testable
without rendering, mirroring the existing
[`llm-model-filter.test.ts`](../../../webapp/src/lib/llm-model-filter.test.ts) pattern.

### 3. Compatibility filter first, substring filter second

In `AdminAnalysisTab` each dropdown computes:

```ts
const compatible = filterModelsForInstrument(models, type)
const options = compatible.map(m => ({ value: m.id, label: formatModelLabel(m) }))
```

and passes `options` to `SearchableSelect`. The search can only narrow the
compatible set, satisfying "query cannot surface incompatible models" without
touching `filterModelsForInstrument`.

### 4. Query state owned by the primitive and reset on close

The typed query lives in `SearchableSelect` and is cleared whenever the panel
closes, so reopening always starts unfiltered. The saved `value` never changes
because of typing: only `onValueChange` (an option click or `Enter` on the
highlighted option) reaches `saveInstrument`, which preserves the existing
save-on-change behavior.

### 5. Portaled, fixed-position panel with viewport flip

The panel renders through `createPortal` into `document.body` and is positioned
from the trigger's `getBoundingClientRect()`, flipping above the trigger when
there is not enough room below, and recomputing on `resize`/`scroll`. An
absolutely positioned panel inside the card was rejected: cards sit inside a
scrollable admin layout, so the list would be clipped near the viewport bottom.

### 6. Keyboard and accessibility contract

- `ArrowDown`/`ArrowUp` move an active option, `Enter` selects it, `Escape`
  closes without changing the value, `Tab` closes.
- The search field receives focus when the panel opens, carries a localized
  label (`aria-label`), and its `keydown` events do not leak into option
  navigation.
- Every focusable element uses the shared focus-ring classes; option rows use
  `bg-accent text-accent-foreground` on active/hover, matching `SelectItem`.

### 7. Localization keys

Two new keys, added to `en` and `ru` together:

| Key | en | ru |
|---|---|---|
| `llm_providers.searchModels` | `Search models…` | `Поиск моделей…` |
| `llm_providers.noMatchingModels` | `No models match your search` | `Нет моделей, соответствующих поиску` |

Passed as props from `AdminAnalysisTab` (`searchPlaceholder`, `noResultsText`),
so the primitive stays i18n-agnostic like `Select`.

### 8. Evidence registration deferred to apply

The delta scenarios carry `focused-spec:id` markers now (planning phase). The
apply phase binds them in `.focused-spec/evidence.yaml` with `vitest::` selectors
(`planned:` only while the test does not yet exist), then runs
`./node_modules/.bin/focused-spec validate --scope current --strict` and
`run --scope current`.

## Risks / Trade-offs

- **Custom combobox drifts from Radix a11y quality** → explicit combobox/listbox
  ARIA, keyboard tests in the primitive's test file, and reuse of the existing
  focus-ring and accent tokens; the control is presentational only.
- **Filtering a long catalog on every keystroke** → the option array is already
  memoized per provider/instrument and the filter is a single pass; `useMemo`
  keeps it stable across unrelated re-renders.
- **Fixed-position panel desyncs while scrolling** → recompute the rect on
  `scroll`/`resize` and close on outside pointerdown; closing on scroll is the
  fallback if sync proves noisy.
- **Two dropdown styles in one card** (provider uses `Select`, model uses
  `SearchableSelect`) → both derive from the same tokens and trigger styling so
  the pair reads as one control family.
- **jsdom has no layout** → component tests assert roles, labels, and filtering,
  not pixel geometry; geometry stays untested by unit tests and is covered by
  manual verification.

## Migration Plan

Additive UI change on a feature branch; no data or API migration. Rollback is
reverting the commit — the five call sites keep their `onValueChange` handlers,
so switching them back to `Select` restores the previous behavior.

## Open Questions

None blocking: option ordering stays as the provider catalog returns it, and the
model label format (name plus optional size) is inherited unchanged from the
current `SelectItem` text.
