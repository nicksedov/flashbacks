# LLM Settings Specification

## Purpose

Defines the per-instrument LLM model/provider assignment for chat, VL, OCR, embedding, and image edit, and how LLM-based OCR is configured independently of VL.

## Requirements

### Requirement: OCR LLM is a distinct instrument

The system SHALL support an `ocr` instrument type, separate from `vl`, so that LLM-based OCR has its own provider and model assignment. The LLM settings response SHALL include an instrument entry of type `ocr` whenever one is configured.

#### Scenario: OCR instrument appears in settings

- **WHEN** an admin fetches LLM settings via `GET /api/llm/settings`
- **THEN** the `instruments` array contains at most one entry whose `type` equals `ocr`, carrying its own `providerId` and `model`

#### Scenario: OCR instrument type is accepted on update

- **WHEN** an admin updates settings with `instrumentType: "ocr"` and an `instrumentModel`
- **THEN** the system persists a separate `ocr` instrument row without modifying the `vl` instrument

### Requirement: LLM OCR recognition uses the OCR instrument

The LLM-based OCR recognition flow SHALL resolve its model and provider from the `ocr` instrument rather than the `vl` instrument.

#### Scenario: Recognize with independently configured OCR model

- **WHEN** an admin starts LLM OCR recognition via `POST /api/llm/recognize`
- **THEN** the recognition task uses the provider and model assigned to the `ocr` instrument, not the `vl` instrument

#### Scenario: Missing OCR instrument fails cleanly

- **WHEN** recognition is requested while no `ocr` instrument is configured
- **THEN** the system responds with an error indicating the OCR settings are not configured, without falling back to the `vl` instrument

### Requirement: recognizeText AI action uses the OCR instrument

The `recognizeText` AI action SHALL use the `ocr` instrument, while `describe`, `tags`, and `askQuestion` continue to use the `vl` instrument.

#### Scenario: recognizeText resolves OCR client

- **WHEN** an AI action request with `action: "recognizeText"` is submitted
- **THEN** the task uses the provider and model from the `ocr` instrument

#### Scenario: VL actions still resolve VL client

- **WHEN** an AI action request with `action: "describe"`, `"tags"`, or `"askQuestion"` is submitted
- **THEN** the task uses the provider and model from the `vl` instrument

### Requirement: VL and OCR settings are independently configurable

Changing the VL model/provider SHALL NOT change the OCR model/provider, and changing the OCR model/provider SHALL NOT change the VL model/provider.

#### Scenario: Independent model assignment

- **WHEN** an admin changes the VL model and then changes the OCR model to a different value
- **THEN** the VL model remains at its previously saved value and the OCR model is persisted separately

### Requirement: OCR LLM settings card in Analysis tab

The admin Analysis tab SHALL render an "OCR LLM Settings" card directly below the "VL LLM Settings" card, using the same provider and model selectors as the VL card. The VL card description SHALL no longer mention OCR.

#### Scenario: OCR card placement

- **WHEN** an admin opens the Analysis tab
- **THEN** an "OCR LLM Settings" card is rendered immediately below the "VL LLM Settings" card

#### Scenario: Localized labels

- **WHEN** the UI is displayed in English
- **THEN** the OCR card title reads "OCR LLM Settings" and the VL description no longer includes "OCR"
- **WHEN** the UI is displayed in Russian
- **THEN** the OCR card title reads "Настройки OCR LLM" and the VL description no longer includes "OCR"

### Requirement: API contract exposes the OCR instrument type

The API contract SHALL list `ocr` as a valid value for the LLM instrument `type` field.

#### Scenario: OpenAPI enum includes ocr

- **WHEN** the core OpenAPI document is inspected
- **THEN** the `LlmInstrumentDTO.type` enum includes the value `ocr` alongside `chat`, `vl`, `embedding`, and `image_edit`

### Requirement: Analysis instrument dropdowns list only compatible models

The Analysis tab instrument model dropdowns SHALL list only models whose capabilities and modalities match the instrument type: chat, VL, OCR, image edit, and embedding.

<!-- focused-spec:id=llm-settings.chat-filter -->
#### Scenario: Chat dropdown lists only chat models

- **WHEN** an admin opens the model dropdown for the chat instrument
- **THEN** only models capable of chat are listed

