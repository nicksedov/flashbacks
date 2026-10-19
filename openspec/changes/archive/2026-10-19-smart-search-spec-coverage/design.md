# Design

## Context

See [`proposal.md`](proposal.md) — Why. Smart search spans two components: the
core Go handler plus the `imaging` application functions, and the React hook and
tab in the webapp. The core unit tests run on in-memory SQLite, so they cannot
exercise pgvector or a live embedding provider. The existing Go tests already
cover parts of exact-tag matching and result merging; the webapp hook has no
tests and the tab test only asserts mobile layout.

## Goals / Non-Goals

**Goals:**

- Characterize the observable smart-search behavior the specs describe with
  executable tests at the closest practical layer.
- Bind every new scenario to exactly one evidence target (or one explicit AND
  contract where a single test cannot observe the whole outcome).
- Keep the test additions fast, deterministic, and SQLite-safe.

**Non-Goals:**

- Testing the pgvector SQL, the real embedding provider, or the MCP semantic
  search tool.
- Changing ranking, normalization, or UI behavior.
- Browser-level or visual-regression testing of the tab.

## Decisions

**1. Test behavior at the narrowest layer that can observe it.**

- Exact-tag normalization and merge ranking are pure application behaviors:
  Go tests in `internal/application/imaging`, seeded through the existing test
  DB helper.
- Missing-query validation is a handler contract: one handler test using the
  existing `serveHandler` helper and a minimal `Server` wired with the i18n
  service.
- Hook guard/abort behavior: Vitest `renderHook` tests with
  `@/api/endpoints` mocked, so no network is involved.
- Tab states: React Testing Library tests in the existing
  `SmartSearchTab.test.tsx`, with the hook and polling modules mocked.

Alternative considered: end-to-end tests through the HTTP API. Rejected
because a successful smart-search response requires a populated pgvector table
and an embedding provider, which the SQLite unit environment cannot supply.

**2. One combined exact-tag test for the normalization scenario.**

The scenario asserts case-insensitivity, whitespace collapse, similarity, and
match type together. A single test that seeds `Mountain View`, queries
`  mountain   view  `, and asserts the full result is clearer than attaching
four narrow tests to one scenario.

**3. Component tests are driven by replaceable mock state.**

The current `SmartSearchTab` test mocks `useSmartSearch` with a fixed object.
The test file will expose a hoisted mutable state per test (hook result and
polling data) so results, empty, error, and no-embeddings cases reuse one render
helper. Assertions target user-visible text and roles rather than CSS.

**4. Evidence registration happens in apply.**

The planning phase records the intended exact runner selectors below. The
`.focused-spec/evidence.yaml` entries are added during apply, when the tests
exist and the focused-spec CLI can run them.

## Evidence plan

| Scenario ID | Runner | Intended selector |
|---|---|---|
| `smart-search.query-required` | `go` | `./internal/interfaces/handler::TestHandleSmartSearch_QueryRequired` |
| `smart-search.exact-tag-normalized` | `go` | `./internal/application/imaging::TestSearchByExactTag_NormalizesCaseAndWhitespace` |
| `smart-search.exact-tag-no-match` | `go` | `./internal/application/imaging::TestSearchByExactTag_NoMatch` |
| `smart-search.ranked-merge` | `go` | `./internal/application/imaging::TestMergeAndRankResults_AllThreeGroupsPresent` |
| `smart-search.short-query-ignored` | `vitest` | `webapp/src/hooks/useSmartSearch.test.tsx::useSmartSearch > ignores queries shorter than the minimum length` |
| `smart-search.previous-request-aborted` | `vitest` | `webapp/src/hooks/useSmartSearch.test.tsx::useSmartSearch > aborts the previous request when a new search starts` |
| `smart-search.results-rendered` | `vitest` | `webapp/src/components/tabs/SmartSearchTab.test.tsx::SmartSearchTab > renders the result count and a tile per result` |
| `smart-search.empty-state` | `vitest` | `webapp/src/components/tabs/SmartSearchTab.test.tsx::SmartSearchTab > shows the empty state after a search with no results` |
| `smart-search.error-shown` | `vitest` | `webapp/src/components/tabs/SmartSearchTab.test.tsx::SmartSearchTab > shows the backend error message when the search fails` |
| `smart-search.no-embeddings-hint` | `vitest` | `webapp/src/components/tabs/SmartSearchTab.test.tsx::SmartSearchTab > shows the no-embeddings hint when no embeddings exist` |

## Risks / Trade-offs

- **SQLite cannot observe the ranked merge end to end** → The merge is tested as
  a pure function with explicit inputs; the vector query itself stays covered by
  the existing not-indexed/not-configured tests and production deployment checks.
- **Hook tests involve async state updates** → Wrap calls and assertions in
  React Testing Library `act`/`waitFor` to avoid act warnings and flakiness.
- **Component mocks can drift from real hook shapes** → Keep the mock state typed
  as the real hook return type and assert only user-visible output.
- **Existing mobile-layout test must keep passing** → Reuse the same module
  mocks and only replace their state, leaving the layout assertions untouched.
