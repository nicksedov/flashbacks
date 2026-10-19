# Design

## Context

See [`proposal.md`](proposal.md) for motivation and
[`specs/service-registry/spec.md`](specs/service-registry/spec.md) for the
behavior contract.

Current constraints that shape the approach:

- Three Go services (`core` 5170, `exif` 5172, `ocr` 5174) read static env vars
  with hard-coded defaults ([`config.go`](../../../backend/core/internal/infrastructure/config/config.go:61)).
- Inter-service calls are static: `core` builds its EXIF/OCR clients from
  `EXIF_SERVICE_URL` / `OCR_SERVICE_URL` at startup
  ([`exifclient/client.go`](../../../backend/core/internal/infrastructure/exifclient/client.go:25),
  [`ocr/client.go`](../../../backend/core/internal/infrastructure/ocr/client.go:58)).
- Dev runs host-native via [`scripts/dev.sh`](../../../scripts/dev.sh:36) with a
  fixed service list; there is no orchestrator to sequence boot.
- `core` uses Google Wire; any DI change requires `go generate ./...`.
- Go rules: clean architecture, explicit errors, `slog`, unit tests, `go vet`.
- focused-spec is installed project-locally (`focused-spec@0.9.1`) with format
  `@focused-spec/format-markdown` and a `go` runner currently scoped to
  `backend/core`.

## Goals / Non-Goals

**Goals:**

- One process (`backend/registry`) owns an embedded, single-node etcd instance
  and is the only component that speaks etcd's protocol.
- A thin REST facade gives consumers configuration, discovery, and secrets over
  the transport they already use.
- Services register with a TTL lease, discover peers by prefix watch, and reload
  configuration without restarts.
- Secrets are unreadable in etcd's data directory (app-level AES-256-GCM).
- The stack remains runnable locally with a small bootstrap env surface.

**Non-Goals:**

- Multi-node etcd quorum, raft tuning, backups, and disaster recovery.
- A management UI or per-key RBAC beyond per-service etcd users.
- Automated master-key rotation.
- Migrating Postgres `app_settings` into etcd.
- Any change to scan/OCR/EXIF/gallery business behavior.

## Decisions

### 1. Embedded etcd in a new `backend/registry` module

- Module `github.com/flashbacks/registry`, layout `cmd/server`,
  `internal/{config,registry,httpapi,secrets}`, exported `client/`.
- etcd is embedded with `go.etcd.io/etcd/server/v3/embed`; client `:2379`,
  peer `:2380`; data dir `REGISTRY_DATA_DIR` (dev default `.run/registry-data`).
- Consumers add `replace github.com/flashbacks/registry => ../registry`,
  mirroring the existing `shared` replace pattern.
- **Alternative considered:** external Consul/NATS. Rejected because etcd
  provides KV + watch + lease natively, is Go-native and embeddable, and keeps
  the dev stack free of an extra external process. Consul would need a separate
  agent; NATS discovery is subject-based rather than a registry.

### 2. REST facade instead of direct etcd clients in consumers

- The registry exposes `/registry/v1/{config,secrets,services,health}`.
- Rationale: centralizes etcd auth and secret encryption in one place, keeps
  consumers on HTTP/REST, avoids shipping the etcd client stack into every
  service, and gives one auditable write path.
- **Trade-off:** one extra hop for low-QPS config/registration traffic; the
  client caches reads and subscribes to watches.

### 3. Key namespace

```
/flashbacks/config/<service>/<key>        # plaintext non-secret config
/flashbacks/secrets/<service>/<key>       # AES-256-GCM ciphertext
/flashbacks/services/<service>/<instance> # JSON registration, lease-attached
```

`<instance>` is a stable instance ID (`SERVICE_ID`, defaulting to hostname+pid).
Leases attach only to `services/*`, so discovery entries disappear on TTL expiry.

### 4. Registration and discovery

- Each service registers `{name, host, port, basePath, healthPath, version}`
  with TTL `15s` and a keepalive goroutine; it deregisters on graceful shutdown.
- Consumers discover peers via `Discover(name)` and keep a watch that updates a
  cached instance list.
- `core` resolves `exif` and `ocr` through discovery. The HTTP clients gain a
  `BaseURLProvider func() string` (backed by the discovery cache) so a changed
  registration is picked up without rebuilding the client.

### 5. Configuration layering and reload

- Bootstrap env is the only required environment surface:
  `REGISTRY_ENDPOINTS`, `REGISTRY_USER`, `REGISTRY_PASSWORD`, `SERVICE_NAME`,
  `SERVICE_ID`, and optional peer bootstrap URLs.
- Precedence: explicit env override (debug only) > etcd value > built-in
  default. `client.GetConfig(service)` merges these; `WatchConfig` swaps an
  atomic snapshot and invokes a callback.
