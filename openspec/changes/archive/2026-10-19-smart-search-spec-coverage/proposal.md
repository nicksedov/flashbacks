# Proposal

## Why

Smart search has backend unit coverage for exact-tag matching and result
ranking, but several user-facing behaviors are unprotected: request validation,
case/whitespace normalization, deterministic merge ranking, the webapp hook's
short-query guard and request abort, and the tab's result, empty, error, and
no-embeddings states. The master `smart-search` spec currently documents only
the two unavailable-search errors, so a regression in the search UX would go
unnoticed.

## What Changes

- Extend the `smart-search` capability with requirements and scenarios for:
  - request validation when the query is missing;
  - exact-tag matching that is case-insensitive and whitespace-normalized,
    including the no-match outcome;
  - deterministic ranking of merged exact and embedding results;
  - the webapp hook ignoring queries shorter than the minimum length;
  - the webapp hook aborting an in-flight request when a new search starts;
  - the tab rendering results, the empty state, the backend error message, and
    the no-embeddings hint.
- Add Go and Vitest tests that characterize these behaviors.
- Register every new scenario and its executable evidence in
  `.focused-spec/evidence.yaml`.

No production behavior change is planned. The new tests characterize the
current implementation; if a test exposes a real defect during apply, the fix
stays minimal and is recorded in the change.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `smart-search`: add requirements and scenarios for query validation,
  exact-tag matching, result ranking, and webapp search states/guards.

## Impact

- `backend/core/internal/application/imaging` — new Go unit tests for exact-tag
  matching and ranking.
- `backend/core/internal/interfaces/handler` — new handler test for query
  validation.
- `webapp/src/hooks/useSmartSearch.test.tsx` — new hook tests.
- `webapp/src/components/tabs/SmartSearchTab.test.tsx` — expanded tab tests.
- `.focused-spec/evidence.yaml` — new scenario bindings.
- `openspec/specs/smart-search/spec.md` — updated by sync after apply.
- No API, MCP contract, database schema, or dependency changes.

## Non-goals

- Changing the ranking algorithm, vector-search SQL, embedding backfill, or the
  MCP semantic-search tool behavior.
- Adding new smart-search UI features or restyling existing states.
- PostgreSQL/pgvector integration tests beyond the existing SQLite-safe unit
  coverage.
