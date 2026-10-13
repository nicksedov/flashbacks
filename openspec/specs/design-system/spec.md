# Design System Specification

## Purpose

Defines the project's design system — the design tokens, component primitive contracts, layout and positioning principles, interaction patterns, responsive behavior, theming rules, and accessibility requirements that every webapp screen MUST follow so the UI is consistent across features.

## Requirements

### Requirement: Design token model

All colors, spacing, corner radii, and elevations in the webapp SHALL be expressed through semantic CSS custom properties (design tokens) defined in [`globals.css`](../../../webapp/src/styles/globals.css). Components MUST reference semantic roles (`background`, `card`, `sidebar`, `header`, `primary`, `secondary`, `muted`, `accent`, `border`, `input`, `ring`, `destructive`, `popover`, and their `-foreground` counterparts) and MUST NOT hardcode raw color literals (hex, `hsl(...)`, or Tailwind palette utilities such as `bg-blue-500`) for surfaces, text, borders, or accents.

#### Scenario: Component uses semantic role token

- **WHEN** a component needs a surface, text, or accent color
- **THEN** it uses a semantic token (e.g., `bg-card`, `text-muted-foreground`, `bg-primary`) rather than a hardcoded palette value

#### Scenario: Raw color rejected in review

- **WHEN** a component file is reviewed for the design system
- **THEN** it contains no raw color literals outside the central theme stylesheet

### Requirement: Foreground token defined for every theme

Every theme (all light and dark accent variants) SHALL define a `--color-foreground` token for the default text color, in addition to the existing `--color-card-foreground`, so that utilities such as `text-foreground` and `hover:text-foreground` resolve correctly everywhere.

#### Scenario: text-foreground legible in dark themes

- **WHEN** the user selects any dark theme
- **THEN** elements styled with `text-foreground` are legible against the theme background

### Requirement: Single button primitive with fixed variants

All interactive buttons SHALL be rendered through the shared `Button`/`IconButton` primitive. The available variants SHALL be exactly: `default` (filled primary), `secondary`, `outline` (bordered), `ghost` (borderless low-emphasis), `destructive` (filled danger), and `link`. Icon-only buttons SHALL use the dedicated icon size rather than ad-hoc `h-8 w-8 p-0` overrides.

#### Scenario: New button uses the primitive

- **WHEN** a developer adds a button to any screen
- **THEN** they use the `Button` primitive with a declared variant, not a raw `<button>` with inline Tailwind classes

#### Scenario: Bordered versus borderless semantics

- **WHEN** a button is a low-emphasis secondary action in a normal context
- **THEN** it uses `ghost` or `secondary`; `outline` is reserved for actions that need a visible boundary (for example, on top of imagery or in dense toolbars)

### Requirement: Single select and single tabs primitive

The webapp SHALL expose exactly one `Select` primitive and one set of tab semantics. Duplicate near-identical implementations of the same control (for example, a `theme-select` copied from `select`) MUST be consolidated into one shared component.

#### Scenario: No duplicated control implementations

- **WHEN** a new dropdown, select, or tabbed section is needed
- **THEN** exactly one shared primitive exists and is reused rather than copied

#### Scenario: Underline tabs have no divider under the tab set

- **WHEN** a `Tabs` with the `underline` variant renders a set of tabs (for example, admin settings "Основные", "Инструменты анализа", "LLM провайдеры")
- **THEN** there is no horizontal divider line under the set of tabs; the active tab is indicated only by its own bottom underline

### Requirement: Dialog surface uses tokens

Dialog and popover surfaces SHALL derive their background and text colors from design tokens (`bg-popover`/`text-popover-foreground` or `bg-card`/`text-card-foreground`). Components MUST NOT branch on the active theme name to hardcode surfaces (for example, `bg-white/90` for light and `bg-black/90` for dark).

#### Scenario: Dialog follows the active theme

