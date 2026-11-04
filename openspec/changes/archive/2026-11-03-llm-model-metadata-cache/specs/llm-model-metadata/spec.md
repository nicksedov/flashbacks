# Spec Delta

## ADDED Requirements

### Requirement: Collected capabilities are deduplicated

The system SHALL record each standardized capability of a model at most once. A model whose
identifiers satisfy several capability detection rules SHALL still carry a single entry per
capability.

<!-- focused-spec:id=llm-model-metadata.capabilities-deduplicated -->
#### Scenario: Duplicate detection rules yield one capability

- **WHEN** a model's identifier satisfies both a vision family rule and an OCR rule
- **THEN** the model's recorded capabilities contain `vision` exactly once

### Requirement: Capabilities are derived from reported modalities

When a provider reports a model's input and output modalities but no capability codes, the
system SHALL derive the model's capabilities from those modalities.

<!-- focused-spec:id=llm-model-metadata.image-input-implies-vision -->
#### Scenario: Image input with text output is vision-capable

- **WHEN** a provider lists a model that accepts image input and answers with text but reports no capability codes
- **THEN** the model's recorded capabilities include `vision`

### Requirement: Model identifiers alone never grant vision

When a model is described only by its identifier and the model-family registry, the system
SHALL grant `vision` only if that identifier carries an explicit vision family token.

<!-- focused-spec:id=llm-model-metadata.chat-identifier-not-vision -->
#### Scenario: Chat model identifier is not vision-capable

- **WHEN** a chat model is described only by an identifier without a vision family token
- **THEN** the model is recorded without the `vision` capability
