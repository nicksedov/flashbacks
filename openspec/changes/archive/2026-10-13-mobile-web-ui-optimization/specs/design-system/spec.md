# Spec Delta

## ADDED Requirements

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

## MODIFIED Requirements

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
