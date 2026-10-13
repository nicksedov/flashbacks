# Design

## Context

The webapp is a React 19 + Tailwind CSS 4 SPA. Responsive behavior already uses
Tailwind breakpoints (`md` = 768px) and a `Sidebar` that collapses to a drawer on
mobile. The offending screens all share one root cause: rows built with
`flex items-center justify-between` and fixed/wide children (a fixed `w-70`
search input, icon+label buttons, long count labels) do not reflow below `md`,
so they overflow horizontally. See
[`proposal.md`](proposal.md) for motivation.

Existing shared primitives relevant here:
[`Button`](../../../webapp/src/components/ui/button.tsx),
[`IconButton`](../../../webapp/src/components/ui/icon-button.tsx),
[`ViewHeader`](../../../webapp/src/components/ui/view-header.tsx),
[`Input`/`InputWithIcon`](../../../webapp/src/components/ui/input.tsx).
There is no existing popover/tooltip primitive, and no Radix popover/tooltip
dependency.

## Goals / Non-Goals

**Goals:**

- Eliminate horizontal overflow on phones for the shared header, the
  gallery search/sort area, the Smart Search bar, and the settings screens.
- Provide a small reusable, accessible, dependency-free "info icon + auto-hiding
  popup" primitive.
- Keep desktop behavior unchanged.

**Non-Goals:**

- Introducing a new UI dependency (e.g. Radix Popover/Tooltip) for this change.
- Reworking breakpoints, tokens, or the grid/drawer system.
- Changing any screen outside the areas listed in the proposal.

## Decisions

### 1. Reflow with Tailwind responsive utilities, not JS media queries

Each toolbar/header row gets responsive flex direction and width classes:
`flex-col` below `md`, `sm:flex-row` (or `md:flex-row`) and `sm:items-center`
above; fixed-width children get `w-full sm:w-auto` (or `sm:w-70`) and
`min-w-0`. This reuses the existing breakpoint system and keeps behavior
purely declarative.

- Alternative: a `useMediaQuery` hook — rejected; it adds runtime state for
  what Tailwind already expresses in CSS.

### 2. Long labels become an icon + auto-hiding popup

Add a dependency-free `InfoPopover` primitive (or extend `ViewHeader` with an
icon-only mobile variant). It renders an icon button (with `aria-label`/`title`
carrying the full text) and, when activated, an absolutely-positioned popover
card; a `setTimeout` (3s) plus cleanup on unmount/close hides it automatically.
The long text is still shown on desktop (`hidden md:block`) and inside the
popover on mobile.

- Alternative: Radix `Popover`/`Tooltip` — rejected as a new dependency for a
  tiny interaction; the auto-hide timer is trivial and fully testable with
  Vitest fake timers.

### 3. Icon-first actions on mobile

For buttons/links that pair an informative icon with a text label (logout,
sort, selection actions), hide the label below `md` with
`hidden md:inline`/`max-md:hidden` while keeping `aria-label`/`title` and the
minimum 44×44 target. The icon alone must be unambiguous (LogOut, ArrowDown/
ArrowUp, X, Move, Trash2).

### 4. Replace the raw calendar sort `<button>` with the `Button` primitive

[`GalleryCalendarView`](../../../webapp/src/components/gallery/GalleryCalendarView.tsx)
currently renders the sort toggle as a raw `<button>`, which violates the
design-system "Single button primitive" rule. This change converts it to the
shared `Button` variant and applies the icon-first treatment.

### 5. Evidence plan (focused-spec)

Identity is bound via `<!-- focused-spec:id=... -->` markers already placed on
each scenario in the delta spec. The intended evidence is Vitest component
tests (the installed `@focused-spec/runner-vitest`, framework ID `vitest`),
with one selector per scenario:

| Scenario ID | Planned selector (`vitest::...`) |
|---|---|
| `design-system.info-label-collapse` | `webapp/src/components/ui/InfoPopover.test.tsx` — icon shown, long text hidden below `md` |
| `design-system.info-popup-reveal` | `webapp/src/components/ui/InfoPopover.test.tsx` — activating the icon shows the full text |
| `design-system.info-popup-autohide` | `webapp/src/components/ui/InfoPopover.test.tsx` — popup unmounts after 3s (fake timers) |
| `design-system.toolbar-stacks-vertically` | `webapp/src/components/gallery/GalleryAllImagesView.test.tsx` — controls reflow without overflow below `md` |
| `design-system.icon-first-actions` | `webapp/src/components/layout/Header.test.tsx` — logout label hidden below `md`, icon + accessible name retained |

At apply time we create `.focused-spec/config.yaml` (markdown format adapter +
`vitest` runner + document layout for `openspec/specs/**/*.md`) and an evidence
shard (`.focused-spec/evidence.yaml`) mapping `current` scope IDs to the
selectors above, then turn each `planned:` reference into a real test and run
`validate` / `run` plus the mutation check.

## Risks / Trade-offs

- [Popup positioned absolutely can clip near viewport edges] → position it
  relative to the icon with a bounded `max-width` and allow it to flip/align
  to the start edge; keep content short.
- [Timer leak / stale popup after unmount] → clear the timeout in `useEffect`
  cleanup and on manual close.
- [Icon-only actions lose discoverability on mobile] → keep `title`/`aria-label`
  and rely on widely understood icons (LogOut, arrows); the full text remains on
  desktop and in the info popup for labels.
- [Marker/id markers must survive sync into the main spec] → the sync step
  carries scenario markers verbatim; verify with
  `openspec validate --specs` after merge.

## Migration Plan

Pure frontend CSS/component change with no data or API migration. Deploy is the
standard webapp build (`npm run build`). Rollback is reverting the feature
branch; no schema or contract changes are involved.
