# Design

## Context

See `proposal.md` — Why. The current state that shapes this design:

- Each provider client in `backend/core/internal/infrastructure/llm/` builds `ModelInfo` its own
  way: `AlibabaClient.ListModels` prefers the native ModelStudio catalog and silently degrades to
  the OpenAI-compatible listing plus `inferAlibabaCapabilities()`; `OpenAIClient` only knows the
  model ID; `DeepSeekClient` has a rich `/models` path; `OllamaClient` reads `/api/show`.
- `handleGetLlmModels` (`internal/interfaces/handler/handlers_llm.go`) has two different answers
  for the same question: without `force` it returns rows from `llm_provider_models`, with
  `force` it returns freshly built `ModelInfo` values and then calls
  `ReplaceProviderModels`, discarding its error.
- `llm_model_capabilities` carries `UNIQUE (model_id, capability)`; the fallback capability
  inference appends `vision` twice for IDs matching both a vision token and an OCR token, so the
  replace transaction rolls back for the whole provider and the stored catalog freezes at the
  last good snapshot.
- The Analysis tab populates `modelCache[alias]` only through the "load models" button, which
  always forces a refresh; therefore the dropdown shows the degraded live list while the model
  details dialog shows the stored, rich one.

## Goals / Non-Goals

**Goals:**