- **WHEN** the user switches between light and dark themes
- **THEN** dialog surfaces follow the theme automatically via tokens, with no per-theme hardcoded values

### Requirement: Unified empty state

Every collection (list, grid, or tree) with no items SHALL render a consistent empty state through a shared `EmptyState` primitive: a muted icon, a title, an optional hint, centered with consistent vertical padding. Screens MUST NOT invent per-screen empty layouts with differing icon sizes and padding.

#### Scenario: Empty collections look alike

- **WHEN** any collection is empty (gallery, folders, users, search results)
- **THEN** the empty state uses the same icon + title + hint structure and spacing

### Requirement: Unified loading state

Loading placeholders for collections and cards SHALL use the shared `Skeleton` primitive. A full-page or full-view initial load MAY use a single centered spinner, but inline list/card loading MUST use skeletons rather than ad-hoc spinners or text.

#### Scenario: Inline loading uses skeleton

- **WHEN** a list or card is loading inline
- **THEN** a skeleton placeholder is shown, not a bare spinner or plain text

### Requirement: Consistent primary action placement

The primary creation action ("add") for a collection SHALL appear in one consistent screen region across all equivalent screens: a primary "+ Add" button in the view's top-right action area (or the toolbar). Inline add-forms at the top of a list SHALL NOT be mixed with top-right dialog launchers for equivalent list screens.

#### Scenario: Adding an item is in the same place everywhere

- **WHEN** the user adds an item on two different list screens (for example, folders and users)
- **THEN** the add control appears in the same region (top-right action area) on both screens

### Requirement: Consistent settings manipulation pattern

Settings and configuration screens SHALL follow one pattern: controls grouped into cards or sections with a visible label above each control, and an explicit primary "Save" action. Nested sub-areas within a card SHALL use a borderless inset fill or a labeled section, never a nested border. The save action SHALL be disabled while nothing has changed and during an in-flight save. Inline per-field edit/confirm flows SHALL be used only for singular rename-style edits and SHALL reuse the same button variants.

#### Scenario: Save disabled when unchanged

- **WHEN** the user opens a settings screen and changes nothing
- **THEN** the primary save action is disabled

#### Scenario: Configuring settings looks the same across screens

- **WHEN** the user manipulates settings on different screens (preferences, LLM providers, analysis)
- **THEN** the grouping, labeling, and save affordance follow the same pattern

### Requirement: Nested areas use inset fill, not nested borders

Grouping and sectioning surfaces in the webapp SHALL be limited to one level of bordered container — the shared `Card` primitive. A nested sub-area inside a `Card` (a read-only status panel, a nested configuration group, or a sub-form) SHALL be visually distinguished with a borderless inset fill: a rounded container using a muted surface token (`bg-muted`, or `bg-accent`/`bg-secondary` for emphasis) and the shared radius token, with no `border` utility — or with a section label and vertical spacing. Adjacent sub-areas MAY be separated by the `Separator` primitive. This rule SHALL NOT apply to interactive form controls, badges, alert or error banners, or floating popover and dialog surfaces, whose borders are defined by their own primitive contracts.

#### Scenario: Read-only status panel uses inset fill

- **WHEN** a settings card contains a read-only status panel or sub-group (for example, sync status, trash status, or thumbnail cache status)
- **THEN** that sub-area renders as a borderless `bg-muted` inset fill with the shared radius and has no `border` class

#### Scenario: Nested configuration group uses inset fill or label

- **WHEN** a card contains a nested configuration group or sub-form (for example, an LLM provider form or embedding settings)
- **THEN** the group renders as a borderless inset fill or a labeled section, not a bordered box

#### Scenario: No border inside a border for grouping

- **WHEN** a component that groups settings is reviewed
- **THEN** no grouping surface nested inside a `Card` carries a `border` utility; only form controls, badges, alert banners, and floating surfaces remain bordered

### Requirement: Feedback via toast, not native dialogs

