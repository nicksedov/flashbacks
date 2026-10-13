# Proposal

## Why

On phone-sized viewports the lightbox lays its image area and its side panel out
side by side, so the panel's fixed width consumes the whole viewport and the
image is squeezed out of view. Every lightbox (gallery/EXIF/OCR/tags unified
lightbox, the enhancement compare-slider view, and the trash lightbox) shares
this two-pane assumption, so none of them is usable on a mobile screen today.

## What Changes

- On mobile viewports (below the `md` breakpoint) each lightbox SHALL show the
  image as the primary, full-area content; the side/details panel becomes a
  collapsible overlay that does not resize the image.
- The panel SHALL be collapsed by default and toggled open/closed through a
  single labelled control inside the lightbox that stays reachable in both
  states and keeps a visible text label (it is exempt from the icon-first rule
  because it is the only affordance for the hidden panel).
- The toggle SHALL expose its expanded state (`aria-expanded`) and the panel it
  controls (`aria-controls`) to assistive technology, and focus SHALL move into
  the panel on expand and back to the toggle on collapse.
- When expanded, the panel SHALL cover roughly 80% of the lightbox area, drawn
  over the image with a dimming scrim; the image keeps its current scale and
  position underneath, and tapping the exposed image area SHALL dismiss the
  panel.
- The expanded panel's position SHALL follow the screen orientation: anchored to
  the bottom edge in portrait orientation and to the right edge in landscape
  orientation, and the open state SHALL survive an orientation change.
- The expanded panel SHALL stay usable on a phone: it SHALL keep a focused text
  input above the on-screen keyboard (dynamic viewport height), keep its content
  and controls clear of device safe-area insets, and close on the platform back
  gesture before the lightbox itself closes.
- The toggle control and panel SHALL meet the existing 44×44 touch-target and
  accessible-name rules, and the collapse/expand transition SHALL respect
  `prefers-reduced-motion`.
- The lightbox chrome SHALL adopt the mobile responsive rules introduced in the
  previous mobile UI change: the mode tab bar wraps or scrolls without
  horizontal overflow, action controls (including the compare-slider actions)
  are icon-first, chrome controls meet the 44×44 touch target, long
  informational readouts collapse to an info icon, and the trash lightbox title
  truncates.
- A lightbox SHALL expose exactly one close control at every viewport width, drawn
  above its detail panel: the lightbox keeps its own close affordance and the
  shared dialog close is suppressed for it.
- The desktop/tablet layout SHALL stay intact: the mobile overlay and orientation
  rules are scoped below `md` so they cannot override the fixed-width detail
  column on a landscape viewport, and the mode tab bar stays on a single row at
  `md` and above.
- A collapsed informational readout's popup SHALL open inside the lightbox width
  instead of overflowing the lightbox's left edge.
- Desktop/tablet behavior at or above `md` (side-by-side panes) is unchanged.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `design-system`: The responsive layout rules gain a lightbox requirement —
  mobile lightboxes present a full-area image plus a collapsible overlay panel
  whose open position follows the screen orientation, covering ~80% of the area
  without resizing the image. The overlay dims the image and dismisses on a tap
  of the exposed area; the toggle is a labelled disclosure with
  `aria-expanded`/`aria-controls`, focus management, and an exemption from
  icon-first collapse; the panel keeps a focused input above the on-screen
  keyboard, respects safe-area insets, suppresses motion under
  `prefers-reduced-motion`, survives rotation, and closes on the back gesture
  before the lightbox. The lightbox chrome also becomes an explicit application
  of the existing mobile rules (tab-bar wrapping, icon-first actions including
  the compare-slider actions, 44×44 chrome touch targets, long informational
  labels collapsing to an info icon with the popup contained inside the lightbox,
  and the trash title bar truncating), that delta gains both the new requirements
  and the lightbox-specific scenarios, and the lightbox's mobile-only rules are
  explicitly scoped below `md` so the desktop two-pane layout — including its
  single close control — is preserved.

## Impact

- webapp components: [`UnifiedLightbox.tsx`](../../../webapp/src/components/gallery/UnifiedLightbox.tsx),
  [`LightboxDialog.tsx`](../../../webapp/src/components/gallery/lightbox/LightboxDialog.tsx),
  [`MobileDetailPanel.tsx`](../../../webapp/src/components/gallery/lightbox/MobileDetailPanel.tsx),
  [`lightboxChrome.tsx`](../../../webapp/src/components/gallery/lightbox/lightboxChrome.tsx),
  the shared [`dialog.tsx`](../../../webapp/src/components/ui/dialog.tsx) primitive
  (an additive close opt-out; no new dependency),
  [`TrashLightbox.tsx`](../../../webapp/src/components/trash/TrashLightbox.tsx),
  and the enhancement compare-slider view inside the unified lightbox.
- webapp i18n: new en/ru keys for the panel toggle control's accessible name and
  its visible label (`lightbox.panel.details`).
- No API, backend, OpenAPI, or dependency changes.

## Non-goals

- Changing the desktop/tablet (`md` and above) side-by-side lightbox layout.
- Redesigning panel content, tabs, or the enhancement comparison workflow.
- Introducing a global drawer/sheet primitive or migrating other dialogs to it.
- Changing the mobile navigation drawer or any non-lightbox screen.
