# Spec Delta

## ADDED Requirements

### Requirement: Analysis instrument dropdowns list only compatible models

The Analysis tab instrument model dropdowns SHALL list only models whose capabilities and modalities match the instrument type: chat, VL, OCR, image edit, and embedding.

<!-- focused-spec:id=llm-settings.chat-filter -->
#### Scenario: Chat dropdown lists only chat models

- **WHEN** an admin opens the model dropdown for the chat instrument
- **THEN** only models capable of chat are listed

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
