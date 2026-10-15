# Go Services — Component Requirements

Component requirements for every Go module in this repository. Read this document
**before changing any `.go` file**, `go.mod`, `wire.go`/`wire_gen.go`, locale
JSON, or the OpenAPI/MCP contracts those services expose.

The project-wide rules (English-only, clean architecture, i18n en/ru parity, task
mode) live in [`AGENTS.md`](../../AGENTS.md:1). Rules here are component-specific
and win over the root file whenever the two differ.

| Component | Path | Role | Port | README |
|---|---|---|---|---|
| core | `backend/core/` | Core REST API: auth, scan, gallery, OCR orchestration, tags, chat, trash | 5170 | [`backend/core/README.md`](../../backend/core/README.md:1) |
| exif | `backend/exif/` | EXIF read/write, GPS writing, MCP endpoint | 5172 | [`backend/exif/README.md`](../../backend/exif/README.md:1) |
| ocr | `backend/ocr/` | Text detection / document classification via Tesseract | 5174 | [`backend/ocr/README.md`](../../backend/ocr/README.md:1) |
| shared | `backend/shared/` | Shared domain types (`exif.go`, `ocr.go`) consumed by the services | — | — |
| embeddings-builder | `tools/embeddings-builder/` | CLI that backfills `tag_embeddings` via pgvector | — | [`tools/embeddings-builder/README.md`](../../tools/embeddings-builder/README.md:1) |

This document is the **only** rule source for these components: they no longer
ship a local `AGENTS.md`. The READMEs above document setup and endpoints for
humans.

---

## 1. Universal rules (every Go module)

### Language & style

- PascalCase exported, camelCase unexported; **no identifier redeclaration in the
  same scope**.
- Explicit error handling, **no `panic`**; wrap with `fmt.Errorf("...: %w", err)`.
- `gofmt` formatting; `go vet ./...` must be clean.
- Structured JSON logging with `slog`.
- Clean architecture layering: `domain` → `application` → `infrastructure` →
  `interfaces`. Dependencies point inward; handlers never reach into the database.
- Exported/public functions carry a godoc comment.
- Do not add a dependency without justifying it (PR description).

### i18n (core and any service with user-facing messages)

- Use `Msg*` constants — never a raw string as a message key.
- Convert with `string(i18n.MsgX)`; never skip the `MessageKey` → `string` step.
- Never mix `Msg*` and `Err*` naming.
- Validation errors via `i18n.CreateValidationError(i18n.ValidationError)`.
- Add the `Msg*` constant **and** both `en`/`ru` locale entries in the same change
  (`backend/core/internal/interfaces/i18n/locales/{en,ru}.json`).
- OCR user documentation is Russian-first; **code and identifiers stay English**.

### Testing

- New code ships with unit tests; run them after *every* change and fix failures
  before continuing (zero failures required).
- Run the relevant test command (see §2) plus `go build ./...` as a compile check.
- Unit tests run on **in-memory SQLite** — never assume Postgres-only behavior.
- Prefer table-driven tests (`[]struct{ name, input, want }`).
- Refactor only where tests already cover the changed code; never refactor the
  OCR `/classify` signature without an explicit requirement.

### Cross-service contracts

- JSON tags are camelCase and must match the webapp TypeScript property names
  exactly — changing a tag is a frontend-breaking contract change.
- REST is the only inter-service channel (core → exif/ocr); **no cross-service DB
  writes**.
- OpenAPI: [`docs/api-contracts/`](../api-contracts/) (`core.yaml`, `exif.yaml`,
  `ocr.yaml`); MCP tool schemas: [`docs/mcp-contracts/`](../mcp-contracts/).
- When `docs/api-contracts/core.yaml` changes, run `make generate-types` to
  refresh `webapp/src/types/api.ts`.

---

## 2. Checks per component

| Component | Tests | Lint / vet | Build |
|---|---|---|---|
| core | `cd backend/core && go test ./internal/application/... -count=1` | `cd backend/core && go vet ./...` | `go build ./...` |
| exif | `cd backend/exif && go test ./... -count=1` | `cd backend/exif && go vet ./...` | `go build ./...` |
| ocr | `. ./scripts/toolenv.sh && cd backend/ocr && go test -race -cover ./...` | `. ./scripts/toolenv.sh && cd backend/ocr && golangci-lint run` | `. ./scripts/toolenv.sh && go build -o ocr ./cmd/server` |
| shared | `go build ./...` in every consumer module | `go vet ./...` where consumed | — |
| embeddings-builder | `cd tools/embeddings-builder && go test ./... -count=1` | `go vet ./...` | `go build -o embeddings-builder ./cmd/embeddings-builder/` |

Make targets wrap the same commands: `make test-core`, `make test-exif`,
`make test-ocr`, `make lint-core`, `make lint-exif`, `make lint-ocr`,
`make build-core`, `make build-exif`, `make build-ocr`.

---

## 3. core — `backend/core/`

- **Stack**: Go 1.25, Gin, GORM (Postgres + SQLite for tests), Google Wire,
  pgvector, MCP go-sdk.
- **DI**: Wire. When the DI graph changes, run `make generate-go`
  (`go generate ./...`) and commit **both** `cmd/server/wire.go` and the
  regenerated `cmd/server/wire_gen.go`.
- **Layout**: `internal/application/{agent,auth,background,geo,imaging,thumbnail}`,
  `internal/domain` (+ `repository` interfaces), `internal/infrastructure/*`,
  `internal/interfaces/{dto,handler,i18n,middleware}`, mocks and fixtures in
  `internal/testutil`.
