# Spec Delta

## ADDED Requirements

### Requirement: One shared schedule editor renders every schedule section

Settings sections that configure a recurring job schedule SHALL render their
weekday and time controls through a single shared schedule-editor component.

<!-- focused-spec:id=design-system.schedule-editor-shared -->
#### Scenario: Both schedule sections use the shared editor

- **WHEN** the sync schedule section and the tagging schedule section are rendered
- **THEN** each renders its weekday controls inside the same shared schedule-editor component

### Requirement: The schedule editor orders weekdays Monday through Sunday

The schedule editor SHALL present its weekday controls in the order Monday,
Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday regardless of the
weekday numbering used by the API.

<!-- focused-spec:id=design-system.schedule-editor-weekday-order -->
#### Scenario: Weekday controls start with Monday

- **WHEN** the schedule editor renders its weekday controls
- **THEN** the first control is Monday and the last control is Sunday

### Requirement: Schedule editor weekday labels follow the interface language

The schedule editor SHALL take its weekday labels from the interface
translations, so the labels match the active language.

<!-- focused-spec:id=design-system.schedule-editor-weekday-labels-localized -->
#### Scenario: Weekday labels are Russian under the Russian interface

- **WHEN** the schedule editor renders while the interface language is Russian
- **THEN** its weekday labels are shown in Russian rather than English
