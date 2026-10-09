## Purpose

Defines the project's design system — the design tokens, component primitive contracts, layout and positioning principles, interaction patterns, responsive behavior, theming rules, and accessibility requirements that every webapp screen MUST follow so the UI is consistent across features.

## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Consistent settings manipulation pattern

Settings and configuration screens SHALL follow one pattern: controls grouped into cards or sections with a visible label above each control, and an explicit primary "Save" action. Nested sub-areas within a card SHALL use a borderless inset fill or a labeled section, never a nested border. The save action SHALL be disabled while nothing has changed and during an in-flight save. Inline per-field edit/confirm flows SHALL be used only for singular rename-style edits and SHALL reuse the same button variants.

#### Scenario: Save disabled when unchanged

- **WHEN** the user opens a settings screen and changes nothing
- **THEN** the primary save action is disabled

#### Scenario: Configuring settings looks the same across screens

- **WHEN** the user manipulates settings on different screens (preferences, LLM providers, analysis)
- **THEN** the grouping, labeling, and save affordance follow the same pattern
