# Design

## Context

See [`proposal.md`](proposal.md) for motivation and
[`specs/llm-model-metadata/spec.md`](specs/llm-model-metadata/spec.md) for the
required behavior.

The capability pipeline lives in the infrastructure layer
([`metadata.go`](../../../backend/core/internal/infrastructure/llm/metadata.go)) and
is provider-agnostic: provider clients collect a `ModelInfo`, and the pipeline
fills unknown fields in a fixed priority order — the provider's native codes, the
curated model-family registry, model-ID heuristics, then the safe default — before
one normalization step. Capabilities then travel through the application catalog
([`catalog.go`](../../../backend/core/internal/application/llm/catalog.go)) into the
stored provider catalog, which the models endpoint serves and the webapp filters on
`image_edit`.

Two constraints shape the approach:

1. `image_edit` is the instrument capability for image generation *and* editing
   (ModelStudio's `IG` is already read that way), and such a model must never be
   advertised as `chat`.
2. A stored catalog outlives the code that collected it, so a classification defect
   has to be repairable by a refresh, not only by a fresh installation.

## Goals / Non-Goals

**Goals:**

- Recognize image-generation and editing models from identifiers without relying on
  a provider capability code that may be omitted.
- Keep image generators out of the chat/VL dropdowns while making them visible to
  the image edit instrument.
- Make the correction reach catalogs that were collected before the fix, without
  weakening the guarantee that an identifier-only listing never overwrites
  provider-reported metadata.

**Non-Goals:**

- No webapp change: `filterModelsForInstrument` remains the single consumer of the
  stored capability.
- No provider-native catalog contract change, no API schema change, no migration.
- No identifier-based video-capability detection; the provider's `VG` code stays
  the source for `video_generation`.

## Decisions

**1. Identifier detection as a shared predicate, not a registry entry.**
A new `isImageGenerationModelID` predicate combines a curated token list
(`image-edit`, `image_edit`, `imageedit`, `qwen-image`, `wanx`, `cogview`,
`gpt-image`, `dall-e`, `stable-diffusion`, `i2i`, `t2i`) with a structural rule for
versioned `wan*`/`qwen*` identifiers that also contain `image`. The registry cannot
express this: its tokens are matched as substrings, and no static token survives the
version number in `wan2.7-image`, while a broad `wan` token would wrongly match the
video variants. The structural rule keeps `wan2.7-t2v`/`wan2.7-i2v` out because they
never contain `image`.
*Alternative rejected:* adding `wan` to `modelFamilies` with `image_edit`
capabilities — it would classify the whole wan family, including video models, and
the registry deliberately only carries capabilities for unambiguous identifiers.

**2. Capability exclusivity for generators.** A detected image generator receives
exactly `image_edit` (no `chat`, `tool_calling`, or `vision`), mirroring the existing
handling of the `IG` code. This keeps the chat dropdown free of image-only models and
matches how `embedding` is already treated as exclusive.

**3. Modality derivation receives both directions.** `capabilitiesFromModalities`
now takes input and output lists: an `image` output implies `image_edit`, otherwise
an `image` input implies `vision` and the model is chat. The previous signature only
saw inputs, which is why an image-output model with no codes was recorded as chat.
`inferModalities` mirrors this in reverse: `image_edit` implies image input and image
output, so a model known only by identifier still reports coherent modalities.
*Alternative rejected:* special-casing only Alibaba — the defect is in the shared
pipeline and affects every provider that lists an image-output model without codes.

**4. Catalog conversion defers to the identifier for the wan/qwen image families.**
`alibabaCatalogModelToInfo` applies the same predicate after the native codes: a
family model that does not answer in text becomes exactly `image_edit`; one that
does answer in text gains `image_edit` additively. The separator-less
`ImageGeneration`/`imagegen` code spellings are mapped as well, so a catalog revision
that changes the code syntax cannot silently drop the capability.

**5. Refresh authority for provider-described models.**
`describesFully` accepts a `partial` entry when it carries capabilities, at least one
input modality, and a context length — that is, a model whose capabilities were
derived from provider-reported modalities rather than from heuristics. Without this,
an installation that stored `wan2.7-image` as `chat` would keep the stale value
forever: the collector deliberately preserves stored descriptive fields for entries
that are not fully described, and the corrected classification would never be
written. Entries whose provenance is `heuristic` (identifier-only listings such as the
compatible-mode fallback) keep the existing protection.
*Alternative rejected:* treating every partial entry as rich — an entry that reports
only a context window has no capability facts and must not replace stored values.

## Risks / Trade-offs

- **Token false positives** (an identifier containing `i2i`, `t2i`, or `image` next to
  `wan`/`qwen` that is not an image model) → each token names a documented image
  family, and the video variants of the same families are covered by an explicit
  negative scenario.
- **Partial entries now refresh capabilities and modalities**, so a provider that
  reports wrong modalities can replace a correct stored value → mitigation: the
  values come from the same provider report the catalog is built from, and
  identifier-only refreshes are still ignored; the behavior is limited to fields the
  entry actually determined.
- **Deletion boundary widens with `rich`** because `describesFully` also feeds the
  deletion flag → the flag now requires every listed model to be provider-described,
  which is the same condition the catalog path already satisfies; a heuristic listing
  still cannot delete stored models.
- **Existing catalogs stay stale until refresh** → the catalog already refreshes on
  its TTL or when an admin presses “load models”; no user action beyond that is
  needed.

## Migration Plan

Deploy the core service, then let the catalog TTL expire or press the model refresh
action on the Analysis tab (or Providers tab) for the Alibaba provider. The refresh
rewrites capabilities and modalities for models whose entries report modalities; no
schema change, no data migration, and rollback is a plain redeploy of the previous
core image (stored rows are re-derived on the next refresh).

## Open Questions

None.
