# llm-model-metadata Specification

## Purpose

Defines how the system collects, stores, and surfaces metadata about LLM models — context length, modalities, capabilities, and token pricing — and how that metadata is shown to admins.

## Requirements

### Requirement: Model metadata includes modalities and token pricing

The system SHALL expose, for each available LLM model, its input and output modalities and its input and output token price per one million tokens, alongside the existing context length and capabilities. An empty modality list or a price of zero SHALL represent an unknown value.

<!-- focused-spec:id=llm-model-metadata.metadata-exposed -->
#### Scenario: Metadata is exposed on the models API

- **WHEN** an available model is returned by the LLM models API
- **THEN** its entry includes `modalities`, `inputPricePerMTok`, `outputPricePerMTok`, `contextLength`, and `capabilities`

### Requirement: Providers normalize native model metadata

The system SHALL parse model metadata from each provider's native model-listing API and normalize it into the shared metadata shape.

<!-- focused-spec:id=llm-model-metadata.ollama-capabilities-normalized -->
#### Scenario: Ollama capabilities are normalized

- **WHEN** Ollama `/api/show` reports capabilities including `completion`, `thinking`, and `tools`
- **THEN** the model's capabilities are recorded as the standardized `chat` and `tool_calling` values

<!-- focused-spec:id=llm-model-metadata.ollama-vision-modality -->
#### Scenario: Ollama vision implies image input

- **WHEN** Ollama `/api/show` reports the `vision` capability
- **THEN** the model's input modalities include `image`

<!-- focused-spec:id=llm-model-metadata.deepseek-context-parsed -->
#### Scenario: DeepSeek context window is parsed

- **WHEN** DeepSeek `/models` returns a `context_window` for a model
- **THEN** the model is recorded with a `contextLength` equal to that context window

<!-- focused-spec:id=llm-model-metadata.deepseek-modalities-parsed -->
#### Scenario: DeepSeek modalities are parsed

- **WHEN** DeepSeek `/models` returns `input_modalities` and `output_modalities` for a model
- **THEN** the model is recorded with those modalities

<!-- focused-spec:id=llm-model-metadata.alibaba-pricing-parsed -->
#### Scenario: Alibaba token pricing is parsed

- **WHEN** Alibaba ModelStudio `/api/v1/models` returns token `prices` whose unit is "Per 1M tokens"
- **THEN** the model is recorded with `inputPricePerMTok` and `outputPricePerMTok` matching the input and output token prices

<!-- focused-spec:id=llm-model-metadata.alibaba-modalities-parsed -->
#### Scenario: Alibaba modalities are parsed

- **WHEN** Alibaba ModelStudio `/api/v1/models` returns `inference_metadata.request_modality` and `response_modality` for a model
- **THEN** the model is recorded with those modalities

### Requirement: Model details window shows the collected metadata

The LLM Providers tab SHALL present a model details view that shows a model's context length, modalities, capabilities, and input and output token price.

<!-- focused-spec:id=llm-model-metadata.model-details-window -->
#### Scenario: Model details window shows all metadata

- **WHEN** an admin opens the details of a model in the LLM Providers tab
- **THEN** the details view shows the model's context length, modalities, capabilities, and input and output token price
