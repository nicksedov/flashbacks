# Design

## Context

See `proposal.md` for motivation. The current normalization lives in
`backend/core/internal/infrastructure/llm/metadata.go`:

- `normalizeCapabilities([]string)` maps provider-specific names into a small
  standardized set (`chat`, `tool_calling`, `vision`, `embedding`, `image_edit`);
  every Alibaba code other than `TG`/`VU`/`IG` is silently dropped.
- `capabilitiesFromModalities([]string)` unconditionally seeds
  `chat` + `tool_calling` and adds `vision` when any modality is `image`.
- `alibabaCatalogModelToInfo` builds capabilities as
  `capabilitiesFromModalities(modalities) ∪ normalizeCapabilities(rawCodes)`, so
  every Alibaba model inherits `chat`/`tool_calling` from its modalities.

Constraints: the webapp consumes the `capabilities` string array directly
(`webapp/src/lib/llm-model-filter.ts`, `ModelDetailsDialog.tsx`,
`AdminLlmProvidersTab.tsx`); `capabilities` in the OpenAPI contract is an
untyped string array, so expanding values needs no schema change.

## Goals / Non-Goals

**Goals:**

- A single explicit mapping from every Alibaba native capability code to the
  standardized vocabulary.
- Capability derivation that never advertises `chat`/`tool_calling` for a model
  whose output is not text.
- The new standardized values are displayed to admins with en/ru labels.

**Non-Goals:**

- No change to `inferCapabilitiesByID` heuristics for Ollama/OpenAI/DeepSeek
  beyond consistency.
- No new instrument types, and no change to the `llm-model-filter.ts`
  instrument→capability values; only the unmatched-mapping branch is hardened to
  fail closed (see D5).
- No refactor of the provider DTO/shape or the REST surface.

## Decisions

### D1 — Standardized capability vocabulary

Extend the vocabulary to twelve values. The five instrument-facing values keep
their meaning; the seven new values are metadata only:

`chat`, `tool_calling`, `vision`, `embedding`, `image_edit`,
`reasoning`, `video_generation`, `text_to_speech`, `speech_recognition`,
`realtime`, `omni`, `world_model`.

### D2 — Native-code mapping table

`normalizeCapabilities` (case-insensitive, trimmed) maps:

| Standardized | Native codes (lower-cased) |
|---|---|
| `chat` | `completion`, `chat`, `tg`, `text`, `realtime-chatting` |
| `tool_calling` | `tools`, `tool_calling`, `function-calling`, `functions` |
| `vision` | `vision`, `visual`, `vu`, `image` |
| `embedding` | `embedding`, `embeddings` |
| `image_edit` | `image_edit`, `image-edit`, `image_generation`, `image-generation`, `ig` |
| `reasoning` | `reasoning` |
| `video_generation` | `vg`, `video_generation`, `video-generation`, `world-model` |
| `world_model` | `world-model`, `world_model` |
| `text_to_speech` | `tts`, `text_to_speech`, `text-to-speech`, `realtime-text-to-speech` |
| `speech_recognition` | `asr`, `speech_recognition`, `speech-recognition`, `realtime-asr` |
| `realtime` | any code with the `realtime-` prefix |
| `omni` | `omni`, `multimodal-omni`, `realtime-omni` |

A single native code may emit more than one standardized value (`Realtime-ASR`
→ `realtime` + `speech_recognition`; `Multimodal-Omni` → `omni` + `chat`;
`World-Model` → `world_model` + `video_generation`). Implementation: replace the
flat `switch` with a per-code mapping that appends all matching values, plus a
`strings.HasPrefix(code, "realtime-")` rule.

*Alternatives considered:* a denylist that removes `chat` from known non-text
codes — rejected as fragile when new codes appear. Prefix matching for the
`realtime-*` family avoids enumerating each variant.

### D3 — Derive capabilities from native codes, not modalities (false-positive fix)

In `alibabaCatalogModelToInfo`:

1. `caps = normalizeCapabilities(rawCodes)`.
2. If `rawCodes` yields nothing, fall back to
   `capabilitiesFromModalities(modalities)`.
3. Add `vision` only when the request modality includes `image` **and** the
   response modality includes `text` (an understanding model, not a generator).

This stops seeding `chat`/`tool_calling` from modalities, so video, speech, and
image generation models no longer appear as chat. The existing
`capabilitiesFromModalities` keeps its current behavior for its only remaining
caller (DeepSeek text models), which preserves `chat` there.

