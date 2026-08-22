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

Settings and configuration screens SHALL follow one pattern: controls grouped into cards or sections with a visible label above each control, and an explicit primary "Save" action. The save action SHALL be disabled while nothing has changed and during an in-flight save. Inline per-field edit/confirm flows SHALL be used only for singular rename-style edits and SHALL reuse the same button variants.

#### Scenario: Save disabled when unchanged

- **WHEN** the user opens a settings screen and changes nothing
- **THEN** the primary save action is disabled

#### Scenario: Configuring settings looks the same across screens

- **WHEN** the user manipulates settings on different screens (preferences, LLM providers, analysis)
- **THEN** the grouping, labeling, and save affordance follow the same pattern

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
