# Tasks

## 1. Registry module foundation

- [x] 1.1 Scaffold the `backend/registry` Go module (`go.mod` with `module github.com/flashbacks/registry`, `cmd/server`, `internal/{config,registry,httpapi,secrets}`, exported `client/`) and verify `cd backend/registry && go build ./...` succeeds.
- [x] 1.2 Embed a single-node etcd server via `go.etcd.io/etcd/server/v3/embed` (client `:2379`, peer `:2380`, data dir from `REGISTRY_DATA_DIR`) with graceful shutdown, and verify it starts, reports healthy, and stops cleanly in a Go test.
- [x] 1.3 Add bootstrap config parsing (`REGISTRY_DATA_DIR`, client/peer URLs, auth, TLS flag, `REGISTRY_MASTER_KEY`) and verify `cd backend/registry && go test ./internal/config/... -count=1` passes.

## 2. Registry facade and secrets

- [x] 2.1 Implement config CRUD and list handlers under `/registry/v1/config/{service}/{key}` and verify handler tests cover write, read, list, and delete.
- [x] 2.2 Implement the config prefix watch endpoint and verify a handler test observes an update delivered to a watcher.
- [x] 2.3 Implement the AES-256-GCM secrets codec with a key-version prefix and verify a round-trip test passes and a ciphertext-is-not-plaintext test passes.
- [x] 2.4 Implement secrets PUT/GET handlers backed by the codec and verify handler tests cover write, encrypted persistence, and plaintext read.
- [x] 2.5 Implement service register/deregister/list/watch on lease-attached keys and verify embedded-etcd tests cover registration visibility and removal after TTL expiry.
- [x] 2.6 Implement the health endpoint reporting embedded-backend readiness and verify a handler test returns success once the backend is initialized.

## 3. Client library

- [x] 3.1 Implement the `client` package transport with config get/watch and secrets get/put calls and verify unit tests pass with an `httptest` facade.
- [x] 3.2 Implement registration with keepalive, deregistration, discovery, and instance watch in the client and verify unit tests cover register, discover, and watch delivery.
- [x] 3.3 Implement configuration layering (explicit env override > registry value > built-in default) and the registry-unavailable bootstrap peer fallback, and verify unit tests cover both paths.

## 4. Dev tooling and container

- [x] 4.1 Add `registry` to [`scripts/dev.sh`](../../../scripts/dev.sh:36) with start-first ordering and a health wait before dependents, and verify `scripts/dev.sh build registry` and `scripts/dev.sh status` report it.
- [x] 4.2 Add `build-registry`, `run-registry`, `dev-registry`, `test-registry`, and `lint-registry` targets to [`Makefile`](../../../Makefile:49) and verify each target runs.
- [x] 4.3 Add a `registry` build target and runtime stage to [`backend/Dockerfile`](../../../backend/Dockerfile:15) and verify the documented `docker build --target=registry` invocation succeeds.
- [x] 4.4 Add the `REGISTRY_*` bootstrap variables to [`.env.example`](../../../.env.example:1) and verify the documented defaults match the code in `backend/registry/internal/config`.

## 5. Consumer migration

- [x] 5.1 Migrate `exif` to load config from the registry, self-register with keepalive, read its DB password from the secrets API, and keep the env bootstrap/fallback path, then verify `cd backend/exif && go test ./... -count=1 && go vet ./...` passes.
- [x] 5.2 Migrate `ocr` to load config from the registry and self-register, keeping the env bootstrap/fallback path, then verify `cd backend/ocr && go test ./... -count=1 && go vet ./...` passes.
- [x] 5.3 Migrate `core` to load config from the registry, resolve `exif`/`ocr` base URLs through discovery (with a base-URL provider on the HTTP clients), read secrets, add the registry client to [`providers.go`](../../../backend/core/internal/infrastructure/di/providers.go:35), and verify `cd backend/core && go generate ./... && git diff --exit-code cmd/server/wire_gen.go && go test ./internal/application/... -count=1 && go vet ./... && go build ./...` passes.

## 6. Contracts, docs, and evidence registration

- [x] 6.1 Add `docs/api-contracts/registry.yaml` covering the facade endpoints and verify it parses as valid OpenAPI 3 YAML.
- [x] 6.2 Update [`docs/architecture.md`](../../../docs/architecture.md:5), [`README.md`](../../../README.md:5), and [`docs/agent/go-services.md`](../../../docs/agent/go-services.md:11) with the registry service, ports, start order, and bootstrap-only env rule, and verify the registry appears in each document.
- [x] 6.3 Add a registry-scoped `go` runner (e.g. `goregistry` with `-C backend/registry`) to [`.focused-spec/config.yaml`](../../../.focused-spec/config.yaml:1), bind all ten `service-registry.*` scenario IDs in [`.focused-spec/evidence.yaml`](../../../.focused-spec/evidence.yaml:1) with concrete selectors, and verify `node_modules/.bin/focused-spec validate --scope current` passes.

## 7. Integration verification

- [x] 7.1 Start the full stack (`registry` + `core` + `exif` + `ocr`) via `scripts/dev.sh start all` and verify `core` discovers `exif`/`ocr` without static service URLs. (Verified: all three instances registered with correct ports and health returned 200. A gallery scan was not exercised because this environment has no configured gallery folders; discovery is additionally covered by the core resolver test.)
- [x] 7.2 Run `node_modules/.bin/focused-spec run --scope current` and verify every `service-registry.*` scenario passes. (All ten scenarios ran with `--scenario` and passed.)
- [x] 7.3 For each new evidence binding, run the mutation check (revert the behavior, observe failure, restore, observe pass) and record the fail → restore → pass sequence. (Eight controlled regressions covered all ten scenarios; all failed on mutation and passed after restore.)

## Workflow follow-up

- Sync the delta spec into `openspec/specs/service-registry/spec.md` with the `openspec-sync-specs` workflow.
- Archive the change after review requirements are satisfied.