User-facing feedback for asynchronous results SHALL use the `sonner` toast system. Native `alert()` and `confirm()` SHALL NOT be used for success/error feedback. Destructive confirmations SHALL use the `Dialog` component.

#### Scenario: Failure surfaced as a toast

- **WHEN** an asynchronous operation fails
- **THEN** the failure is shown as an error toast, not a native `alert()`

#### Scenario: Confirmation uses a dialog

- **WHEN** an operation needs user confirmation (including permanent-delete warnings)
- **THEN** a `Dialog` is used, not `window.confirm()`

### Requirement: Destructive action confirmation

Destructive actions (delete, remove, purge) SHALL require explicit confirmation via a `Dialog` before execution, and the confirming button SHALL use the `destructive` variant while the cancel action uses a neutral variant (`outline` or `ghost`).

#### Scenario: Delete requires confirmation

- **WHEN** the user clicks a delete or remove action
- **THEN** a confirmation dialog is shown before any deletion occurs, with a destructive confirm button

### Requirement: Responsive layout and touch targets

The layout SHALL adapt to viewport width using defined breakpoints. On mobile the sidebar SHALL collapse into a drawer opened from the header menu button; toolbars SHALL wrap or condense; image grids SHALL reduce their column count. All interactive elements SHALL present a minimum touch target of 44×44 CSS pixels (24×24 visual + padding).

#### Scenario: Mobile navigation via drawer

- **WHEN** the viewport is below the `md` breakpoint
- **THEN** navigation is reachable through the header menu button and drawer rather than a fixed sidebar

#### Scenario: Grid reflows on small screens

- **WHEN** the viewport narrows to a phone width
- **THEN** image grids reduce columns and toolbars wrap without horizontal page overflow

<!-- focused-spec:id=design-system.toolbar-stacks-vertically -->
#### Scenario: Toolbar stacks vertically on mobile

- **WHEN** the viewport is below the `md` breakpoint and a toolbar or header row contains multiple horizontally-stretched controls (for example, a search input and a sort control)
- **THEN** those controls stack vertically or wrap without causing horizontal page overflow

<!-- focused-spec:id=design-system.icon-first-actions -->
#### Scenario: Icon-first actions on mobile

- **WHEN** the viewport is below the `md` breakpoint and a button or link has an informative icon together with a text label
- **THEN** the text label is hidden while the icon and an accessible name remain available

### Requirement: Long informational labels collapse to an icon on mobile

On mobile viewports, long informational labels (for example, an image count such as "N images in gallery") SHALL be replaced by an icon-only control. Activating the icon SHALL reveal the full information in a small popup that auto-hides three seconds after it appears.

<!-- focused-spec:id=design-system.info-label-collapse -->
#### Scenario: Label collapses to an icon on mobile

- **WHEN** the viewport is below the `md` breakpoint and a screen shows a long informational label such as an image count
- **THEN** the long label is not rendered and an icon-only control is shown in its place

<!-- focused-spec:id=design-system.info-popup-reveal -->
#### Scenario: Info popup reveals the full label

- **WHEN** the user activates the info icon
- **THEN** a small popup appears showing the full informational text

<!-- focused-spec:id=design-system.info-popup-autohide -->
#### Scenario: Info popup auto-hides after three seconds

- **WHEN** the info popup is shown
- **THEN** it disappears automatically after three seconds without further user action

### Requirement: Token-driven theme support

The webapp SHALL support light and dark themes per accent hue via a theme class on the `<html>` element. All components MUST derive their colors from tokens so a theme switch re-renders correctly without per-component theme logic. Components MUST NOT read the active theme name to choose colors (the theme picker preview swatches are the only exception).

#### Scenario: Theme switch re-renders without hardcoded branches

- **WHEN** the user switches between light and dark themes
- **THEN** every component re-renders with correct contrast using only token-derived styles

### Requirement: Keyboard focus and accessibility semantics

