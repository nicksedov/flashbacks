# Tasks

## 1. Shared mobile lightbox panel primitive

- [x] 1.1 Add the panel toggle translation keys (`lightbox.panel.show`, `lightbox.panel.hide`, `lightbox.panel.details`) to `webapp/src/i18n/translations.en.ts` and `webapp/src/i18n/translations.ru.ts` together; verify `cd webapp && npm test` (includes `translations.test.ts`).
- [x] 1.2 Create `webapp/src/components/gallery/lightbox/MobileDetailPanel.tsx`: below `md` an absolutely positioned collapsible overlay — `absolute inset-x-0 bottom-0 h-[80dvh] pb-[env(safe-area-inset-bottom)]` in `portrait:`, `absolute inset-y-0 right-0 w-[80%] pr-[env(safe-area-inset-right)]` in `landscape:`, hidden when closed, with `motion-safe:` transition — resetting at `md:` to the current static fixed-width column (`md:static md:w-[400px] lg:w-[450px]`); render a dimming scrim (`absolute inset-0 bg-background/60`) that collapses the panel on click; a toggle button fixed at a consistent reachable position in both states, showing the visible `lightbox.panel.details` label plus `aria-expanded`/`aria-controls` and a 44×44 target; move focus into the panel on expand and back to the toggle on collapse; collapse on Escape/`popstate` before the dialog closes; verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 1.3 Add `webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx` covering all twenty panel scenarios (image-first/overlay, collapsed default, toggle expand, toggle collapse, toggle placement, image unchanged, ~80% overlay, scrim, tap-outside dismiss, portrait bottom, landscape right, rotation persist, accessible name + touch target, aria-expanded/controls, visible collapsed label, keyboard/dvh, safe-area, reduced motion, focus management, back-before-dialog); verify `cd webapp && npm test`.
- [x] 1.4 Add `interactive-widget=resizes-content` to the webapp viewport meta in `webapp/index.html` so the dynamic-viewport panel keeps a focused input above the on-screen keyboard; verify `cd webapp && npm run lint && npx tsc -b && npm run build`.
- [x] 1.5 Update `docs/design-system.md` drift notes to record the new mobile lightbox patterns (overlay + scrim, labelled disclosure toggle, keyboard/safe-area/reduced-motion behavior); verify the file lists them under the responsive section.

## 2. Unified lightbox integration (gallery / EXIF / OCR / tags / compare slider)

- [x] 2.1 In `webapp/src/components/gallery/UnifiedLightbox.tsx`, render the AI/EXIF/OCR/tags panel through `MobileDetailPanel` and add the `open`/`onOpenChange` state that resets to collapsed when `imagePath`/`initialMode` change; keep the image area unchanged below and above `md`; verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 2.2 Confirm the enhancement compare-slider branch keeps its own full-area layout and gains no side column below `md`; verify by rendering the compare-slider state in a component test in `webapp/src/components/gallery/UnifiedLightbox.test.tsx` (mock the data hooks) and running `cd webapp && npm test`.
- [x] 2.3 Verify the collapsed-by-default and toggle behavior for the real unified lightbox: extend `webapp/src/components/gallery/UnifiedLightbox.test.tsx` to assert the toggle opens and closes the panel; verify `cd webapp && npm test`.
- [x] 2.4 In `webapp/src/components/gallery/lightbox/LightboxDialog.tsx`, make the mode tab bar wrap or scroll within the lightbox width below `md` (no horizontal overflow), apply the icon-first treatment to lightbox action controls (hide the text label, keep the icon and `aria-label`/`title`), and give the close control a 44×44 touch target; verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 2.5 Collapse a long lightbox informational readout (for example the chat token or OCR processing-time text) to the shared `InfoPopover` icon-only control below `md`, reusing existing translation keys and adding en/ru entries together if a new key is needed; verify `cd webapp && npm run lint && npx tsc -b` and `cd webapp && npm test` (translations parity).
- [x] 2.6 Add `webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx` asserting, below `md`: the tab bar wraps without horizontal overflow, action text labels are hidden while the icon and accessible name remain, the close control presents a 44px touch target, and a long informational readout collapses to the info icon; verify `cd webapp && npm test`.
- [x] 2.7 In the enhancement compare-slider view (inside `UnifiedLightbox.tsx` / `ImageCompareSlider.tsx`), apply the icon-first treatment and a 44×44 touch target to the replace/save-copy/reject actions below `md`; extend `webapp/src/components/gallery/UnifiedLightbox.test.tsx` to assert the hidden text label with icon + accessible name and the touch target; verify `cd webapp && npm run lint && npx tsc -b && npm test`.

## 3. Trash lightbox integration

- [x] 3.1 In `webapp/src/components/trash/TrashLightbox.tsx`, render the image as full-area content below `md` and move the location/date metadata into `MobileDetailPanel` (content unchanged); verify `cd webapp && npm run lint && npx tsc -b`.
- [x] 3.2 Add `webapp/src/components/trash/TrashLightbox.test.tsx` asserting the image is visible with the metadata panel collapsed on mobile and expands via the toggle; verify `cd webapp && npm test`.
- [x] 3.3 Align the trash lightbox chrome with the mobile rules below `md` (title truncation with `min-w-0`/`truncate`, icon-first actions, and a 44×44 close control) and extend `webapp/src/components/trash/TrashLightbox.test.tsx` to assert the truncated title and the touch target; verify `cd webapp && npm run lint && npx tsc -b && npm test`.

