## Purpose

Defines the per-instrument LLM model/provider assignment for chat, VL, OCR, embedding, and image edit, and how LLM-based OCR is configured independently of VL.

## ADDED Requirements

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

- **WHEN** the api-service OpenAPI document is inspected
- **THEN** the `LlmInstrumentDTO.type` enum includes the value `ocr` alongside `chat`, `vl`, `embedding`, and `image_edit`