All interactive components SHALL expose a visible keyboard focus indicator (focus ring) and correct ARIA semantics. Dialogs, tabs, and selects SHALL provide the accessibility behavior from their Radix primitives. Icon-only buttons SHALL expose an accessible name via `aria-label` or `title`. Text SHALL NOT be the only channel for conveying state (for example, errors SHALL use an icon or border in addition to color).

#### Scenario: Icon-only button has an accessible name

- **WHEN** an icon-only button is rendered
- **THEN** it exposes an accessible name via `aria-label` or `title`

#### Scenario: Error state is not color-only

- **WHEN** a field or message is in an error state
- **THEN** the state is conveyed by more than color (icon and/or text), keeping it accessible

### Requirement: Typography, spacing, and page gutter

Text SHALL use the defined typography scale (`text-xs` through `text-2xl` with semantic weight helpers). Layout SHALL use the 4px-based spacing scale (Tailwind spacing utilities) and the shared radius tokens. Main content SHALL use one horizontal padding and max-width policy across tabs, with no per-tab special cases.

#### Scenario: Consistent page gutter across tabs

- **WHEN** any tab renders its main content
- **THEN** it uses the same horizontal padding and max-width policy as every other tab

### Requirement: Design system is the authority for agents

The design system specification SHALL be the authoritative reference for UI decisions. It MUST be present in the project's master specification (`openspec/specs/design-system/spec.md`) and referenced from the project [`AGENTS.md`](../../../AGENTS.md). Agents creating new features MUST consult it before implementing any UI.

#### Scenario: Agent consults the design system

- **WHEN** an agent implements a new feature that includes UI
- **THEN** it consults the design system spec and follows its component, layout, and positioning rules

### Requirement: Consistency inventory and reconciliation backlog

The design system SHALL maintain an inventory of known inconsistencies between the current UI and this spec, each mapped to its target resolution, so drift can be fixed in small, separately reviewable changes.

#### Scenario: Drift is tracked

- **WHEN** a screen deviates from this spec (for example, a raw `<button>` or a duplicate `Select`)
- **THEN** the deviation is recorded in the reconciliation backlog with the intended fix

### Requirement: Mobile lightbox is image-first

On a viewport below the `md` breakpoint a lightbox SHALL render the image as the primary content that fills the lightbox area, and any lightbox detail panel SHALL NOT occupy a permanent side column beside the image.

<!-- focused-spec:id=design-system.mobile-lightbox-image-first -->
#### Scenario: Image fills the lightbox on mobile

- **WHEN** a lightbox is opened on a viewport below the `md` breakpoint
- **THEN** the image is displayed within the lightbox area and no detail panel occupies a side column beside it

### Requirement: Mobile lightbox panel is collapsible

On a viewport below the `md` breakpoint a lightbox that has a detail panel SHALL show it collapsed by default and SHALL let the user open and close it through a single toggle control inside the lightbox, and the toggle SHALL keep a consistent position that stays reachable in both states.

<!-- focused-spec:id=design-system.mobile-lightbox-panel-collapsed -->
#### Scenario: Panel is collapsed when the lightbox opens

- **WHEN** a lightbox with a detail panel is opened on a viewport below the `md` breakpoint
- **THEN** the detail panel is collapsed and only the image and the toggle control are shown

<!-- focused-spec:id=design-system.mobile-lightbox-panel-toggle -->
#### Scenario: Toggle expands the collapsed panel

- **WHEN** the user activates the panel toggle while the panel is collapsed on a mobile viewport
- **THEN** the detail panel is expanded

<!-- focused-spec:id=design-system.mobile-lightbox-panel-collapse -->
#### Scenario: Toggle collapses the expanded panel

- **WHEN** the user activates the panel toggle while the panel is expanded on a mobile viewport
- **THEN** the detail panel is collapsed and the image is fully visible again

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-placement -->
#### Scenario: Toggle stays reachable in both panel states

