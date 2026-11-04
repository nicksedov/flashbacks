# Spec Delta

## ADDED Requirements

### Requirement: Instrument dropdowns are populated automatically

The Analysis tab SHALL populate every instrument model dropdown from the provider's stored
catalog when the tab opens. Listing the available models SHALL NOT require a separate manual
load action.

<!-- focused-spec:id=llm-settings.dropdown-populated-automatically -->
#### Scenario: Dropdown is filled without a manual load

- **WHEN** an admin opens the Analysis tab for an instrument whose provider has a stored catalog
- **THEN** the instrument dropdown already lists that provider's compatible models

### Requirement: Explicit refresh updates the dropdowns

The Analysis tab SHALL keep an explicit model refresh action that re-collects the provider
catalog and updates the instrument dropdowns with the refreshed models.

<!-- focused-spec:id=llm-settings.refresh-updates-dropdowns -->
#### Scenario: Refreshed catalog reaches the dropdown

- **WHEN** an admin triggers the model refresh action for an instrument's provider
- **THEN** the instrument dropdown lists the models returned by the refreshed catalog

### Requirement: A stale model catalog is indicated to the admin

When the models response reports that the catalog could not be refreshed, the Analysis tab
SHALL tell the admin that the model list may be outdated, in English and in Russian.

<!-- focused-spec:id=llm-settings.stale-notice-english -->
#### Scenario: Stale notice in English

- **WHEN** the Analysis tab renders the stale catalog notice while the UI language is English
- **THEN** the notice reads "The model list may be outdated"

<!-- focused-spec:id=llm-settings.stale-notice-russian -->
#### Scenario: Stale notice in Russian

- **WHEN** the Analysis tab renders the stale catalog notice while the UI language is Russian
- **THEN** the notice reads "Список моделей может быть устаревшим"
