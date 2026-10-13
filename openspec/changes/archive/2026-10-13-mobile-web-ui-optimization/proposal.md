# Proposal

## Why

On narrow (mobile) viewports several webapp screens overflow horizontally: the
shared header ("Выход"/"Logout" label), the search + sort area on the "All
images" and "Calendar" gallery views, the Smart Search bar, and most settings
screens. Long informational labels and icon+label buttons that fit on desktop
push content past the screen edge, making the app hard to use on phones.

## What Changes

- Collapse long informational labels (e.g. "N изображений в галерее" / "N images
  in gallery") into an icon on mobile; tapping the icon shows a small popup with
  the info that auto-hides after 3 seconds.
- Stack horizontally-stretched controls vertically on mobile so toolbars and
  form rows wrap instead of overflowing.
- Remove the text label from icon+label buttons/links on mobile when the icon is
  sufficiently informative, keeping an accessible name (`aria-label`/`title`).
- Apply these rules to the identified problem areas:
  - Shared [`Header`](../../../webapp/src/components/layout/Header.tsx) — logout
    and profile labels.
  - [`GalleryAllImagesView`](../../../webapp/src/components/gallery/GalleryAllImagesView.tsx)
    and [`GalleryCalendarView`](../../../webapp/src/components/gallery/GalleryCalendarView.tsx)
    — image-count label, search input, sort control.
  - [`SmartSearchTab`](../../../webapp/src/components/tabs/SmartSearchTab.tsx) —
    search bar and result-limit control.
  - Settings screens (user preferences and admin settings tabs) — card rows and
    tab labels.

No API or backend changes are required.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `design-system`: extend the responsive-layout rules with mobile overflow
  behavior (long labels collapse to icon + auto-hiding popup, controls stack
  vertically, icon-first actions), covering the shared header, gallery
  search/sort, smart search, and settings screens.

## Impact

- Affected code: `webapp/src/components/layout/Header.tsx`,
  `webapp/src/components/gallery/GalleryAllImagesView.tsx`,
  `webapp/src/components/gallery/GalleryCalendarView.tsx`,
  `webapp/src/components/tabs/SmartSearchTab.tsx`, settings tabs/components,
  and the shared UI primitives (`view-header`, `button`, `icon-button`).
- Affected i18n: new `en`/`ru` strings for the auto-hiding info popup (e.g.
  accessible labels/titles), kept in sync in both locales.
- No OpenAPI/MCP contract changes; no new dependencies.

## Non-goals

- Redesigning the desktop layout.
- Restyling the trash, deduplication, OCR, EXIF, or lightbox views beyond the
  identified overflow areas.
- Changing backend services, API contracts, or database behavior.
- Replacing the existing breakpoint/token system.
