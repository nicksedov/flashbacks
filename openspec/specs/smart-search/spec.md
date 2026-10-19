# Smart Search Specification

## Purpose

Defines how the core service answers a semantic (smart) search query when the
image embeddings index is unavailable, so the user receives an actionable,
human-readable explanation instead of an empty result set or a raw message key.

## Requirements

### Requirement: Unindexed library is reported instead of an empty search

When the embeddings index for the configured embedding model does not exist or
contains no rows, the core SHALL reject the smart-search request with HTTP 409
and a human-readable message stating that images must be indexed before smart
search can run. The message SHALL use the `smart.not_indexed` translation (en
and ru) and MUST NOT expose the raw message key.

<!-- focused-spec:id=smart-search.not-indexed -->
#### Scenario: Smart search explains that images are not indexed

- **WHEN** a user runs a smart-search query and no image embeddings have been generated
- **THEN** the response is HTTP 409 with a human-readable message that tells the user to generate embeddings and retry, rather than an empty result set or the raw `smart.not_indexed` key

### Requirement: Missing embedding configuration is reported separately

When no embedding instrument is configured, the core SHALL reject the
smart-search request with HTTP 409 and a human-readable message stating that an
embedding model must be configured before smart search can run. The message
SHALL use the `smart.not_configured` translation (en and ru) and MUST NOT expose
the raw message key.

<!-- focused-spec:id=smart-search.not-configured -->
#### Scenario: Smart search explains that the embedding model is not configured

- **WHEN** a user runs a smart-search query and no embedding instrument is configured
- **THEN** the response is HTTP 409 with a human-readable message that tells the user to configure an embedding model and index images, rather than the raw `smart.not_configured` key
