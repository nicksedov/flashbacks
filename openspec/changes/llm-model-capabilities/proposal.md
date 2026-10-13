# Proposal

## Why

Alibaba ModelStudio reports model capabilities with native codes (`IG`, `VG`, `TTS`,
`ASR`, `Reasoning`, `Realtime-*`, `Multimodal-Omni`, `World-Model`) that the current
normalizer only partially maps. Two defects result: image-generation models can be
dropped from the image-edit instrument because their capability code is unmapped,
and — more broadly — because capability derivation unconditionally seeds
`chat`/`tool_calling` from modalities, non-chat models (video generation,
text-to-speech, speech recognition) are advertised as chat-capable and appear in
the chat instrument dropdown. The metadata surfaced to admins is therefore both
incomplete and misleading.

## What Changes

- Expand the standardized capability vocabulary with `reasoning`, `video_generation`,
  `text_to_speech`, `speech_recognition`, `realtime`, `omni`, and `world_model`,
  alongside the existing `chat`, `tool_calling`, `vision`, `embedding`, and
  `image_edit`. The new values may not be consumed by any current instrument; they
  exist so model metadata is accurate and future instruments can use them.
- Map every Alibaba ModelStudio native capability code to the standardized
  vocabulary, including multi-value codes (`Realtime-ASR` → `realtime` +
  `speech_recognition`; `Multimodal-Omni` → `omni` + `chat`; `World-Model` →
  `world_model` + `video_generation`).
- Stop seeding `chat`/`tool_calling` for models that cannot produce text output, so
  only genuinely chat-capable models are advertised as chat.
- **BREAKING** (admin-facing metadata): the `capabilities` values returned by the LLM
  models API change for Alibaba models — non-text generators lose the incorrect
  `chat`/`tool_calling` (and where applicable `vision`) values, and gain the new
  specific values.
- Surface the standardized capabilities in the LLM model details view with localized
  (en/ru) labels, and include the new capabilities in the LLM Providers model list.

## Capabilities

### New Capabilities

_None — the behavior changes belong to the existing metadata and settings capabilities._

### Modified Capabilities

- `llm-model-metadata`: the standardized capability vocabulary is expanded and every
  provider capability code is mapped; capability derivation no longer assigns
  `chat`/`tool_calling` to non-text models; the model details view displays the new
  capabilities with en/ru labels.
- `llm-settings`: the chat instrument dropdown lists only genuinely chat-capable
  models; non-text generators (video, speech, image generation) are excluded.

## Impact

- **Backend core**: `backend/core/internal/infrastructure/llm/metadata.go`
  (`normalizeCapabilities`, `capabilitiesFromModalities`) and
  `backend/core/internal/infrastructure/llm/alibaba_client.go`
  (`alibabaCatalogModelToInfo`), plus their unit tests under
  `backend/core/internal/infrastructure/llm/`.
- **Webapp**: `webapp/src/components/settings/ModelDetailsDialog.tsx`,
  `webapp/src/components/tabs/AdminLlmProvidersTab.tsx`, a shared capability
  vocabulary module, `webapp/src/i18n/translations.en.ts` +
  `translations.ru.ts`, and their tests.
- **API contract**: `docs/api-contracts/core.yaml`, schema `LlmModelDTO.capabilities`
  — the value set expands; the field remains an array of strings (no enum), so no
  schema change and no `make generate-types` regeneration is required.
- **Not affected**: database schema, Wire DI graph, and inter-service REST calls.

## Non-goals

- No new instrument types. No video, speech, or audio instrument is introduced; the
  new capabilities are metadata only.
- No change to the instrument→capability mapping in
  `webapp/src/lib/llm-model-filter.ts` (chat stays `chat`; vl/ocr stay `vision`;
  image edit stays `image_edit`; embedding stays `embedding`).
- No change to Ollama/DeepSeek/OpenAI provider parsing beyond keeping them consistent
  with the shared normalizer.
- No new dependency.