## 4. Focused-spec evidence integration

- [x] 4.1 Add registry entries to `.focused-spec/evidence.yaml` (scope `current`) binding the twenty-six design-system scenario IDs to the exact `vitest::` selectors recorded in `design.md`; verify `node_modules/.bin/focused-spec validate --scope current --strict` reports 26 registered items and 0 planned evidence.
- [x] 4.2 Run `node_modules/.bin/focused-spec run --scope current` and confirm every new scenario passes alongside the existing five.
- [x] 4.3 Mutation check each new evidence entry: revert one mobile behavior at a time (overlay positioning, default-collapsed state, toggle expand/collapse, toggle placement, 80% size, scrim, tap-outside dismiss, portrait/landscape anchoring, rotation persist, accessible name/touch target, aria-expanded/controls, visible collapsed label, keyboard/dvh, safe-area insets, reduced motion, focus management, back-before-dialog, tab-bar wrap, icon-first labels, chrome touch target, info-label collapse, compare actions, trash header), confirm the matching scenario fails on its behavioral assertion, then restore and confirm it passes again.

## 5. Full verification

- [x] 5.1 Run the complete webapp check suite and confirm green: `cd webapp && npm run lint && npx tsc -b && npm test && npm run build`.
- [x] 5.2 Run `openspec validate mobile-lightbox-adaptation --strict` and confirm the change validates.

## 6. Post-apply regression fixes (lightbox)

- [x] 6.1 In `webapp/src/components/gallery/lightbox/MobileDetailPanel.tsx`, scope the landscape overlay utilities below `md` (`max-md:landscape:*`) so they cannot override `md:w-[400px]`/`md:h-full` on a landscape desktop viewport; verify `cd webapp && npm run lint && npx tsc -b` and confirm the scoping in the built CSS. Result: lint + `tsc -b` clean; the built stylesheet now emits every panel orientation rule inside `@media not all and (width>=48rem){@media (orientation:landscape)}` with no bare `.landscape\:` selector left, while `md\:w-\[400px\]` is still emitted.
- [x] 6.2 Add an additive `hideClose` opt-out to `DialogContent` in `webapp/src/components/ui/dialog.tsx` and set it from `webapp/src/components/gallery/lightbox/LightboxDialog.tsx`, which keeps its own close control (`z-30`, stacked above the panel's `z-20` and its `z-10` scrim) as the lightbox's only one; verify `cd webapp && npm run lint && npx tsc -b` — clean.
- [x] 6.3 In `webapp/src/components/gallery/lightbox/lightboxChrome.tsx`, scope the tab-bar wrap below `md` (`max-md:flex-wrap max-md:gap-y-1`) and pass a left-anchored, width-clamped popup position from `LightboxInfoReadout` to `InfoPopover`; verify `cd webapp && npm run lint && npx tsc -b` — clean.
- [x] 6.4 Add the regression tests — `webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx` (single close control, close above the panel, single-row tab bar at `md`+, popup anchored inside the lightbox; 8 tests) and `webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx` (fixed-width column at `md`+; 21 tests) — and repair the `max-md:` class assertions in the existing tests; verify `cd webapp && npm test` — 71/71 pass across 13 files.
- [x] 6.5 Register the five new scenario IDs in `.focused-spec/evidence.yaml` with the exact selectors recorded in `design.md`; verify `node_modules/.bin/focused-spec validate --scope current --strict` — `focused specifications are valid: 36 registered items, 0 planned evidence, 36 unique targets`.
- [x] 6.6 Run `node_modules/.bin/focused-spec run --scope current` and mutation-check every new or repaired evidence entry. Result: baseline `36 PASS, 0 FAIL, 0 SKIP, 0 ERROR`. Mutation pairs (mutate → fail → restore → pass): dropping `hideClose` fails `mobile-lightbox-single-close-control`; lowering the close to `z-10` fails `mobile-lightbox-close-above-panel`; unscoping `max-md:landscape:` fails `mobile-lightbox-desktop-column-ratio`, `mobile-lightbox-panel-overlay`, `mobile-lightbox-panel-landscape-right`, `mobile-lightbox-rotation-persist` and `mobile-lightbox-safe-area`; unscoping the tab-bar wrap fails `mobile-lightbox-desktop-tabs-single-row` and `mobile-lightbox-tabs-wrap`; removing the popup anchoring fails `mobile-lightbox-info-popup-contained`. Each mutation was reverted and every entry passes again.
- [x] 6.7 Update `docs/design-system.md` drift notes with the single close control, the `max-md:` scoping rule for the lightbox's orientation/overlay utilities, and the contained info popup.
- [x] 6.8 Run the complete webapp check suite: `cd webapp && npm run lint && npx tsc -b && npm test && npm run build` — all green (71 tests, build ok).
- [x] 6.9 Run `openspec validate mobile-lightbox-adaptation --strict`.

## Workflow follow-up

- Sync the delta spec into `openspec/specs/design-system/spec.md` (openspec-sync-specs) and run `openspec validate --specs`.
- Archive the change into `openspec/changes/archive/`.