<!-- focused-spec:id=llm-settings.chat-excludes-non-text -->
#### Scenario: Chat dropdown excludes non-text models

- **WHEN** an admin opens the model dropdown for the chat instrument for a provider whose model is capable only of `video_generation`, `text_to_speech`, or `speech_recognition`
- **THEN** that model is not listed

<!-- focused-spec:id=llm-settings.vl-filter -->
#### Scenario: VL dropdown lists only vision models

- **WHEN** an admin opens the model dropdown for the VL instrument
- **THEN** only models capable of vision are listed

<!-- focused-spec:id=llm-settings.ocr-filter -->
#### Scenario: OCR dropdown lists only vision models

- **WHEN** an admin opens the model dropdown for the OCR instrument
- **THEN** only models capable of vision are listed

<!-- focused-spec:id=llm-settings.image-edit-filter -->
#### Scenario: Image edit dropdown lists only image editing models

- **WHEN** an admin opens the model dropdown for the image edit instrument
- **THEN** only models capable of image editing are listed

<!-- focused-spec:id=llm-settings.embedding-filter -->
#### Scenario: Embedding dropdown lists only embedding models

- **WHEN** an admin opens the model dropdown for the embedding instrument
- **THEN** only models capable of embedding are listed

<!-- focused-spec:id=llm-settings.embedding-excludes-non-embedding -->
#### Scenario: Embedding dropdown excludes chat and vision models

- **WHEN** an admin opens the model dropdown for the embedding instrument for a provider whose model is capable of `chat`, `tool_calling`, and `vision` but not `embedding`
- **THEN** that model is not listed

<!-- focused-spec:id=llm-settings.unknown-instrument-fails-closed -->
#### Scenario: Unknown instrument type lists no models

- **WHEN** the model filter receives an instrument type it does not recognize
- **THEN** it fails closed and returns no models instead of listing unfiltered models

### Requirement: Instrument model dropdowns provide a search field

The Analysis tab model dropdowns SHALL render an editable search field while the dropdown is open, so an admin can narrow the listed models by typing. The field SHALL be available on every instrument model dropdown: chat, VL, OCR, image edit, and embedding.

<!-- focused-spec:id=llm-settings.model-search-visible -->
#### Scenario: Search field appears in an open model dropdown

- **WHEN** an admin opens the model dropdown of an instrument in the Analysis tab
- **THEN** the open dropdown renders an editable search field above the model options

<!-- focused-spec:id=llm-settings.model-search-embedding-card -->
#### Scenario: Search field appears in the embedding model dropdown

- **WHEN** an admin opens the model dropdown inside the "Embeddings LLM Settings" card
- **THEN** the open dropdown renders an editable search field above the model options

### Requirement: Typed query filters models by substring

While the search field holds a query, the dropdown SHALL list only models whose display label contains that query as a case-insensitive substring. The filter SHALL narrow only the models already eligible for the instrument and SHALL NOT reintroduce a model the instrument type excludes.

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

When no compatible model label contains the query, the dropdown SHALL show a localized no-match message in place of model options, and that message SHALL be provided in both `en` and `ru`.

<!-- focused-spec:id=llm-settings.model-search-empty-state-en -->
#### Scenario: No-match message in English

- **WHEN** an admin types a query that matches no compatible model and the UI language is English
- **THEN** the dropdown shows the message "No models match your search"

<!-- focused-spec:id=llm-settings.model-search-empty-state-ru -->
#### Scenario: No-match message in Russian

- **WHEN** an admin types a query that matches no compatible model and the UI language is Russian
- **THEN** the dropdown shows the message "Нет моделей, соответствующих поиску"

### Requirement: Search does not change instrument selection semantics

Filtering SHALL preserve the instrument's stored model, and choosing an option from a filtered list SHALL save that model exactly as choosing from the unfiltered list did.

<!-- focused-spec:id=llm-settings.model-search-keeps-saved-model -->
#### Scenario: Closing a filtered dropdown without choosing keeps the saved model

- **WHEN** an admin types a query that hides the currently saved model and then closes the dropdown without selecting an option
- **THEN** the instrument's stored model is unchanged and no update request is sent

<!-- focused-spec:id=llm-settings.model-search-select-saves -->
#### Scenario: Selecting a filtered option saves that model

- **WHEN** an admin selects a model that is listed only because of the active search query
- **THEN** the instrument is saved with that model id
