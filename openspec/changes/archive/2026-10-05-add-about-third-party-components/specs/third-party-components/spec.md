# Spec Delta

## Purpose

Discloses the third-party frameworks and libraries that ship inside Flashbacks,
so that self-hosted operators can see which components are used and under which
licenses the product is distributed.

## ADDED Requirements

### Requirement: About section discloses third-party components

The webapp SHALL provide a "Third-party components" view inside the About
section. It SHALL render a single combined list of the third-party frameworks
and libraries used by the product, and each entry SHALL display the component
name together with its license identifier.

#### Scenario: Viewing the third-party components list

- **WHEN** a user opens the About section and selects the "Third-party
  components" view
- **THEN** a single combined list of third-party components is displayed
- **AND** each entry shows the component name and its license identifier

### Requirement: Backend Go dependencies are attributed

The combined list SHALL include the third-party Go frameworks and libraries
used by the backend services (api-service, exif, ocr, shared, and
embeddings-builder). Modules owned by the Flashbacks project SHALL NOT be
listed.

#### Scenario: Backend frameworks appear in the list

- **WHEN** the third-party components list is rendered
- **THEN** it contains the third-party Go frameworks and libraries used by each
  backend service

#### Scenario: Own modules are excluded

- **WHEN** the third-party components list is rendered
- **THEN** Flashbacks-owned modules are not present

### Requirement: Each component links to its license

Every listed component SHALL link to an external resource describing its
license. Backend component links SHALL target the dependency's license text,
or its canonical license page when no single license file exists. Every link
SHALL open in a new browser tab.

#### Scenario: Opening a component license link

- **WHEN** a user activates a component's link
- **THEN** the component's license information opens in a new browser tab

#### Scenario: Backend link targets license information

- **WHEN** a backend component entry is inspected
- **THEN** its link points to that dependency's license text or canonical
  license page

### Requirement: Existing frontend attributions are preserved

The change SHALL NOT remove or alter the existing frontend component
attributions. Backend entries SHALL be added to the same combined list.

#### Scenario: Existing entries remain after the change

- **WHEN** the third-party components list is rendered after the change
- **THEN** all previously listed frontend components are still present

### Requirement: Component list is consistent across locales

Component names and license identifiers SHALL be identical in the English and
Russian locales; only the surrounding descriptive text is localized.

#### Scenario: Switching locale preserves the list

- **WHEN** the user switches the interface language between English and Russian
- **THEN** the same components with the same license identifiers are shown in
  both languages