*Alternative considered:* change `capabilitiesFromModalities` to require a text
*output* modality and thread input/output separately — deferred; it touches the
DeepSeek path without changing observable behavior for a text-only catalog.

### D4 — Webapp display of the vocabulary

Introduce `webapp/src/lib/llm-capabilities.ts` as the single source for the
capability union and its label keys. `ModelDetailsDialog.tsx` renders
`t(labelKey[cap])` with the raw value as fallback for unknown capabilities;
`AdminLlmProvidersTab.tsx` extends its `ModelCapability` union, label map, and
the `inferCapabilities` filter to accept all standardized values. All labels are
added to `translations.en.ts` and `translations.ru.ts` together.

### D5 — Instrument model filter fails closed

`filterModelsForInstrument` keeps its instrument→capability values but no longer
returns every model when an instrument type is missing from the mapping. An
unrecognized type yields an empty list, so a missing or mistyped mapping can
never leak non-compatible models into a dropdown — for example a chat-only model
into the Embeddings LLM Settings list. The embedding dropdown therefore lists only
models whose capabilities include `embedding`.

*Alternative considered:* leave the fail-open branch and rely on the mapping being
complete — rejected because a single missing or renamed key would silently expose
every model to every instrument.

## Risks / Trade-offs

- **[Admin-facing metadata changes]** → Documented as **BREAKING** in the
  proposal; capability values for Alibaba models change.
- **[Fail-closed filter hides models for an unmapped instrument type]** → An
  instrument added without a capability mapping would show an empty list instead
  of every model; the explicit mapping and its regression test surface the
  omission during development.
- **[`exactly` assertions may be order-sensitive]** → Backend tests compare
  capability sets order-insensitively (sort before compare).
- **[Vision heuristic could regress image-input chat models]** → The rule adds
  `vision` only when the response modality is text; pure generators are
  unaffected.
- **[New i18n keys drift]** → The `translations.test.ts` parity test plus
  `npm run lint && npx tsc -b` catch missing/duplicate keys.

## Migration Plan

No data migration. The model catalog is queried live per provider request; after
deploying core, the next `GET /api/llm/models` call returns the corrected
capabilities. Rollback is a revert of the change; no persisted state is
affected.

## Evidence plan

Runner IDs resolve from `.focused-spec/config.yaml` (`go`, `vitest`). Each new
scenario binds to one focused evidence reference; existing scenarios keep their
current bindings in `.focused-spec/evidence.yaml` (registry writes happen in the
apply phase).

| Scenario id | Runner | Selector |
|---|---|---|
| `llm-model-metadata.alibaba-image-generation` | `go` | `./internal/infrastructure/llm::TestAlibabaImageGenerationModelIsImageEditCapable` |
| `llm-model-metadata.alibaba-video-generation` | `go` | `./internal/infrastructure/llm::TestAlibabaVideoGenerationCapability` |
| `llm-model-metadata.alibaba-world-model` | `go` | `./internal/infrastructure/llm::TestAlibabaWorldModelCapability` |
| `llm-model-metadata.alibaba-text-to-speech` | `go` | `./internal/infrastructure/llm::TestAlibabaTextToSpeechCapability` |
| `llm-model-metadata.alibaba-speech-recognition` | `go` | `./internal/infrastructure/llm::TestAlibabaSpeechRecognitionCapability` |
| `llm-model-metadata.alibaba-realtime` | `go` | `./internal/infrastructure/llm::TestAlibabaRealtimeAsrCapability` |
| `llm-model-metadata.alibaba-omni` | `go` | `./internal/infrastructure/llm::TestAlibabaOmniCapability` |
| `llm-model-metadata.alibaba-reasoning` | `go` | `./internal/infrastructure/llm::TestAlibabaReasoningCapability` |
| `llm-model-metadata.model-details-new-capability-label` | `vitest` | `webapp/src/components/settings/ModelDetailsDialog.test.tsx::ModelDetailsDialog > labels a new capability` |
| `llm-model-metadata.ollama-chat-vision-not-embedding` | `go` | `./internal/infrastructure/llm::TestOllamaChatVisionModelIsNotEmbeddingCapable` |
| `llm-settings.chat-excludes-non-text` | `vitest` | `webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > chat excludes non-text models` |
| `llm-settings.embedding-excludes-non-embedding` | `vitest` | `webapp/src/lib/llm-model-filter.test.ts::filterModelsForInstrument > embedding excludes a chat/vision model (e.g. Ollama gemma4)` |

## Open Questions

None.
