# Spec Delta

## ADDED Requirements

### Requirement: Image generation and editing models are image-edit capable

The system SHALL record `image_edit`, and SHALL NOT record `chat` or `tool_calling`,
for a model that generates or edits images. An image-generation or editing family
SHALL be recognized from its model identifier, and a reported image output SHALL
imply the same capability.

<!-- focused-spec:id=llm-model-metadata.wan-image-identifier -->
#### Scenario: Wan image identifier is image-edit capable

- **WHEN** a provider exposes only the model identifier `wan2.7-image`
- **THEN** the model's capabilities are exactly `image_edit`

<!-- focused-spec:id=llm-model-metadata.image-family-identifiers -->
#### Scenario: Image family identifiers are image-edit capable

- **WHEN** a provider exposes only the identifiers `wan2.5-i2i-preview`, `wan2.1-t2i-turbo`, and `qwen-image-2.1-pro`
- **THEN** each of those models is recorded as image-edit capable

<!-- focused-spec:id=llm-model-metadata.video-family-not-image-edit -->
#### Scenario: Wan video identifier is not image-edit capable

- **WHEN** a provider exposes only the model identifier `wan2.7-t2v`
- **THEN** the model is not recorded as image-edit capable

<!-- focused-spec:id=llm-model-metadata.image-output-implies-image-edit -->
#### Scenario: Image output without capability codes implies image editing

- **WHEN** a provider lists a model with an image output and no capability codes
- **THEN** the model's capabilities are exactly `image_edit`

<!-- focused-spec:id=llm-model-metadata.alibaba-underreported-image-model -->
#### Scenario: Alibaba image model without the native code is image-edit capable

- **WHEN** the Alibaba catalog entry for `wan2.7-image` omits the native capability code but reports an image output
- **THEN** the model's capabilities are exactly `image_edit`

<!-- focused-spec:id=llm-model-metadata.alibaba-image-code-spellings -->
#### Scenario: Separator-less image-generation code is image-edit capable

- **WHEN** the Alibaba catalog reports the capability code `ImageGeneration`
- **THEN** the model's capabilities include `image_edit`

<!-- focused-spec:id=llm-model-metadata.alibaba-video-model-not-image-edit -->
#### Scenario: Wan video model stays video-only

- **WHEN** the Alibaba catalog reports the capability code `VG` for `wan2.7-t2v`
- **THEN** the model's capabilities are exactly `video_generation`

### Requirement: A refresh replaces a classification the provider determined

The system SHALL replace a stored model's capabilities when a catalog refresh
reports that model's modalities and capabilities, so a corrected classification
reaches a catalog that was collected before the correction existed.

<!-- focused-spec:id=llm-model-metadata.refresh-replaces-stored-classification -->
#### Scenario: Refresh corrects a stored chat classification

- **WHEN** a refresh reports the modalities of a stored model classified as `chat`
- **THEN** the stored capabilities become `image_edit`
