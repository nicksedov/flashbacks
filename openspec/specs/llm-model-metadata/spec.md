# llm-model-metadata Specification

## Purpose

Defines how the system collects, stores, and surfaces metadata about LLM models — context length, modalities, capabilities, and token pricing — and how that metadata is shown to admins.

## Requirements

### Requirement: Model metadata includes modalities and token pricing

The system SHALL expose, for each available LLM model, its input modalities and its output modalities separately, and its input and output token price per one million tokens, alongside the existing context length and capabilities. An empty modality list or a price of zero SHALL represent an unknown value.

<!-- focused-spec:id=llm-model-metadata.metadata-exposed -->
#### Scenario: Metadata is exposed on the models API

- **WHEN** an available model is returned by the LLM models API
- **THEN** its entry includes `inputModalities`, `outputModalities`, `inputPricePerMTok`, `outputPricePerMTok`, `contextLength`, and `capabilities`

### Requirement: Providers normalize native model metadata

The system SHALL parse model metadata from each provider's native model-listing API and normalize its capabilities into the standardized vocabulary: `chat`, `tool_calling`, `vision`, `embedding`, `image_edit`, `reasoning`, `video_generation`, `text_to_speech`, `speech_recognition`, `realtime`, `omni`, and `world_model`. A model SHALL receive `chat` and `tool_calling` only when it can produce text output; a provider SHALL NOT derive `chat` or `tool_calling` for a model whose primary output is not text. Each modality SHALL be recorded with its direction — input or output — and a provider SHALL NOT expose an undifferentiated merged modality list.

<!-- focused-spec:id=llm-model-metadata.ollama-capabilities-normalized -->
#### Scenario: Ollama capabilities are normalized

- **WHEN** Ollama `/api/show` reports capabilities including `completion`, `thinking`, and `tools`
- **THEN** the model's capabilities are recorded as the standardized `chat` and `tool_calling` values

<!-- focused-spec:id=llm-model-metadata.ollama-vision-modality -->
#### Scenario: Ollama vision implies image input

- **WHEN** Ollama `/api/show` reports the `vision` capability
- **THEN** the model's input modalities include `image`

<!-- focused-spec:id=llm-model-metadata.ollama-chat-vision-not-embedding -->
#### Scenario: Ollama chat/vision model is not embedding capable

- **WHEN** Ollama `/api/show` reports the capabilities `completion`, `vision`, `tools`, and `thinking` for a model
- **THEN** the model's capabilities do not include `embedding`

<!-- focused-spec:id=llm-model-metadata.deepseek-context-parsed -->
#### Scenario: DeepSeek context window is parsed

- **WHEN** DeepSeek `/models` returns a `context_window` for a model
- **THEN** the model is recorded with a `contextLength` equal to that context window

<!-- focused-spec:id=llm-model-metadata.deepseek-modalities-parsed -->
#### Scenario: DeepSeek modalities are parsed

- **WHEN** DeepSeek `/models` returns `input_modalities` and `output_modalities` for a model
- **THEN** the model is recorded with `input_modalities` as its input modalities and `output_modalities` as its output modalities

<!-- focused-spec:id=llm-model-metadata.alibaba-pricing-parsed -->
#### Scenario: Alibaba token pricing is parsed

- **WHEN** Alibaba ModelStudio `/api/v1/models` returns token `prices` whose unit is "Per 1M tokens"
- **THEN** the model is recorded with `inputPricePerMTok` and `outputPricePerMTok` matching the input and output token prices

<!-- focused-spec:id=llm-model-metadata.alibaba-modalities-parsed -->
#### Scenario: Alibaba modalities are parsed

- **WHEN** Alibaba ModelStudio `/api/v1/models` returns `inference_metadata.request_modality` and `response_modality` for a model
- **THEN** the model is recorded with `request_modality` as its input modalities and `response_modality` as its output modalities

<!-- focused-spec:id=llm-model-metadata.alibaba-image-generation -->
#### Scenario: Alibaba image-generation models are image-edit capable

- **WHEN** Alibaba ModelStudio reports the capability `IG` for `qwen-image-2.1-pro`
- **THEN** the model's capabilities are exactly `image_edit`

<!-- focused-spec:id=llm-model-metadata.alibaba-video-generation -->
#### Scenario: Alibaba video-generation models are video-generation capable

- **WHEN** Alibaba ModelStudio reports the capability `VG` for a video model
- **THEN** the model's capabilities are exactly `video_generation`

<!-- focused-spec:id=llm-model-metadata.alibaba-world-model -->
#### Scenario: Alibaba world models are video-generation capable

- **WHEN** Alibaba ModelStudio reports the capability `World-Model`
- **THEN** the model's capabilities include both `world_model` and `video_generation`

<!-- focused-spec:id=llm-model-metadata.alibaba-text-to-speech -->
#### Scenario: Alibaba text-to-speech models are speech-synthesis capable

- **WHEN** Alibaba ModelStudio reports the capability `TTS` for a text-to-speech model
- **THEN** the model's capabilities are exactly `text_to_speech`

<!-- focused-spec:id=llm-model-metadata.alibaba-speech-recognition -->
#### Scenario: Alibaba speech-recognition models are transcription capable

- **WHEN** Alibaba ModelStudio reports the capability `ASR` for a speech-recognition model
- **THEN** the model's capabilities are exactly `speech_recognition`

<!-- focused-spec:id=llm-model-metadata.alibaba-realtime -->
#### Scenario: Alibaba realtime codes add the realtime capability

- **WHEN** Alibaba ModelStudio reports the capability `Realtime-ASR`
- **THEN** the model's capabilities include both `realtime` and `speech_recognition`

<!-- focused-spec:id=llm-model-metadata.alibaba-omni -->
#### Scenario: Alibaba omni models are multimodal conversational

- **WHEN** Alibaba ModelStudio reports the capability `Multimodal-Omni`
- **THEN** the model's capabilities include both `omni` and `chat`

<!-- focused-spec:id=llm-model-metadata.alibaba-reasoning -->
#### Scenario: Alibaba reasoning flag is preserved alongside chat

- **WHEN** Alibaba ModelStudio reports the capabilities `Reasoning` and `TG` for a text-output model
- **THEN** the model's capabilities include both `reasoning` and `chat`

### Requirement: Model details window shows the collected metadata

The LLM Providers tab SHALL present a model details view that shows a model's context length, its input modalities, its output modalities, capabilities, and input and output token price. Each capability SHALL be displayed with a human-readable, localized label.

<!-- focused-spec:id=llm-model-metadata.model-details-window -->
#### Scenario: Model details window shows all metadata

- **WHEN** an admin opens the details of a model in the LLM Providers tab
- **THEN** the details view shows the model's context length, input modalities, output modalities, capabilities, and input and output token price

<!-- focused-spec:id=llm-model-metadata.model-details-new-capability-label -->
#### Scenario: Model details window labels a new capability

- **WHEN** an admin opens the details of a model whose capabilities include `video_generation`
- **THEN** the details view displays a localized label for `video_generation` instead of the raw value
