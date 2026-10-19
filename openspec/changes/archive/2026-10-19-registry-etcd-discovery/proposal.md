# Proposal

## Why

Every Go service currently reads static environment variables with hard-coded
defaults, and inter-service URLs (`EXIF_SERVICE_URL`, `OCR_SERVICE_URL`) are
fixed at boot — there is no single source of truth, no discovery, and no live
reload. As the number of services grows this causes configuration drift and
forces restarts for every change.

## What Changes

- Add a new Go service `backend/registry` that embeds single-node **etcd**
  (client port `2379`, peer port `2380`; cluster-ready later) and becomes the
  single source of truth for configuration and service locations.
- Expose an HTTP REST facade for the registry so consumers stay on REST:
  config CRUD + watch, service register/deregister/discover/watch, and health.
- Introduce self-registration: each Go service registers itself under a TTL
  lease with a keepalive and is discovered by peers via prefix watch.
- Move application configuration into etcd under `/flashbacks/config/<service>/*`
  and watch it for live reload. Bootstrap stays in env (registry endpoint,
  credentials, service identity); env may still override for local debugging.
- Store secrets under `/flashbacks/secrets/<service>/*` encrypted at rest with
  AES-256-GCM (etcd has no native value encryption); enable etcd auth (RBAC)
  and TLS.
- Ship a shared Go client library `github.com/flashbacks/registry/client`
  (register/keepalive, discover/watch, config get/watch, secrets get/put) with
  retry and env-fallback degradation.
- Migrate `core`, `exif`, and `ocr` to load config from the registry and resolve
  peers via discovery; `core` stops using `EXIF_SERVICE_URL` / `OCR_SERVICE_URL`.
- Update dev tooling (`scripts/dev.sh`, `Makefile`, `.env.example`,
  `backend/Dockerfile`) and documentation (`docs/architecture.md`, `README.md`,
  `docs/agent/go-services.md`), and add `docs/api-contracts/registry.yaml`.
- **BREAKING**: application-level configuration is no longer supplied by env
  vars; only bootstrap variables remain, so existing `.env`-only deployments
  must provision the registry and populate its KV store.

## Capabilities

### New Capabilities

- `service-registry`: central service registration and discovery, a KV
  configuration store as the single source of truth, and encrypted secret
  storage served by the embedded-etcd registry service.

### Modified Capabilities

None — no existing capability's requirements change; this adds a new
infrastructure capability.

## Non-goals

- Multi-node etcd clustering, quorum, and disaster recovery are out of scope for
  this change; single-node is cluster-ready by configuration only.
- A web UI for editing registry values is out of scope; management is via the
  REST facade and CLI/tests.
- Automatic secret rotation/key re-encryption is deferred; the master-key
  rotation procedure is documented only.
- No changes to business behavior of scanning, OCR, EXIF, or gallery features.
- Not migrating Postgres `app_settings` values into etcd in this change; the
  registry becomes the source for service/config parameters, and `app_settings`
  stays as user-level runtime settings.

## Impact

- **Code**: new `backend/registry` Go module; config + DI + HTTP clients in
  `backend/core`, `backend/exif`, `backend/ocr`; `scripts/dev.sh`; `Makefile`;
  `backend/Dockerfile`; `.env.example`.
- **APIs**: new OpenAPI contract `docs/api-contracts/registry.yaml`; no changes
  to existing `core`, `exif`, or `ocr` OpenAPI surfaces.
- **Dependencies**: `go.etcd.io/etcd/server/v3` (embed) and
  `go.etcd.io/etcd/client/v3` in the new module; consumers add a `replace`
  directive for the registry client module.
- **Systems**: dev stack gains a start-order dependency (registry first) and a
  new runtime service/container; deployment must provision etcd storage, auth,
  TLS, and the master key.
- **Docs**: `docs/architecture.md`, `README.md`, `docs/agent/go-services.md`.
