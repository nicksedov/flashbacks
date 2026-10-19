# Spec Delta

## ADDED Requirements

### Requirement: Missing search query is rejected

The core SHALL reject a smart-search request that does not provide a `q` query parameter with HTTP 400 and the localized `smart.query_required` message. The response MUST contain human-readable text rather than the raw message key.

<!-- focused-spec:id=smart-search.query-required -->
#### Scenario: Smart search rejects a request without a query

- **WHEN** a smart-search request is sent without the `q` parameter
- **THEN** the response is HTTP 400 with a human-readable validation message telling the user to enter a search query

### Requirement: Exact-tag matching normalizes the query

The exact-tag portion of smart search SHALL match tags case-insensitively after trimming leading and trailing whitespace and collapsing runs of internal whitespace to one space. A matched image SHALL report similarity `1.0` and match type `exact`.

<!-- focused-spec:id=smart-search.exact-tag-normalized -->
#### Scenario: Case and whitespace differences still match a tag

- **WHEN** an image is tagged `Mountain View` and the query is `  mountain   view  `
- **THEN** that image is returned with similarity `1.0` and match type `exact`

### Requirement: Exact-tag search reports no matches

The exact-tag portion of smart search SHALL return an empty result set, without an error, when no tag matches the normalized query.

<!-- focused-spec:id=smart-search.exact-tag-no-match -->
#### Scenario: Unknown tag yields no exact matches

- **WHEN** no image tag matches the query
- **THEN** exact-tag search returns no results and no error

### Requirement: Merged results are ranked and deduplicated

The merged smart-search response SHALL order images matching both streams first, then exact-tag-only images, then embedding-only images. An image present in both streams SHALL appear exactly once with match type `both` and a similarity above `1.0`.

<!-- focused-spec:id=smart-search.ranked-merge -->
#### Scenario: Both-stream images rank above single-stream images

- **WHEN** exact-tag and embedding search return overlapping and distinct images
- **THEN** each both-stream image appears once at the front, followed by exact-only images and then embedding-only images

### Requirement: Short queries are ignored by the search hook

The webapp smart-search hook SHALL NOT call the search API when the trimmed query is shorter than three characters, and SHALL keep the search state reset.

<!-- focused-spec:id=smart-search.short-query-ignored -->
#### Scenario: Query below the minimum length performs no search

- **WHEN** the hook is asked to search for `ab`
- **THEN** no API request is made and the hook reports no completed search

### Requirement: Starting a search aborts the previous request

The webapp smart-search hook SHALL abort any in-flight search request before starting a new one.

<!-- focused-spec:id=smart-search.previous-request-aborted -->
#### Scenario: A new search aborts the in-flight request

- **WHEN** a second search starts before the first one completes
- **THEN** the first request's abort signal is aborted

### Requirement: Search results are rendered with a count

The smart-search tab SHALL render the result count and one tile per returned image when results are available.

<!-- focused-spec:id=smart-search.results-rendered -->
#### Scenario: Results and their count are visible

- **WHEN** the hook reports a completed search with results
- **THEN** the tab shows the result count and every returned file name

### Requirement: Empty search shows the empty state

The smart-search tab SHALL show its empty state when a completed search returns zero results and no error.

<!-- focused-spec:id=smart-search.empty-state -->
#### Scenario: A search with no matches shows the empty state

- **WHEN** the hook reports a completed search with zero results and no error
- **THEN** the tab shows the empty-state title and hint

### Requirement: Search errors are shown to the user

The smart-search tab SHALL display the error message reported by the search hook in an error banner.

<!-- focused-spec:id=smart-search.error-shown -->
#### Scenario: A failed search displays its message

- **WHEN** the hook reports an error message
- **THEN** the tab renders that message in the error banner

### Requirement: Missing embeddings are advertised in the tab

The smart-search tab SHALL show the no-embeddings hint when the embedding status reports no embeddings and no backfill is running.

<!-- focused-spec:id=smart-search.no-embeddings-hint -->
#### Scenario: An unindexed library shows the embeddings hint

- **WHEN** the embedding status reports zero embeddings and no running backfill
- **THEN** the tab shows the no-embeddings hint and configuration guidance
