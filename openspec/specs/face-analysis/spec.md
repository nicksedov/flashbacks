# face-analysis Specification

## Purpose

Makes face model execution hardware-aware and batched: the same detection and
embedding pipeline runs on a GPU when one is available, degrades to CPU when it
is not, and can analyse several images per request so a GPU is actually kept
busy.

## Requirements

### Requirement: Execution provider comes from configuration

The faces service SHALL select ONNX Runtime execution providers from the
configured provider list, in the configured order, and SHALL use only providers
the installed runtime reports as available.

<!-- focused-spec:id=face-analysis.provider-configurable -->
#### Scenario: The configured GPU provider is used when the runtime offers it

- **WHEN** the service is configured with providers `cuda,cpu` and the installed runtime reports `CUDAExecutionProvider` as available
- **THEN** the service reports `cuda` as its active execution provider

<!-- focused-spec:id=face-analysis.provider-cpu-default -->
#### Scenario: An unconfigured service runs on the CPU provider

- **WHEN** the service starts without any provider configured
- **THEN** the service reports `cpu` as its active execution provider

### Requirement: An unavailable provider degrades to CPU

When a configured acceleration provider is not available, the faces service SHALL
continue with the remaining available providers and SHALL report that fallback
instead of failing analysis.

<!-- focused-spec:id=face-analysis.provider-fallback -->
#### Scenario: Requested CUDA on a CPU-only runtime falls back

- **WHEN** the service is configured with providers `cuda,cpu` and the runtime reports only `CPUExecutionProvider`
- **THEN** the service reports `cpu` as its active execution provider and analysis requests still succeed

### Requirement: Health reports the active provider

`GET /faces/health` SHALL report the active execution provider of the running
service so an operator can confirm whether the GPU path is live.

<!-- focused-spec:id=face-analysis.health-reports-provider -->
#### Scenario: Health response names the active provider

- **WHEN** the health endpoint is called on a service whose active provider is `cuda`
- **THEN** the response reports `status` equal to `ok` and an active provider of `cuda`

### Requirement: A batch request analyses several images at once

`POST /faces/analyze-batch` SHALL detect and embed the faces of every submitted
image in one request and SHALL return exactly one entry per submitted image, in
request order.

<!-- focused-spec:id=face-analysis.batch-entry-per-image -->
#### Scenario: A batch response carries one entry per image in order

- **WHEN** a batch of three images is analysed and the second image contains two faces
- **THEN** the response carries three entries whose second entry lists two embeddings

### Requirement: An unusable image does not fail its batch

A batch analysis response SHALL report a per-image failure without failing the
whole request, so one unreadable photo cannot stop a scan batch.

<!-- focused-spec:id=face-analysis.batch-image-isolated -->
#### Scenario: An undecodable image reports its own error

- **WHEN** one image of a batch cannot be decoded and the others can
- **THEN** that image's entry carries an error and the remaining entries still carry their detected faces

### Requirement: The recognition job submits images in batches

The background face recognition job SHALL submit up to the configured batch size
images per faces-service request instead of one request per photo.

<!-- focused-spec:id=face-analysis.job-batch-size -->
#### Scenario: Five images with batch size two use three requests

- **WHEN** the recognition job has five images to analyse and the batch size is two
- **THEN** the faces service receives three batch requests covering those five images

### Requirement: A failed batch leaves its images retryable

When a batch request fails, the recognition job SHALL leave every image of that
batch unprocessed so the next pass analyses it again.

<!-- focused-spec:id=face-analysis.failed-batch-retried -->
#### Scenario: Images of a failed batch are analysed again next pass

- **WHEN** a batch request fails for two images
- **THEN** both images keep their previous scan version and are analysed again on the next pass