- **WHEN** the lightbox panel is collapsed or expanded on a mobile viewport
- **THEN** the toggle control is rendered at a consistent position that remains reachable in both states

### Requirement: Expanding the panel does not resize the image

On a viewport below the `md` breakpoint, toggling the lightbox detail panel SHALL NOT change the rendered size or position of the image.

<!-- focused-spec:id=design-system.mobile-lightbox-image-unchanged -->
#### Scenario: Image keeps its size when the panel expands

- **WHEN** the detail panel is expanded on a mobile viewport
- **THEN** the image keeps the same rendered size and position as when the panel was collapsed

### Requirement: Expanded mobile lightbox panel overlays the image

When expanded on a viewport below the `md` breakpoint, the lightbox detail panel SHALL cover approximately 80% of the lightbox area and SHALL be drawn over the image, with a scrim separating the panel from the image area that remains visible beside it.

<!-- focused-spec:id=design-system.mobile-lightbox-panel-overlay -->
#### Scenario: Expanded panel covers about 80% over the image

- **WHEN** the detail panel is expanded on a mobile viewport
- **THEN** the panel covers approximately 80% of the lightbox area and is drawn over the image

<!-- focused-spec:id=design-system.mobile-lightbox-scrim -->
#### Scenario: Expanded panel renders a scrim over the image

- **WHEN** the detail panel is expanded on a mobile viewport
- **THEN** a scrim is rendered over the image area that is not covered by the panel

### Requirement: Expanded mobile lightbox panel can be dismissed without the toggle

When expanded on a viewport below the `md` breakpoint, the lightbox detail panel SHALL close when the user taps the image area that remains visible beside the panel.

<!-- focused-spec:id=design-system.mobile-lightbox-dismiss-outside -->
#### Scenario: Tapping the exposed image area closes the panel

- **WHEN** the detail panel is expanded on a mobile viewport and the user taps the visible image area beside the panel
- **THEN** the detail panel is collapsed

### Requirement: Expanded mobile lightbox panel follows screen orientation

On a viewport below the `md` breakpoint the expanded lightbox detail panel SHALL be anchored to the bottom edge of the lightbox in portrait orientation and to the right edge in landscape orientation, and its open state SHALL survive an orientation change.

<!-- focused-spec:id=design-system.mobile-lightbox-panel-portrait-bottom -->
#### Scenario: Expanded panel is anchored to the bottom in portrait

- **WHEN** the detail panel is expanded on a mobile viewport in portrait orientation
- **THEN** the panel is anchored to the bottom edge of the lightbox

<!-- focused-spec:id=design-system.mobile-lightbox-panel-landscape-right -->
#### Scenario: Expanded panel is anchored to the right in landscape

- **WHEN** the detail panel is expanded on a mobile viewport in landscape orientation
- **THEN** the panel is anchored to the right edge of the lightbox

<!-- focused-spec:id=design-system.mobile-lightbox-rotation-persist -->
#### Scenario: Panel open state survives an orientation change

- **WHEN** the detail panel is expanded on a mobile viewport and the orientation changes between portrait and landscape
- **THEN** the panel remains expanded and re-anchors to the new edge

### Requirement: Mobile lightbox panel toggle is accessible

The mobile lightbox panel toggle SHALL present a touch target of at least 44×44 CSS pixels, SHALL expose an accessible name in the active language (English or Russian), and SHALL expose its expanded state and the panel it controls to assistive technology. Because the toggle is the only affordance that reveals the collapsed panel, it SHALL keep a visible text label on mobile and is exempt from the icon-first collapse rule.

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-accessible -->
#### Scenario: Panel toggle exposes an accessible name and a large touch target

- **WHEN** the lightbox panel toggle is rendered on a mobile viewport
- **THEN** it exposes an accessible name in the active language and a touch target of at least 44×44 CSS pixels

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-aria-expanded -->
#### Scenario: Panel toggle exposes its expanded state

