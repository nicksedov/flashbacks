# Tasks

## 1. Backend coverage

- [x] 1.1 Add `TestSearchByExactTag_NormalizesCaseAndWhitespace` to `backend/core/internal/application/imaging/smart_search_test.go`, seeding a `Mountain View` tag and querying `  mountain   view  ` while asserting similarity `1.0` and `MatchExact`; verify with `cd backend/core && go test ./internal/application/imaging/... -count=1`
- [x] 1.2 Add `TestHandleSmartSearch_QueryRequired` to `backend/core/internal/interfaces/handler/handlers_smart_search_test.go`, asserting HTTP 400 and the resolved `smart.query_required` message; verify with `cd backend/core && go test ./internal/interfaces/handler/... -count=1`
- [x] 1.3 Confirm the existing `TestSearchByExactTag_NoMatch` and `TestMergeAndRankResults_AllThreeGroupsPresent` remain green and cover their scenarios; verify with `cd backend/core && go test ./internal/application/... -count=1 && go vet ./... && go build ./...`

## 2. Webapp hook coverage

- [x] 2.1 Create `webapp/src/hooks/useSmartSearch.test.tsx` with tests for the short-query guard (no API call, no completed search) and previous-request abort (first signal aborted when a second search starts); verify with `cd webapp && npm test -- useSmartSearch && npm run lint && npx tsc -b`

## 3. Webapp tab coverage

- [x] 3.1 Refactor the `useSmartSearch` and `usePolling` mocks in `webapp/src/components/tabs/SmartSearchTab.test.tsx` to hoisted mutable state while keeping the existing mobile-layout test unchanged
- [x] 3.2 Add tests for rendered results and count, the empty state, the backend error message, and the no-embeddings hint; verify with `cd webapp && npm test -- SmartSearchTab && npm run lint && npx tsc -b`

## 4. Evidence integration

- [x] 4.1 Register the ten scenario entries from the design evidence plan in `.focused-spec/evidence.yaml` and verify `node_modules/.bin/focused-spec validate --scope current --strict` passes
- [x] 4.2 Run each new scenario with `node_modules/.bin/focused-spec run --scope current --scenario <id>` and confirm PASS
- [x] 4.3 Mutation-check every new evidence entry: introduce a reversible product regression, observe the scenario FAIL on its behavioral assertion, restore, and re-run PASS; record each fail → restore → pass pair
- [x] 4.4 Run the full scope with `node_modules/.bin/focused-spec run --scope current` and confirm 0 FAIL/SKIP/ERROR

### Mutation-check record (each pair: FAIL on regression → PASS after restore)

| Scenario | Reversible regression | Observed |
|---|---|---|
| `smart-search.query-required` | missing-query guard disabled in the handler | FAIL → PASS |
| `smart-search.exact-tag-normalized` | query whitespace normalization replaced with a trim | FAIL → PASS |
| `smart-search.exact-tag-no-match` | tag lookup filter removed | FAIL → PASS |
| `smart-search.ranked-merge` | exact-only group appended before the both-stream group | FAIL → PASS |
| `smart-search.short-query-ignored` | minimum-query-length guard lowered | FAIL → PASS |
| `smart-search.previous-request-aborted` | in-flight abort call removed | FAIL → PASS |
| `smart-search.results-rendered` | results grid render condition made unreachable | FAIL → PASS |
| `smart-search.empty-state` | empty-state render condition inverted | FAIL → PASS |
| `smart-search.error-shown` | error banner gated on the loading state | FAIL → PASS |
| `smart-search.no-embeddings-hint` | no-embeddings banner condition inverted | FAIL → PASS |

Full-scope evidence run after restoring every mutation: `211 PASS, 0 FAIL, 0 SKIP, 0 ERROR`.

## Workflow follow-up

- Run the §7 validate stage on the final diff against `origin/main` and record the verdict.
- Sync the `smart-search` delta into `openspec/specs/smart-search/spec.md` with `openspec-sync-specs`, then run `openspec validate --specs`.
- Archive the change after validation and sync are complete.
