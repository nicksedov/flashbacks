# Design

## Context

See [`proposal.md`](proposal.md) — Why. The lightbox is built from a thin shared
shell plus per-view content:

- [`LightboxDialog.tsx`](../../../webapp/src/components/gallery/lightbox/LightboxDialog.tsx)
  renders the Radix dialog surface and close button.
- [`UnifiedLightbox.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.tsx)
  owns the two-pane split: a flexible image area (`flex-1`) beside a fixed-width
  panel (`w-full md:w-[400px] lg:w-[450px]`). The panel holds the AI/EXIF/OCR/tags
  tabs and is also where the enhancement compare-slider actions surface.
- [`TrashLightbox.tsx`](../../../webapp/src/components/trash/TrashLightbox.tsx)
  is a separate dialog with an image and header metadata, no side panel.

Below `md` the fixed-width panel currently fills the dialog, so `flex-1` leaves
the image no width. There is no orientation-aware behavior anywhere in the
webapp today. The design system already standardises on the `md` breakpoint and
on token-driven surfaces, and Tailwind 4 is available for both.

The lightbox chrome also predates the previous mobile UI pass: the mode tab bar
does not wrap, the dialog close button is a 32px (`h-8 w-8`) icon button rather
than a 44px touch target, and informational readouts are rendered inline at full
length. These are applications of rules that already exist in the main
`design-system` spec, not new rules.

The first implementation pass of this change leaked two mobile-only rules above
`md` — the orientation overlay utilities override the `md:` column, because
Tailwind emits the orientation media block after the `md` block, so a bare
`landscape:` utility wins on any landscape desktop viewport, and the tab bar
wraps at every width — and the lightbox ended up with two overlapping close
controls (its own plus the shared `DialogContent` one that the same pass
enlarged). D16–D19 record the corrections.

## Goals / Non-Goals

**Goals:**

- One shared, testable primitive that turns a lightbox detail panel into a
  collapsible overlay below `md`, so every lightbox (unified lightbox across all
  its tabs, the compare-slider view, and the trash lightbox) gets the same
  behavior without duplicating logic.
- Orientation-aware anchoring (bottom in portrait, right in landscape) with no
  image resize and no layout shift.

**Non-Goals:**

- The desktop/tablet layout at or above `md` (stays side-by-side).
- Panel content, tabs, or compare-slider workflows.
- A generic drawer/sheet system reused outside the lightbox.

## Decisions

**D1 — Extract a shared `MobileDetailPanel` primitive + `useIsMobileLightbox`
hook.** A single component under
`webapp/src/components/gallery/lightbox/MobileDetailPanel.tsx` renders the panel
slot: below `md` it is an absolutely positioned overlay that can be collapsed,
and at `md`+ it is the inline static column it is today. `UnifiedLightbox` and
`TrashLightbox` both consume it. Rationale: the behavior is cross-cutting and
must be unit-testable without mounting the heavy `UnifiedLightbox` (many data
hooks, API calls). Alternative considered: put the logic directly in
`UnifiedLightbox` and duplicate it in `TrashLightbox` — rejected because it
duplicates the orientation/overlay rules and makes evidence tests heavy.

**D2 — Drive the layout with Tailwind variants, keep React state only for
open/closed.** The breakpoint switch uses `md:`; orientation uses Tailwind's
`portrait:` / `landscape:` variants, which compile to media queries. The only
React state is the panel's open/closed boolean, defaulting to `false` and reset
when the image changes (following the existing reset-on-render pattern in
`UnifiedLightbox`). Rationale: no `matchMedia` listeners to leak or to desync on
rotation; CSS re-evaluates on resize/rotate for free. Alternative considered: a
`useMediaQuery` hook — rejected as more code and re-render churn for the same
result.

**D3 — The expanded panel is an overlay, never a flex sibling, below `md`.**
`absolute inset-x-0 bottom-0 h-[80dvh]` (portrait) / `absolute inset-y-0 right-0
w-[80%]` (landscape), reset at `md:` to `static` with the existing fixed width.
Because it is out of the flex flow while mobile, the image keeps its exact size
and position when the panel opens. Rationale: satisfies "image does not change
size" structurally rather than by measuring. The orientation overrides carry the
`max-md:` scope (D17) so they cannot win on a landscape viewport above the
breakpoint.

**D4 — A single labelled toggle button, always present below `md`.** Rendered
inside the panel primitive with `aria-label`/`title` from a new translation key
and a 44×44 CSS-pixel target; hidden at `md`+. Rationale: matches the existing
icon-first/touch-target rules in the design system spec.

**D5 — Motion is opt-in via `motion-safe:`.** The expand/collapse transition
uses `motion-safe:transition-*` so `prefers-reduced-motion` is respected with no
extra code.

**D6 — i18n keys added in both locales.** New keys for the toggle's accessible
name (show/hide) land in `translations.en.ts` and `translations.ru.ts` together;
no raw strings in `t()`.

**D7 — Reuse the existing mobile primitives for the lightbox chrome instead of
inventing new ones.** The tab bar reuses the shared `Tabs`/`TabsList` with the
same wrap/scroll treatment the settings tabs already use below `md` — scoped to
`max-md:` so the desktop tab bar keeps its single row (D18); icon-first
actions reuse the shared `Button` plus icon-first pattern from the header; the
long informational readout reuses the `InfoPopover` primitive added by the
previous mobile change, left-anchored so its popup stays inside the lightbox
(D19); and the chrome close control is resized to the shared 44×44 touch target
for the dialog that relies on the shared button (the trash lightbox) while the
unified lightbox keeps its own single control (D16). Rationale: the last commit
already established these
patterns, so the lightbox only needs to adopt them, keeping the design system's
single-primitive rule intact. Alternative considered: lightbox-local tweaks —
rejected because it would fork the mobile behavior the design system already
owns.

**D8 — The expanded panel renders a scrim that also dismisses on tap.** Below
`md` an absolutely positioned scrim (`absolute inset-0 bg-background/60`, below
the panel's z-index) covers the image while the panel is open; clicking it
collapses the panel. Rationale: a modal sheet needs an obvious way out besides
the toggle, and tapping the dimmed area to dismiss is the platform convention.
Alternative considered: a transparent hit area only over the exposed image
strip — rejected because a visible scrim both labels the layer as temporary and
gives a larger, unambiguous dismiss target.

**D9 — Dynamic viewport height keeps the panel above the on-screen keyboard.**
The portrait panel uses `h-[80dvh]` instead of `h-[80%]`, and the document
viewport declares `interactive-widget=resizes-content`, so a focused panel
input (the AI chat composer) stays above the on-screen keyboard without a
`visualViewport` listener. Rationale: `dvh` tracks the dynamic visual viewport
and re-evaluates for free; the landscape panel is width-bound and unaffected.
Alternative considered: a `visualViewport` resize listener — rejected for the
same re-render/leak reasons as D2.

**D10 — Safe-area insets protect the panel controls.** The overlay adds
`pb-[env(safe-area-inset-bottom)]` in the portrait base and
`max-md:landscape:pr-[env(safe-area-inset-right)]` in landscape so the toggle and
content clear the home indicator and notch. Rationale: the webapp is an installable PWA
(`manifest.json` + `sw.js`) and runs fullscreen on iOS/Android, where a raw
`bottom-0` sheet would sit under the OS chrome.

**D11 — The toggle is a labelled disclosure, exempt from icon-first collapse.**
The toggle carries `aria-expanded`, an `aria-controls` reference to the panel
id, and a visible text label (`lightbox.panel.details`). It is deliberately
excluded from the mobile icon-first rule (D7): it is the only affordance that
reveals a collapsed-by-default panel, so hiding its label would hide the
feature. Rationale: discoverability and assistive-technology state take
priority over uniformity for this one control.

**D12 — Focus follows the panel.** On expand, focus moves to the panel's first
focusable element; on collapse, focus returns to the toggle. Rationale: keeps
keyboard and screen-reader users oriented across the disclosure; the Radix
`Dialog` focus scope covers the dialog, not the nested panel toggle.

**D13 — Back/Escape collapses the panel before the dialog.** A keydown handler
for Escape plus a `popstate` guard collapse the panel when it is open; only when
the panel is already collapsed does the event reach the dialog. Rationale:
matches the platform back-stack convention (one layer at a time) without a
router.

**D14 — 80% is a fixed, scrollable default.** `h-[80%]` / `w-[80%]` stay as a
deterministic, testable default; the panel content already scrolls internally
(`overflow-y-auto`), so long EXIF lists and short OCR results both fit. A
content-aware min/max height was considered but rejected for determinism and
testability. Rationale: the spec says "approximately 80%", and internal scroll
removes the main downside of a fixed fraction.

**D15 — Trash header and compare actions adopt existing patterns.** The trash
title bar truncates (`truncate` + `min-w-0`) and reuses the 44×44 chrome target;
the compare-slider actions reuse the icon-first + 44×44 pattern from D7.
Rationale: the two remaining lightboxes then satisfy the same mobile rules
without new primitives.

**D16 — The unified lightbox owns its close control and suppresses the shared
one.** `DialogContent` gains an explicit `hideClose` opt-out that
[`LightboxDialog.tsx`](../../../webapp/src/components/gallery/lightbox/LightboxDialog.tsx)
sets, so the lightbox renders exactly one close control — its own circle, 44×44
below `md` and the pre-existing 32px (`h-8 w-8`) at `md` and above, raised above
the panel so the panel cannot cover it. Rationale: the shared `DialogContent`
close is the *only* close for the trash lightbox, whose scenario requires its
44×44 mobile target, so removing or un-styling it would regress that surface;
leaving both controls is what produced the duplicate. Alternative considered:
revert the shared close resize and drop the lightbox's own button — rejected
because it removes the trash lightbox's touch target and the lightbox's
localized, tested affordance.

**D17 — Mobile-only overlay rules are scoped `max-md:`.** Every overlay,
orientation and safe-area utility in `MobileDetailPanel` that only makes sense
below the breakpoint carries a `max-md:` prefix (the landscape sheet uses
`max-md:landscape:`) instead of a bare `landscape:`. Rationale: Tailwind emits the
orientation media block after the `md` block, so at equal specificity a bare
`landscape:w-[80%]`/`landscape:h-auto` beats `md:w-[400px]`/`md:h-full` on every
landscape desktop viewport and squeezes the image area to ~20 %. Scoping removes
the conflict structurally, whatever the emission order. Alternative considered:
adding more `md:` overrides — rejected because the later media block still wins.

**D18 — The tab-bar wrap is scoped below `md`.** The lightbox tab list carries
`max-md:flex-wrap max-md:gap-y-1` instead of an unconditional `flex-wrap`, so at
`md` and above it keeps the single `inline-flex` row it had before this change.
Rationale: the wrap exists for the narrow lightbox; leaving it on at desktop let
the four tabs wrap under the close control inside the 400px column.

**D19 — The lightbox info popup is anchored inside the lightbox.** The lightbox
readout passes a popup position to `InfoPopover` (`left-0 right-auto` plus a
viewport-derived width clamp) instead of the default `right-0`. Rationale: in the
chat header the readout sits at the panel's left edge, so a right-anchored popup
grows past the window's left edge and is clipped by the dialog's
`overflow-hidden`.

## Risks / Trade-offs

- [jsdom does not evaluate real media queries] → evidence asserts the presence
  and correctness of the responsive utility classes (the mechanism), matching
  how the existing mobile tests already verify breakpoint behavior, rather than
  computed layout.
- [80% is approximate and expressed as a utility class] → the panel uses a fixed
  `h-[80%]` / `w-[80%]` token so it is deterministic and testable; exact pixel
  rounding on device is acceptable per the requirement ("approximately").
- [Refactoring the panel wrapper could regress the desktop layout] → the `md:`
  branch reproduces the current `w-[400px]`/`lg:w-[450px]` classes verbatim, and
  the existing desktop behavior is covered by the full webapp test suite.
- [TrashLightbox has no panel today] → it gains the same collapsible treatment
  for its metadata so "every lightbox" is honoured; its content is unchanged.
- [jsdom cannot simulate a real keyboard or safe-area inset] → evidence asserts
  the mechanism (the `dvh` height class and `env(safe-area-inset-*)` classes)
  rather than a computed visual-viewport result, consistent with the existing
  breakpoint tests.
- [Platform back is not reachable in jsdom] → evidence dispatches an Escape
  keydown and asserts the panel collapses before the dialog; the `popstate`
  guard shares the same handler.
- [Scrim, tap-to-dismiss and focus moves widen the interactive surface] → each
  is covered by its own focused scenario so a regression fails exactly one item.
- [jsdom does not evaluate media queries, so "mobile-only rules stay below `md`"
  cannot be observed directly] → evidence asserts the mechanism (the `max-md:`
  prefixes and the absence of the unscoped orientation variant), and the built CSS
  is inspected once per change to confirm the panel width rule is no longer
  emitted after the `md` block.
- [Suppressing the shared close touches a shared primitive] → the opt-out is
  additive and defaults to today's behavior, so every other dialog (including the
  trash lightbox) keeps the shared close unchanged.
- [Re-anchoring the popup could push it off the right edge instead] → the popup
  keeps a viewport-derived width clamp and is left-anchored only for the lightbox
  readout, whose trigger sits at the panel's left edge.
- [Repairing existing class assertions leaves their registry selectors unchanged]
  → each repaired entry is mutation-checked again before implementation is
  declared complete.

## Migration Plan

Pure frontend change; no data or API migration. Rollback is a revert of the
webapp commit. Ship behind the normal webapp checks (`npm run lint && npx tsc -b
&& npm test && npm run build`).

## Evidence plan (intended runner and selectors)

During the propose phase only the native identity markers are authored; the
evidence registry and tests are created in the apply phase (focused-spec "respect
the current phase"). Runner: `vitest` (`@focused-spec/runner-vitest`) using the
repo-root Vitest config. The regression-fix rows at the end of the table were
added when the change was revised; `tabs-wrap`, `panel-overlay`,
`panel-landscape-right`, `rotation-persist` and `safe-area` keep their existing
selectors, but their assertions are repaired (the class tokens gain the `max-md:`
prefix), so each of them is mutation-checked again.

| Scenario ID | Intended selector |
|---|---|
| `design-system.mobile-lightbox-image-first` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > overlays the panel without adding a side column below md` |
| `design-system.mobile-lightbox-panel-collapsed` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > starts collapsed below md` |
| `design-system.mobile-lightbox-panel-toggle` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > expands the panel when the toggle is activated` |
| `design-system.mobile-lightbox-image-unchanged` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > keeps the panel absolutely positioned so the image keeps its size` |
| `design-system.mobile-lightbox-panel-overlay` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > covers about 80% of the area when expanded below md` |
| `design-system.mobile-lightbox-panel-portrait-bottom` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > anchors to the bottom in portrait` |
| `design-system.mobile-lightbox-panel-landscape-right` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > anchors to the right in landscape` |
| `design-system.mobile-lightbox-toggle-accessible` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > exposes an accessible name and a 44px touch target` |
| `design-system.mobile-lightbox-tabs-wrap` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > wraps the tab bar without horizontal overflow below md` |
| `design-system.mobile-lightbox-icon-first-actions` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > hides action text labels below md while keeping the icon and accessible name` |
| `design-system.mobile-lightbox-chrome-touch-targets` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > gives the close control a 44px touch target below md` |
| `design-system.mobile-lightbox-info-label-collapse` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > collapses a long informational readout to an info icon below md` |
| `design-system.mobile-lightbox-panel-collapse` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > collapses the panel when the toggle is activated again` |
| `design-system.mobile-lightbox-toggle-placement` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > keeps the toggle at a consistent reachable position in both states` |
| `design-system.mobile-lightbox-scrim` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > renders a scrim over the exposed image when expanded` |
| `design-system.mobile-lightbox-dismiss-outside` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > collapses when the exposed image area is tapped` |
| `design-system.mobile-lightbox-rotation-persist` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > keeps the panel expanded across orientation changes` |
| `design-system.mobile-lightbox-toggle-aria-expanded` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > exposes aria-expanded and aria-controls on the toggle` |
| `design-system.mobile-lightbox-toggle-visible-label` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > shows a visible label on the collapsed toggle` |
| `design-system.mobile-lightbox-keyboard-visible` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > uses a dynamic viewport height so the input stays above the keyboard` |
| `design-system.mobile-lightbox-safe-area` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > insets the panel from the safe-area edges` |
| `design-system.mobile-lightbox-reduced-motion` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > suppresses the transition under reduced motion` |
| `design-system.mobile-lightbox-focus-management` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > moves focus into the panel and back to the toggle` |
| `design-system.mobile-lightbox-back-dismiss` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > collapses on Escape before the dialog closes` |
| `design-system.mobile-lightbox-compare-actions` | `vitest::webapp/src/components/gallery/UnifiedLightbox.test.tsx::UnifiedLightbox > collapses compare-slider actions to icons below md` |
| `design-system.mobile-lightbox-trash-header` | `vitest::webapp/src/components/trash/TrashLightbox.test.tsx::TrashLightbox > truncates the title and keeps a 44px chrome target below md` |
| `design-system.mobile-lightbox-single-close-control` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > renders a single close control` |
| `design-system.mobile-lightbox-close-above-panel` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > draws the close control above the detail panel` |
| `design-system.mobile-lightbox-desktop-column-ratio` | `vitest::webapp/src/components/gallery/lightbox/MobileDetailPanel.test.tsx::MobileDetailPanel > keeps the fixed-width column at md and above` |
| `design-system.mobile-lightbox-desktop-tabs-single-row` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > keeps the tab bar on a single row at md and above` |
| `design-system.mobile-lightbox-info-popup-contained` | `vitest::webapp/src/components/gallery/lightbox/LightboxDialog.test.tsx::LightboxDialog > opens the info popup inside the lightbox on mobile` |
