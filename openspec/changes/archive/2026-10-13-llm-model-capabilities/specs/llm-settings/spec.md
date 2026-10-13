# Spec Delta

## MODIFIED Requirements

### Requirement: Analysis instrument dropdowns list only compatible models

The Analysis tab instrument model dropdowns SHALL list only models whose capabilities and modalities match the instrument type: chat, VL, OCR, image edit, and embedding.

#### Scenario: Chat dropdown lists only chat models

- **WHEN** an admin opens the model dropdown for the chat instrument
- **THEN** only models capable of chat are listed

<!-- focused-spec:id=llm-settings.chat-excludes-non-text -->
#### Scenario: Chat dropdown excludes non-text models

- **WHEN** an admin opens the model dropdown for the chat instrument for a provider whose model is capable only of `video_generation`, `text_to_speech`, or `speech_recognition`
- **THEN** that model is not listed

#### Scenario: VL dropdown lists only vision models

- **WHEN** an admin opens the model dropdown for the VL instrument
- **THEN** only models capable of vision are listed

#### Scenario: OCR dropdown lists only vision models

- **WHEN** an admin opens the model dropdown for the OCR instrument
- **THEN** only models capable of vision are listed

#### Scenario: Image edit dropdown lists only image editing models

- **WHEN** an admin opens the model dropdown for the image edit instrument
- **THEN** only models capable of image editing are listed

#### Scenario: Embedding dropdown lists only embedding models

- **WHEN** an admin opens the model dropdown for the embedding instrument
- **THEN** only models capable of embedding are listed

<!-- focused-spec:id=llm-settings.embedding-excludes-non-embedding -->
#### Scenario: Embedding dropdown excludes chat and vision models

- **WHEN** an admin opens the model dropdown for the embedding instrument for a provider whose model is capable of `chat`, `tool_calling`, and `vision` but not `embedding`
- **THEN** that model is not listed
