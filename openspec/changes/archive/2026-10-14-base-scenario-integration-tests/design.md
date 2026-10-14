# Design

## Context

See `proposal.md` — Why. The relevant current state:

- `tests/webapp/integration/` already boots the real `core` Go binary against the
  `flashbacks-test` PostgreSQL database and drives it over HTTP. Its Vitest runner
  config is [`webapp/vitest.integration.config.ts`](../../../../webapp/vitest.integration.config.ts:1)
  (`root` = repo root, `include` = `tests/webapp/integration/**/*.test.ts`,
  `environment: node`, `fileParallelism: false`, `hookTimeout: 180s`).
- Shared helpers live in [`tests/webapp/integration/support.ts`](../../../../tests/webapp/integration/support.ts:1):
  build/boot the service, reset bootstrap state, an HTTP `CoreClient`, cookie
  handling.
- focused-spec is installed at the repo root with the `vitest` runner registered
  in [`.focused-spec/config.yaml`](../../../../.focused-spec/config.yaml:1) bound to
  the **root** [`vitest.config.ts`](../../../../vitest.config.ts:1), whose `include`
  is `webapp/src/**/*.test.ts(x)` only. Integration tests are therefore not
  reachable through the existing `vitest` registration.
- The service contracts are in [`docs/api-contracts/core.yaml`](../../../../docs/api-contracts/core.yaml:1);
  the handlers return the camelCase DTOs in
  [`dto/media.go`](../../../../backend/core/internal/interfaces/dto/media.go:1).

## Goals / Non-Goals

**Goals:**

- Add HTTP integration tests for the base journeys in the new specs and bind each
  to a focused-spec ID with an exact, executed evidence selector.
- Keep the new tests in the existing integration harness so they reuse the boot,
  DB-reset, and auth helpers.

**Non-Goals:**

- Changing `core` behavior, DI graph, or API contracts.
- Modifying the existing `vitest` registration or the root `vitest.config.ts`
  unit-test `include` (integration tests must not join the default `npm test`
  run).
- Enrolling or re-specifying existing `trash` behavior (see proposal Non-goals).

## Decisions

### D1 — Reuse the existing integration harness and config

New suites live in `tests/webapp/integration/` and run under
`webapp/vitest.integration.config.ts`, exactly like the existing
`bootstrap-login`/`account-creation`/`gallery-folders` suites. Alternative
considered: a new harness — rejected, it would duplicate boot/DB/auth plumbing.

### D2 — One `vitest` runner, two projects in the root config

The official `@focused-spec/runner-vitest` package is reused (no custom plugin)
and the existing framework-only `vitest` registration in
[`.focused-spec/config.yaml`](../../../../.focused-spec/config.yaml:1) is kept
unchanged. The official runner executes the project's default root
[`vitest.config.ts`](../../../../vitest.config.ts:1), so that config now declares
two Vitest projects:

- `webapp-unit` — the jsdom component tests under `webapp/src` (behavior
  unchanged).
- `webapp-integration` — the suites under `tests/webapp/integration` in the `node`
  environment with the `pg` alias, mirroring
  [`webapp/vitest.integration.config.ts`](../../../../webapp/vitest.integration.config.ts:1).

The integration scenarios are enrolled through the existing runner with standard
selectors `vitest::tests/webapp/integration/<file>::<full test name>`, and the
pre-existing `vitest::webapp/src/...` evidence keeps resolving unchanged (verified
against `design-system.info-popup-reveal` and `llm-model-metadata.metadata-exposed`).

Alternatives considered and rejected:
- A second runner registration selecting the integration config via
  `options.commandArgs` — the official runner inserts `options.commandArgs`
  immediately after the resolved command (`node`), so `--config …` is rejected as
  a Node option (`node: bad option: --config`).
- A second registration overriding `options.command` to the Vitest binary — this
  replaces the runner's own collection harness and breaks discovery
  (`No test files found`).
- A project-local custom runner module under `.focused-spec/runners/` — unnecessary
  once the default config exposes both test sets.

### D3 — Deterministic fixtures and async-scan handling

- Each suite creates an isolated temporary gallery root (`mkdtemp`) and writes a
  known image (copied from `tests/ocr/dataset`) plus, where duplicates are needed,
  a byte-identical copy under the same root. Byte-identical content is what the
  content-hash grouping keys on.
- `POST /api/scan` is asynchronous (HTTP 202) and reports progress through
  `GET /api/status` (`scanning`, `filesProcessed`). Suites poll `/api/status`
  until `scanning` is false (bounded) before asserting on gallery/duplicates.
- Trash-dependent scenarios set the trash directory with
  `PUT /api/settings { "trashDir": <temp> }` before deleting, and clean up in
  `afterAll` (restore settings, remove temp dirs, reset bootstrap state).
- The database is shared across suites and files do not run in parallel
  (`fileParallelism: false`); each suite clears gallery folders and legacy trash
  rows it created to keep assertions exact.

### D4 — One scenario, one test

Each focused-spec scenario maps to exactly one `it(...)` whose assertion is the
single observable outcome (response status + the specific JSON fields the
scenario names). No test asserts two independent outcomes.

### D5 — Evidence registry placement

New `(current, <id>)` entries go into the existing single shard
[`.focused-spec/evidence.yaml`](../../../../.focused-spec/evidence.yaml:1) under
`scopes.current`, matching the project's single-file registry ownership. Entries
are added as `planned:` when implementation starts and converted to concrete
selectors before the change is declared complete.

## Risks / Trade-offs

- **Runner cannot target the integration config via `commandArgs`** → mitigated by
  the mandatory resolution check in D2 and the documented custom-runner fallback.
- **Async scan timing flakiness** → poll `/api/status` with a bounded timeout
  instead of fixed sleeps; fail with the captured service logs on timeout.
- **Shared test database / leftover rows** (gallery folders, trash items, image
  files) → deterministic cleanup in `beforeAll`/`afterAll`; suites run serially.
- **Environment availability** (Go toolchain + reachable `flashbacks-test`) → the
  suite already requires these; if unavailable, evidence cannot be executed and
  must be reported as unverified rather than claimed green.
- **Duplicate hashing cost** → fixtures are tiny (single small JPEG copies), so
  scans are fast.

## Migration Plan

1. Land the tests and registry entries on the feature branch.
2. Convert `planned:` evidence to concrete selectors, run `validate --strict` and
   `run`, and mutation-check each new reference.
3. Sync delta specs into main specs, then archive the change (separate stage).
   Rollback is reverting the branch; no production code or schema changes.