- One code path that produces `ModelInfo` for every provider, with provenance ("how much do we
  know about this model") recorded so a degraded result can never overwrite a richer one.
- The stored catalog is the only thing the API returns; freshness is a property of the stored
  catalog, not of the request.
- Capability/modality normalization is total: no duplicates, no unknown values, no `vision` from
  an ID token accident.

**Non-Goals:**

- Changing how recognition resolves its client at request time.
- Introducing a background scheduler per model; the TTL is evaluated per provider on read.
- Reworking provider client HTTP layers, only the metadata assembly they feed.

## Decisions

### D1 — The pipeline lives in an application service, not in clients or handlers

New `internal/application/llm/catalog` service: `Get(ctx, providerAlias, force) (CatalogResult, error)`.
It resolves the provider, asks the infrastructure `llm.Client` for raw `ModelInfo`, enriches,
normalizes, merges and persists. Handlers only translate HTTP; provider clients stay transport
adapters.

*Alternatives:* enrich inside each client (keeps provider quirks local but duplicates the
priority logic five times and cannot implement registry fallback consistently); keep it in the
handler (untestable without HTTP, already the source of the divergence).

### D2 — Enrichment is a prioritized chain with explicit provenance

`enrichModel(info, providerKind)` applies, in order: native catalog fields → native per-model
detail fields → curated model-family registry → ID heuristics → safe default. Each step only
fills fields that are still unknown. Provenance is returned as a single level
(`rich | partial | heuristic`) for the whole refresh, plus per-field knowledge used by the merge.

*Alternatives:* keep per-provider enrichment (rejected, see D1); infer capabilities purely from
modalities (loses native codes such as `image_edit`, `world_model`).

### D3 — The curated registry is data, in the same package as the pricing table

Extend `metadata.go`'s `knownModelPrices` into a `modelFamilies` table: family token → required
capabilities, default modalities, context length, prices. Ordered longest-token-first so
`qwen-vl-ocr` beats `qwen`. It is consulted only when the provider did not describe the model.

*Alternatives:* scrape provider docs at runtime (fragile, no SLA); probe models with real
requests to discover modalities (cost, latency, side effects).

### D4 — Normalization is enforced before persistence

`normalizeModelInfo` canonicalizes capability codes through the existing vocabulary map,
deduplicates them, lower-cases and deduplicates modalities while keeping direction, and drops
unknown values. The `(model_id, capability)` unique index stays as a safety net rather than the
rule, and the repository insert uses `ON CONFLICT DO NOTHING` for capabilities/modalities so a
single odd row can never roll back a whole provider again.

*Alternatives:* rely on the DB constraint only (a single duplicate currently wipes the refresh,
as observed); drop the indexes (loses protection).

### D5 — Freshness is stored per provider

New column `llm_providers.models_fetched_at` (nullable timestamp) plus `models_source_rich`
boolean, written in the same transaction as the catalog rows. TTL comes from
`LLM_MODELS_CACHE_TTL` (Go duration, default `24h`).

*Alternatives:* separate `llm_model_catalog_state` table (extra join for one value); per-model
timestamps (overkill — metadata ages as a provider snapshot); in-memory cache (lost on restart,
diverges between replicas).

### D6 — Request semantics

`GET /api/llm/models?provider=<alias>[&refresh=true]`:

1. load stored catalog + `models_fetched_at`;
2. if `refresh` or the catalog is missing or older than TTL → collect and persist;
3. always answer from the stored catalog after the write.

Response gains `fetchedAt` (RFC3339) and `stale` (boolean, true when the last refresh failed)
and keeps `success`/`provider`; `refreshError` carries the failure text while `stale` is true.
A failed refresh with no stored catalog returns `success: false` with `error`.

### D7 — Merge, not replace

Persist by `model_id` upsert. A refresh with provenance `rich` may delete models absent from it;
`partial`/`heuristic` refreshes delete nothing and only overwrite fields they actually
determined. Deletions happen in the same transaction as upserts.

### D8 — Wire DI graph

`catalog.Service` is provided by Wire (depends on `llm.ClientFactory`-style factory,
`repository.LlmRepo`, config), injected into `Server`. `cmd/server/wire.go` changes and
`cmd/server/wire_gen.go` must be regenerated with `go generate ./...` and committed together.

### D9 — Frontend

`AdminAnalysisTab` loads each instrument's provider catalog on mount with a non-forcing request
and keeps the refresh button as the explicit forcing action; while `stale` is true it renders a
notice from new i18n keys (`llm_ocr.catalogStale` in `translations.en.ts` /
`translations.ru.ts`). `filterModelsForInstrument` keeps mapping OCR and VL to `vision`.

### D10 — Error handling and logging

The repository returns errors instead of swallowing them; the catalog service wraps and logs
them with `slog` (`provider`, `stage`), and the handler maps "refresh failed, catalog exists" to
`stale: true` and "no catalog at all" to an error response.

### D11 — Schema change

`AutoMigrate` adds the two provider columns. No data migration: the first read per provider
after deploy re-collects and merges, preserving every field the new source cannot determine.

### D12 — Evidence binding (focused-spec)

Bound scenario IDs live in the delta specs (`openspec/changes/llm-model-metadata-cache/specs/**`).
Registry entries under `.focused-spec/evidence.yaml` are implementation-phase work: at propose
time only native IDs are authored. Intended runner selectors:

| Scenario ID | Runner selector |
|---|---|
| `llm-model-metadata.capabilities-deduplicated` | `go::./internal/infrastructure/llm::TestAlibabaCapabilitiesDeduplicated` |
| `llm-model-metadata.image-input-implies-vision` | `go::./internal/infrastructure/llm::TestAlibabaListingDerivesVisionFromModalities` |
| `llm-model-metadata.chat-identifier-not-vision` | `go::./internal/infrastructure/llm::TestAlibabaChatIdentifierNotVision` |
| `llm-model-catalog.native-source-preferred` | `go::./internal/application/llm::TestCatalogPrefersRichSource` |
| `llm-model-catalog.registry-fills-partial-listing` | `go::./internal/application/llm::TestCatalogRegistryFillsListing` |
| `llm-model-catalog.unknown-model-safe-default` | `go::./internal/application/llm::TestCatalogUnknownModelDefault` |
| `llm-model-catalog.response-equals-stored` | `go::./internal/application/llm::TestModelsResponseEqualsStored` |
| `llm-model-catalog.response-exposes-freshness` | `go::./internal/application/llm::TestModelsResponseExposesFetchedAt` |
| `llm-model-catalog.fresh-catalog-not-refetched` | `go::./internal/application/llm::TestCatalogFreshNotRefetched` |
| `llm-model-catalog.expired-catalog-refetched` | `go::./internal/application/llm::TestCatalogExpiredRefetched` |
| `llm-model-catalog.explicit-refresh-ignores-ttl` | `go::./internal/application/llm::TestCatalogExplicitRefreshIgnoresTTL` |
| `llm-model-catalog.degraded-refresh-keeps-vision` | `go::./internal/application/llm::TestMergeKeepsVision` |
| `llm-model-catalog.degraded-refresh-keeps-context` | `go::./internal/application/llm::TestMergeKeepsContextLength` |
| `llm-model-catalog.degraded-refresh-keeps-models` | `go::./internal/application/llm::TestMergeKeepsMissingModels` |
| `llm-model-catalog.write-failure-reported` | `go::./internal/application/llm::TestRefreshWriteFailureReported` |
| `llm-model-catalog.write-failure-keeps-catalog` | `go::./internal/application/llm::TestRefreshWriteFailureKeepsCatalog` |
| `llm-settings.dropdown-populated-automatically` | `vitest::webapp/src/components/tabs/AdminAnalysisTab.test.tsx::loads models on mount without forcing a refresh` |
| `llm-settings.refresh-updates-dropdowns` | `vitest::webapp/src/components/tabs/AdminAnalysisTab.test.tsx::refresh action replaces the dropdown models` |
| `llm-settings.stale-notice-english` | `vitest::webapp/src/components/tabs/AdminAnalysisTab.test.tsx::renders the stale notice in English` |
| `llm-settings.stale-notice-russian` | `vitest::webapp/src/components/tabs/AdminAnalysisTab.test.tsx::renders the stale notice in Russian` |

## Risks / Trade-offs

- **Provider native catalog unavailable** (Alibaba `GET /api/v1/models` currently answers 404) →
  Mitigation: registry fallback covers the known families, degraded refreshes never overwrite
  known fields, degradation is logged and surfaced as `stale`.
- **Merge keeps outdated metadata indefinitely** → Mitigation: `rich` refreshes are allowed to
  delete, so a recovered catalog cleans up; the provider timestamp shows age.
- **Expired refresh blocks the request** → Mitigation: keep the existing bounded 30s listing
  client, per-provider single-flight so concurrent requests do not stampede the provider.
- **Registry becomes a maintenance burden** → Mitigation: registry only fills gaps; wrong or
  missing entries degrade to the safe default instead of breaking the dropdown filter.
- **`ON CONFLICT DO NOTHING` hides real bugs** → Mitigation: normalization is unit-tested and the
  catalog service logs how many rows were skipped.
- **OpenAPI change ripples to webapp types** → Mitigation: additive fields only; run
  `make generate-types` and `npx tsc -b` in the same task.

## Migration Plan

1. Ship core with the new column and TTL default; deploy.
2. First read per provider re-collects and merges, preserving unknown fields, so the corrupted
   state cannot get worse and recovers as soon as a rich source answers.
3. Verify `GET /api/llm/models?provider=<alias>` returns `fetchedAt` and matches the stored rows.
4. Rollback: the previous image ignores the new column; the table shapes stay compatible. If a
   rollback must repair metadata, re-run a refresh from the previous image (it replaces rows).

## Open Questions

- Which additional model families belong in the curated registry beyond the current production
  providers; this can be extended during apply without changing the specs.
