# Tasks

## 1. Filter foundation

- [x] 1.1 Add `filterOptionsByQuery()` to `webapp/src/lib/option-filter.ts` (trim query, empty query returns all options, otherwise case-insensitive substring match on `label`, input order preserved) and verify with `npx vitest run src/lib/option-filter.test.ts` from `webapp/` covering empty query, exact substring, different letter cases, and no-match.

## 2. Searchable select primitive

- [x] 2.1 Create `webapp/src/components/ui/searchable-select.tsx` with the `SearchableSelect` props from `design.md` (trigger `role="combobox"` with `aria-expanded`/`aria-controls`/`aria-haspopup`, portaled fixed panel with viewport flip, `Input` search field with `aria-label`, `role="option"` rows with `aria-selected`, `noResultsText` when the filtered list is empty, query cleared on close) and verify `npm run lint && npx tsc -b` passes in `webapp/`.
- [x] 2.2 Add `webapp/src/components/ui/searchable-select.test.tsx` verifying: opening renders the search field; typing filters options; a non-matching query shows `noResultsText`; `Escape` closes without calling `onValueChange`; choosing an option calls `onValueChange` with its value — run `npx vitest run src/components/ui/searchable-select.test.tsx`.

## 3. Localization

- [x] 3.1 Add `llm_providers.searchModels` (`Search models…` / `Поиск моделей…`) and `llm_providers.noMatchingModels` (`No models match your search` / `Нет моделей, соответствующих поиску`) to `webapp/src/i18n/translations.en.ts` and `webapp/src/i18n/translations.ru.ts` together, and verify `npx vitest run src/i18n/translations.test.ts` passes.

## 4. Analysis tab integration

- [x] 4.1 In `webapp/src/components/tabs/AdminAnalysisTab.tsx`, map `filterModelsForInstrument(...)` output to `{ value, label }` options once per card and render `SearchableSelect` instead of `Select` for the model control inside `renderInstrumentCard`, keeping `saveInstrument(type, "model", v)` as the change handler; verify `npm run lint && npx tsc -b` passes.
- [x] 4.2 Replace the inline embedding model `Select` in the Embeddings card with the same `SearchableSelect` wiring (compatibility filter still `filterModelsForInstrument(models, "embedding")`); verify `npm run lint && npx tsc -b` passes and the embedding option list is still capability-filtered.
- [x] 4.3 Extend `webapp/src/components/tabs/AdminAnalysisTab.test.tsx` with: a search field present in the chat card model dropdown, filtering by a typed substring, the localized no-match state, a filtered option selection calling `updateLlmSettings` with that model, and the search field in the Embeddings card dropdown; run `npx vitest run src/components/tabs/AdminAnalysisTab.test.tsx` with all pre-existing tests still green.

## 5. Documentation

- [x] 5.1 Record the new primitive in the drift audit table of `docs/design-system.md` (shared `SearchableSelect`, used by the five Analysis-tab model dropdowns, separate from and not a duplicate of `Select`) and confirm the row references existing token and ARIA rules rather than introducing new ones.

## 6. Evidence and full verification

- [x] 6.1 Bind every `llm-settings.model-search-*` scenario id from `openspec/changes/searchable-model-select/specs/llm-settings/spec.md` in `.focused-spec/evidence.yaml` under scope `current` with exact `vitest::<file>::<suite > test name` references, and verify `./node_modules/.bin/focused-spec validate --scope current --strict` reports no problems.
- [x] 6.2 Run `./node_modules/.bin/focused-spec run --scope current` and observe `PASS` for every new reference, then prove sensitivity: break the substring comparison in `filterOptionsByQuery`, observe the evidence `FAIL`, restore the comparison and observe `PASS` again.
- [x] 6.3 Run the full webapp check suite from `webapp/`: `npm run lint && npx tsc -b && npm test`, and confirm no theme, i18n, or snapshot regressions.

## Workflow follow-up

- Review the diff against `docs/agent/sdd-workflow.md` §7 before syncing.
- Sync the delta spec into `openspec/specs/llm-settings/spec.md` with `openspec-sync-specs`.
- Archive the change with `openspec-archive-change`.
