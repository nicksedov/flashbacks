# Tasks

## 0. Test infrastructure (justified new devDependencies)

- [x] 0.1 Add `jsdom` and `@testing-library/react` as devDependencies in `webapp/package.json` so React components can be rendered and interacted with in tests (justification: the SDD evidence gate requires executable component evidence; the webapp previously had only node-env logic tests). Tests use plain assertions + a `hasClass` helper (no jest-dom matchers) so they run under any Vitest instance.
- [x] 0.2 Enable the `jsdom` environment and register `webapp/src/test/setup.ts` (RTL cleanup) in `webapp/vitest.config.ts`. Verify existing logic tests still pass with `cd webapp && npm test`.
- [x] 0.3 Add `vitest` + `jsdom` at the repo root and a root `vitest.config.ts` (dedupe `vitest`, `@` alias → `webapp/src`, jsdom, webapp tests) so the focused-spec `vitest` runner, which resolves Vitest from the repo root, can execute the webapp component tests.

## 1. Shared info icon primitive

- [x] 1.1 Create `webapp/src/components/ui/info-popover.tsx` (a dependency-free `InfoPopover` primitive: icon button with `aria-label`/`title` and a 3-second auto-hiding popover showing the full text). Verify with `cd webapp && npm run lint && npx tsc -b`.
- [x] 1.2 Reuse existing translation keys for the popup text/accessible name (no new en/ru keys); if a genuinely new label is required, add it to `translations.en.ts` and `translations.ru.ts` together. Verify `cd webapp && npm test` (includes `translations.test.ts`).
- [x] 1.3 Add `webapp/src/components/ui/info-popover.test.tsx` covering: icon renders, activation reveals the full text, popup auto-hides after 3s (Vitest fake timers). Verify `cd webapp && npm test`.

## 2. Shared Header

- [x] 2.1 In `webapp/src/components/layout/Header.tsx`, hide the logout/profile text labels below `md` while keeping the icons and `aria-label`/`title`, and wrap the selection action row so it does not overflow. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 2.2 Add `webapp/src/components/layout/Header.test.tsx` asserting the logout label is hidden below `md` while the icon and accessible name remain. Verify `cd webapp && npm test`.

## 3. Gallery All Images and Calendar search/sort

- [x] 3.1 In `webapp/src/components/gallery/GalleryAllImagesView.tsx`, make the search+sort row stack vertically below `md`, give the search input a flexible width (`w-full sm:w-70` with `min-w-0`), and render the sort control icon-first. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 3.2 Replace the long image-count label in `GalleryAllImagesView` with the `InfoPopover` icon-only control below `md` (full text retained on desktop). Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 3.3 In `webapp/src/components/gallery/GalleryCalendarView.tsx`, convert the raw sort `<button>` to the shared `Button` primitive, apply the icon-first treatment, and stack the count + sort header vertically below `md`. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 3.4 Add gallery component tests asserting the search/sort controls reflow without horizontal overflow below `md` and the count collapses to the info icon. Verify `cd webapp && npm test`.

## 4. Smart Search

- [x] 4.1 In `webapp/src/components/tabs/SmartSearchTab.tsx`, stack the search input and the result-limit control vertically below `md` and make the search input full-width. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 4.2 Add a Smart Search test asserting the search bar and limit control reflow on mobile. Verify `cd webapp && npm test`.

## 5. Settings screens

- [x] 5.1 In `webapp/src/components/tabs/AdminSettingsTab.tsx`, make the underline tab list wrap (or scroll) below `md` so tab labels do not overflow. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 5.2 Ensure settings card rows (`AdminGeneralTab`, `AdminAnalysisTab`, `AdminLlmProvidersTab`, `SettingsTab`) use `min-w-0`/`w-full sm:w-auto` and stack vertically below `md` where a fixed-width input/control sits beside a label. Verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 5.3 Add/extend a settings test asserting a representative form row reflows on mobile without overflow. Verify `cd webapp && npm test`.

## 6. Focused-spec evidence integration

- [x] 6.1 Create `.focused-spec/config.yaml` registering the `@focused-spec/format-markdown` adapter, the `@focused-spec/runner-vitest` runner (`vitest`), and document layouts covering `openspec/specs/**/*.md` and active change deltas `openspec/changes/*/specs/**/*.md` (scope `current`). Verify `node_modules/.bin/focused-spec validate --scope current --syntax-only` reports "focused specifications are structurally valid".
- [x] 6.2 Create `.focused-spec/evidence.yaml` binding each scenario ID (`design-system.info-label-collapse`, `design-system.info-popup-reveal`, `design-system.info-popup-autohide`, `design-system.toolbar-stacks-vertically`, `design-system.icon-first-actions`) to its concrete Vitest selector. Verify `node_modules/.bin/focused-spec validate --scope current --strict` reports `5 registered items, 0 planned evidence, 5 unique targets`.
- [x] 6.3 Turn every `planned:` reference into the concrete Vitest selector from the tests written in groups 1–5. Verify `node_modules/.bin/focused-spec run --scope current` reports `5 PASS, 0 FAIL`.
- [x] 6.4 Mutation check each evidence entry: reverted the behavior under test (auto-hide timer removed, popup text removed, `md:hidden` count wrapper dropped, mobile-stack class dropped, logout label class dropped), confirmed `focused-spec run --scope current` reported `0 PASS, 5 FAIL` on the behavioral assertions, restored the behaviors, and confirmed `5 PASS, 0 FAIL`.

## 7. Full verification

- [x] 7.1 Run the complete webapp check suite and confirm green: `cd webapp && npm run lint && npx tsc -b && npm test && npm run build` (36 tests pass; build succeeds).

## Workflow follow-up

- Sync the delta spec into `openspec/specs/design-system/spec.md` (openspec-sync-specs) and run `openspec validate --specs`.
- Archive the change into `openspec/changes/archive/`.