- Reload callbacks apply only fields that are safe to change at runtime; values
  that require reconstruction (e.g. DB pool) are documented as restart-required.

### 6. Secrets encryption at rest

- etcd has no native value encryption, so the registry encrypts secret values
  with AES-256-GCM before `Put` and decrypts only in the secrets read handler.
- Master key from `REGISTRY_MASTER_KEY` (32 bytes, base64). Ciphertext layout:
  `nonce || ciphertext` with a key version prefix for future rotation.
- etcd auth is enabled (root + per-service users) and TLS is supported; the dev
  profile may disable TLS behind an explicit flag.
- Secret values are never logged and never returned by config endpoints.

### 7. Consumer integration and DI

- `core`: add a registry client provider in
  [`providers.go`](../../../backend/core/internal/infrastructure/di/providers.go:35),
  derive EXIF/OCR base URLs from discovery, load config at boot, and regenerate
  `wire_gen.go` with `go generate ./...`.
- `exif`/`ocr`: load config, self-register, and read the DB password from the
  secrets API.
- All services keep an env-fallback path so a registry outage degrades rather
  than blocks startup.

### 8. Dev tooling and deployment

- [`scripts/dev.sh`](../../../scripts/dev.sh:36): add `registry` to the service
  list, start it first, and wait for its health path before dependents.
- [`Makefile`](../../../Makefile:49): add `run-registry`, `dev-registry`,
  `build-registry`, `test-registry`, `lint-registry`.
- [`backend/Dockerfile`](../../../backend/Dockerfile:15): add a `registry`
  target and runtime stage; copy `registry/` in the builder for the `replace`.
- [`.env.example`](../../../.env.example:1): add the `REGISTRY_*` bootstrap vars.

### 9. focused-spec evidence plan (implementation phase)

Native IDs are already attached to each scenario in the delta spec. The evidence
registry, runner configuration, and tests are deferred to apply, as required for
a planning phase. Intended bindings:

| Scenario ID | Runner | Intended selector |
|---|---|---|
| `service-registry.config-roundtrip` | `go` | registry HTTP handler test |
| `service-registry.config-watch` | `go` | registry watch handler test |
| `service-registry.register-discovery` | `go` | registry discovery test |
| `service-registry.lease-expiry` | `go` | embedded-etcd lease expiry test |
| `service-registry.discovery-resolves-peer` | `go` | client resolver test (core) |
| `service-registry.bootstrap-fallback` | `go` | client fallback test |
| `service-registry.bootstrap-only-env` | `go` | bootstrap config test |
| `service-registry.secret-encrypted-at-rest` | `go` | secrets codec/store test |
| `service-registry.secret-roundtrip` | `go` | secrets handler test |
| `service-registry.health-ready` | `go` | health handler test |

The existing `go` runner hard-codes `commandArgs: ["-C", "backend/core"]`, so
apply must add a registry-scoped `go` runner registration (separate ID, e.g.
`goregistry`, with `-C backend/registry`) and use it for registry-module tests,
while keeping `go` for core tests.

## Risks / Trade-offs

- **Dependency weight** (etcd embed) → isolated in the registry module; consumers
  depend only on the thin REST client package.
- **Single-node etcd is a SPOF** → cluster-ready config, bootstrap fallback, and
  documented recovery; HA is a follow-up.
- **Secret key management** → master key from env, never persisted in etcd;
  rotation is documented and deferred.
- **Stale peer URLs during rollout** → discovery watch + resolver refresh, with
  client retry/fallback; `core` still accepts bootstrap URLs.
- **Runtime reload safety** → only safe fields reload live; others require a
  restart, documented per key.
- **Boot ordering in dev** → `dev.sh` waits for registry health before starting
  dependents.
- **Config drift during migration** → phased rollout A–D with env fallback kept
  until Phase D.
- **focused-spec runner scope** → add a registry-scoped `go` runner in apply;
  do not repoint the existing core runner.

## Migration Plan

1. **Phase A — foundation**: registry service, facade, secrets codec, client
   library, dev tooling, Dockerfile target, docs. No consumer behavior change.
2. **Phase B — exif/ocr**: pull config from the registry, self-register, read
   secrets; keep env fallback.
3. **Phase C — core**: config from registry, discovery-based EXIF/OCR URLs,
   secrets, Wire regeneration.
4. **Phase D — cleanup**: remove static URL defaults, finalize contracts/docs.

Rollback: every service keeps env bootstrap/fallback, so reverting a consumer
commit restores static configuration without data loss. The registry container
can be stopped without taking down the stack while fallbacks remain.

## Open Questions

- Rotation procedure for `REGISTRY_MASTER_KEY` (dual-key read window) — defer.
- Whether to eventually migrate Postgres `app_settings` into etcd — defer.
- TLS certificate provisioning for staging/production — defer to deployment.
