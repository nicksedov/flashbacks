# Spec Delta

## ADDED Requirements

### Requirement: Instrument model dropdowns provide a search field

The Analysis tab model dropdowns SHALL render an editable search field while the
dropdown is open, so an admin can narrow the listed models by typing. The field
SHALL be available on every instrument model dropdown: chat, VL, OCR, image edit,
and embedding.

<!-- focused-spec:id=llm-settings.model-search-visible -->
#### Scenario: Search field appears in an open model dropdown

- **WHEN** an admin opens the model dropdown of an instrument in the Analysis tab
- **THEN** the open dropdown renders an editable search field above the model options

<!-- focused-spec:id=llm-settings.model-search-embedding-card -->
#### Scenario: Search field appears in the embedding model dropdown

- **WHEN** an admin opens the model dropdown inside the "Embeddings LLM Settings" card
- **THEN** the open dropdown renders an editable search field above the model options

### Requirement: Typed query filters models by substring

While the search field holds a query, the dropdown SHALL list only models whose
display label contains that query as a case-insensitive substring. The filter
SHALL narrow only the models already eligible for the instrument and SHALL NOT
reintroduce a model the instrument type excludes.

<!-- focused-spec:id=llm-settings.model-search-filters-substring -->
#### Scenario: Query narrows the model list

- **WHEN** an admin types `mini` into the search field of the chat instrument model dropdown whose catalog offers `minicpm-v` and `llama3`
- **THEN** only `minicpm-v` is listed

<!-- focused-spec:id=llm-settings.model-search-case-insensitive -->
#### Scenario: Query matching ignores letter case

- **WHEN** an admin types `MINI` into the search field of the chat instrument model dropdown whose catalog offers `minicpm-v`
- **THEN** `minicpm-v` is listed

<!-- focused-spec:id=llm-settings.model-search-respects-compatibility -->
#### Scenario: Query cannot surface incompatible models

- **WHEN** an admin types a query matching a provider model that lacks the capability required by the open instrument's dropdown
- **THEN** that model is not listed

<!-- focused-spec:id=llm-settings.model-search-clear-restores -->
#### Scenario: Clearing the query restores every compatible model

- **WHEN** an admin clears the search field after filtering the model dropdown
- **THEN** every model compatible with the instrument is listed again

<!-- focused-spec:id=llm-settings.model-search-reopen-resets -->
#### Scenario: Reopening the dropdown starts unfiltered

- **WHEN** an admin closes a model dropdown that has a search query and reopens it
- **THEN** the search field is empty and every compatible model is listed

### Requirement: Empty search result state is localized

When no compatible model label contains the query, the dropdown SHALL show a
localized no-match message in place of model options, and that message SHALL be
provided in both `en` and `ru`.

<!-- focused-spec:id=llm-settings.model-search-empty-state-en -->
#### Scenario: No-match message in English

- **WHEN** an admin types a query that matches no compatible model and the UI language is English
- **THEN** the dropdown shows the message "No models match your search"

<!-- focused-spec:id=llm-settings.model-search-empty-state-ru -->
#### Scenario: No-match message in Russian

- **WHEN** an admin types a query that matches no compatible model and the UI language is Russian
- **THEN** the dropdown shows the message "Нет моделей, соответствующих поиску"

### Requirement: Search does not change instrument selection semantics

Filtering SHALL preserve the instrument's stored model, and choosing an option
from a filtered list SHALL save that model exactly as choosing from the unfiltered
list did.

<!-- focused-spec:id=llm-settings.model-search-keeps-saved-model -->
#### Scenario: Closing a filtered dropdown without choosing keeps the saved model

- **WHEN** an admin types a query that hides the currently saved model and then closes the dropdown without selecting an option
- **THEN** the instrument's stored model is unchanged and no update request is sent

<!-- focused-spec:id=llm-settings.model-search-select-saves -->
#### Scenario: Selecting a filtered option saves that model

- **WHEN** an admin selects a model that is listed only because of the active search query
- **THEN** the instrument is saved with that model id
