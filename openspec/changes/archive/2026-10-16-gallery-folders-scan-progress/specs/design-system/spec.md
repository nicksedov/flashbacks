# Spec Delta

## ADDED Requirements

### Requirement: Unified progress indicator

Background operations SHALL display progress through a single shared
progress-indicator primitive. The primitive SHALL render determinate progress
when a total is known and indeterminate progress when it is not, and SHALL show
a localized phase label together with an optional statistics line.

<!-- focused-spec:id=design-system.progress-indicator-shared -->
#### Scenario: Both scan progress displays use the shared primitive

- **WHEN** the gallery scan banner and the automatic image tagging progress are rendered
- **THEN** both render through the same shared progress-indicator component

<!-- focused-spec:id=design-system.progress-indicator-indeterminate -->
#### Scenario: Unknown total renders an indeterminate bar

- **WHEN** the progress indicator is rendered without a known total
- **THEN** its progress bar is indeterminate

<!-- focused-spec:id=design-system.progress-indicator-localized -->
#### Scenario: Phase label follows the interface language

- **WHEN** the progress indicator is rendered while the interface language is Russian
- **THEN** its phase label is shown in Russian rather than English

### Requirement: Progress indicator follows the active theme

The progress indicator SHALL derive all of its surface, text, and bar colors
from semantic design tokens and SHALL NOT use raw color literals. When it is
nested inside a `Card`, it SHALL render as a borderless inset fill with the
shared radius.

<!-- focused-spec:id=design-system.progress-indicator-token-based -->
#### Scenario: Indicator uses semantic tokens

- **WHEN** the progress indicator is rendered
- **THEN** its surface and bar use semantic token classes and no raw palette literals

<!-- focused-spec:id=design-system.progress-indicator-inset-fill -->
#### Scenario: Indicator is a borderless inset fill inside a card

- **WHEN** the progress indicator is rendered inside a `Card`
- **THEN** it renders as an inset fill with no border utility