- **Schema**: GORM auto-migration at startup — model changes migrate on boot.
- **Security**: session auth (bcrypt), CSRF on mutations, login rate limiting,
  per-environment CORS, encrypted API keys.
- **Errors i18n**: `internal/interfaces/i18n/` with `en.json` + `ru.json`.
- **Endpoints**: `/api/auth/*`, `/api/scan`, `/api/gallery`, `/api/ocr/*`,
  `/api/chat/*`, `/api/media/*`, `/api/trash*`, MCP tools under
  `internal/infrastructure/mcpserver/`.
- **Required checks**: `go test ./internal/application/... -count=1` and
  `go vet ./...`; add `make generate-types` when an OpenAPI response changed.

## 4. exif — `backend/exif/`

- **Stack**: Go 1.25, Gin, GORM, ExifTool (must be on `PATH` or in `.tools/`),
  MCP go-sdk.
- **Layout**: `cmd/server`, `internal/{application,domain,infrastructure,interfaces}`,
  MCP server in `mcp/`.
- **Routes** (prefixed `/exif`): `/health`, `/metadata?path=`, `/missing`,
  `PUT /gps`, `PUT /gps/batch`, `/location-candidates`, `ANY /mcp`.
- ExifTool is invoked through a pooled runner — do not spawn it per request
  outside the pool.
- MCP tool definitions must stay aligned with
  [`docs/mcp-contracts/exif-mcp-tools.yaml`](../mcp-contracts/exif-mcp-tools.yaml).
- **Required checks**: `go test ./... -count=1`, `go vet ./...`.

## 5. ocr — `backend/ocr/`

- **Stack**: Go 1.25, gosseract (Tesseract via CGO), bild, imaging.
- **Toolchain**: Tesseract is bootstrapped project-locally with
  `make tools-tesseract` and exposed by `scripts/toolenv.sh`, which exports
  `PATH`, `LD_LIBRARY_PATH`, `CGO_*FLAGS`, `TESSDATA_PREFIX`. **Source it before
  running any `go` command by hand** (`. ../../scripts/toolenv.sh`); `make`
  targets already do it.
- **HTTP surface**: `POST /classify` (text detection / document verdict) and
  `GET /health`.
- **Frozen surface**: do not change the `/classify` signature without an explicit
  requirement; no breaking changes to the public API.
- **Tests**: table-driven style; the dataset is shared and lives in
  [`tests/dataset/`](../../tests/dataset/README.md) (`eng`, `image`, `inscription`,
  `rus`); keep classification accuracy from regressing.
- **Performance targets**: p95 latency for `/classify` < 5000 ms, memory < 100 MB
  per request.
- **Coverage**: `go test -race -coverprofile=coverage.out ./...`; coverage must
  not drop.
- **Required checks**: `go build -o ocr ./cmd/server`, `go vet ./...`,
  `go test -race -cover ./...`, `golangci-lint run` (config `.golangci.yml`).
  Integration suite: `make test-ocr-integration` (pytest, requires a running
  service).

## 6. shared — `backend/shared/`

- Holds domain types shared between services (`domain/exif.go`, `domain/ocr.go`).
- A change here is a **cross-service contract change**: update this module, then
  build and test *every* consumer (`core`, `exif`, `ocr`) and refresh any OpenAPI/
  MCP schema that mirrors the type.
- No service-specific logic and no dependencies on Gin/GORM belong here.

## 7. embeddings-builder — `tools/embeddings-builder/`

- Standalone CLI: `cmd/embeddings-builder/{main,db,embedding,models}.go`.
- Idempotent by design: tracks tag content changes via MD5 hashes and skips
  unchanged images; `--force` recomputes, `--dry-run` counts, `--batch-size`
  tunes batching (default 100).
- Reads `llm_settings` / `llm_providers` from the core database and creates
  per-model child tables (`tag_embeddings_<model>`); it does **not** own the core
  schema — coordinate model changes with core.
- **Required checks**: `go test ./... -count=1`, `go vet ./...`.

---

## 8. Definition of done for a Go change

- [ ] `gofmt` clean, `go vet ./...` clean, no identifier redeclaration.
- [ ] Errors handled explicitly; no `panic`; wrapped with `%w`.
- [ ] Clean-architecture layering respected; Wire graph regenerated and both
      `wire.go` + `wire_gen.go` committed when DI changed.
- [ ] New code covered by unit tests; component test command green (zero failures).
- [ ] i18n `Msg*` constant **and** both `en`/`ru` locale entries added together.
- [ ] JSON tags camelCase and matching the webapp TS types; OpenAPI/MCP contracts
      updated and `make generate-types` run when a contract changed.
- [ ] Component-specific checks from §2 executed and reported.

## 9. See also

- [`docs/agent/webapp.md`](webapp.md) — the TypeScript/React counterpart.
- [`docs/agent/sdd-workflow.md`](sdd-workflow.md) — SDD cycle and evidence gate.
- [`docs/architecture.md`](../architecture.md) — service topology and ports.
- [`backend/core/README.md`](../../backend/core/README.md:1),
  [`backend/exif/README.md`](../../backend/exif/README.md:1),
  [`backend/ocr/README.md`](../../backend/ocr/README.md:1),
  [`tools/embeddings-builder/README.md`](../../tools/embeddings-builder/README.md:1)
  — service setup and endpoints.