- **WHEN** the lightbox panel toggle is rendered on a mobile viewport
- **THEN** it exposes an expanded/collapsed state and references the panel it controls

<!-- focused-spec:id=design-system.mobile-lightbox-toggle-visible-label -->
#### Scenario: Collapsed toggle keeps a visible label

- **WHEN** the lightbox panel is collapsed on a mobile viewport and the toggle is rendered
- **THEN** the toggle shows a visible text label in the active language rather than an icon alone

### Requirement: Mobile lightbox panel stays usable with the on-screen keyboard

On a viewport below the `md` breakpoint, when the on-screen keyboard is shown while the lightbox detail panel is expanded, the panel's text input SHALL remain visible above the keyboard.

<!-- focused-spec:id=design-system.mobile-lightbox-keyboard-visible -->
#### Scenario: Expanded panel keeps its input above the keyboard

- **WHEN** the lightbox detail panel is expanded on a mobile viewport and the on-screen keyboard is shown while a panel text input has focus
- **THEN** the panel's text input remains visible above the keyboard

### Requirement: Mobile lightbox panel respects safe-area insets

The expanded lightbox detail panel on a viewport below the `md` breakpoint SHALL keep its content and controls clear of the device safe-area insets (notch and home indicator).

<!-- focused-spec:id=design-system.mobile-lightbox-safe-area -->
#### Scenario: Expanded panel avoids the device safe areas

- **WHEN** the detail panel is expanded on a mobile viewport that has safe-area insets
- **THEN** the panel content and toggle control are inset from the safe-area edges

### Requirement: Mobile lightbox panel motion respects reduced-motion

The lightbox detail panel's expand/collapse transition on a viewport below the `md` breakpoint SHALL be suppressed when the user prefers reduced motion.

<!-- focused-spec:id=design-system.mobile-lightbox-reduced-motion -->
#### Scenario: Panel transition is suppressed under reduced motion

- **WHEN** the detail panel expands or collapses on a mobile viewport and the user prefers reduced motion
- **THEN** the panel changes state without a transition animation

### Requirement: Mobile lightbox panel manages keyboard focus

When the lightbox detail panel is expanded or collapsed on a viewport below the `md` breakpoint, focus SHALL move into the panel on expand and return to the toggle control on collapse.

<!-- focused-spec:id=design-system.mobile-lightbox-focus-management -->
#### Scenario: Focus moves into the panel and back to the toggle

- **WHEN** the detail panel is expanded on a mobile viewport and then collapsed
- **THEN** focus moves into the panel on expand and returns to the toggle control on collapse

### Requirement: Mobile lightbox panel closes before the lightbox on back

On a viewport below the `md` breakpoint, when the detail panel is expanded and the user triggers the back gesture (Escape or the platform back action), the panel SHALL collapse before the lightbox itself closes.

<!-- focused-spec:id=design-system.mobile-lightbox-back-dismiss -->
#### Scenario: Back closes the expanded panel first

- **WHEN** the detail panel is expanded on a mobile viewport and the user triggers the back gesture
- **THEN** the panel collapses and the lightbox stays open

### Requirement: Lightbox controls follow the mobile responsive rules

On a viewport below the `md` breakpoint the lightbox chrome — its mode tab bar, action controls, and informational readouts — SHALL follow the design system's existing mobile responsive rules for layout, icon-first actions, touch targets, and informational labels, and any popup that a collapsed informational readout reveals SHALL open inside the lightbox width rather than overflowing the lightbox edge.

<!-- focused-spec:id=design-system.mobile-lightbox-tabs-wrap -->
#### Scenario: Lightbox tab bar wraps without horizontal overflow

- **WHEN** a lightbox below the `md` breakpoint shows a mode tab bar with multiple tabs
- **THEN** the tabs wrap or scroll within the lightbox width without causing horizontal overflow

<!-- focused-spec:id=design-system.mobile-lightbox-icon-first-actions -->
#### Scenario: Lightbox action labels collapse to icons

- **WHEN** a lightbox action control has an informative icon together with a text label below the `md` breakpoint
- **THEN** the text label is hidden while the icon and an accessible name remain available

<!-- focused-spec:id=design-system.mobile-lightbox-chrome-touch-targets -->
#### Scenario: Lightbox chrome controls meet the touch target

- **WHEN** a lightbox chrome control (for example the dialog close button) is rendered below the `md` breakpoint
- **THEN** it presents a touch target of at least 44×44 CSS pixels

<!-- focused-spec:id=design-system.mobile-lightbox-info-label-collapse -->
#### Scenario: Long lightbox informational labels collapse to an info icon

- **WHEN** the lightbox below the `md` breakpoint shows a long informational label such as a token or processing-time readout
- **THEN** the long label is replaced by an icon-only control whose popup reveals the full text

<!-- focused-spec:id=design-system.mobile-lightbox-info-popup-contained -->
#### Scenario: Info popup opens inside the lightbox

- **WHEN** the icon control of a collapsed long informational readout is activated below the `md` breakpoint
- **THEN** the popup opens inside the lightbox width instead of overflowing the lightbox edge

<!-- focused-spec:id=design-system.mobile-lightbox-compare-actions -->
#### Scenario: Compare-slider actions collapse to icons with touch targets

- **WHEN** the enhancement compare-slider view is shown below the `md` breakpoint with its action controls (replace, save copy, reject)
- **THEN** each action hides its text label, keeps its icon and accessible name, and presents a touch target of at least 44×44 CSS pixels

### Requirement: Lightbox exposes a single close control

A lightbox SHALL render exactly one close control at every viewport width, and that control SHALL be drawn above the lightbox detail panel whenever the panel is shown.

<!-- focused-spec:id=design-system.mobile-lightbox-single-close-control -->
#### Scenario: Lightbox renders a single close control

- **WHEN** a lightbox is rendered at any viewport width
- **THEN** exactly one close control is present in the lightbox

<!-- focused-spec:id=design-system.mobile-lightbox-close-above-panel -->
#### Scenario: Close control is drawn above the detail panel

- **WHEN** the lightbox is shown with its detail panel at or above the `md` breakpoint
- **THEN** the single close control is drawn above the panel and remains visible

### Requirement: Desktop lightbox layout is unaffected by the mobile rules

At the `md` breakpoint and above a lightbox SHALL keep its pre-existing side-by-side layout — the detail panel as a fixed-width column beside the image area and the mode tab bar on a single row — and the mobile overlay and orientation rules SHALL NOT apply at that width.

<!-- focused-spec:id=design-system.mobile-lightbox-desktop-column-ratio -->
#### Scenario: Detail panel keeps its fixed-width column at md and above

- **WHEN** a lightbox with a detail panel is shown at the `md` breakpoint or above
- **THEN** the detail panel keeps its fixed-width column and the mobile overlay and orientation sizing do not apply at that width

<!-- focused-spec:id=design-system.mobile-lightbox-desktop-tabs-single-row -->
#### Scenario: Tab bar stays on a single row at md and above

- **WHEN** the lightbox mode tab bar is shown at the `md` breakpoint or above
- **THEN** the tabs stay on a single row instead of wrapping

### Requirement: Trash lightbox header follows the mobile rules

On a viewport below the `md` breakpoint the trash lightbox title bar SHALL truncate long file names instead of overflowing, and its chrome controls SHALL present a touch target of at least 44×44 CSS pixels.

<!-- focused-spec:id=design-system.mobile-lightbox-trash-header -->
#### Scenario: Trash lightbox title truncates and its controls meet the touch target

- **WHEN** the trash lightbox is shown below the `md` breakpoint with a long file name
- **THEN** the title truncates rather than overflowing and its chrome controls present a touch target of at least 44×44 CSS pixels
