import type { LlmInstrumentType, LlmModelDTO } from "@/types"

// Standardized capability required by each instrument type.
const INSTRUMENT_CAPABILITY: Record<LlmInstrumentType, string> = {
  chat: "chat",
  vl: "vision",
  ocr: "vision",
  image_edit: "image_edit",
  embedding: "embedding",
}

/**
 * Returns the models whose capabilities are compatible with the given LLM
 * instrument type. Models without the required capability (including those with
 * an unknown/empty capability list) are excluded so each dropdown only lists
 * models that can actually serve the instrument.
 *
 * The filter fails closed: an unrecognized instrument type yields no models
 * rather than every model, so a missing or mistyped capability mapping can never
 * leak non-compatible models into an instrument dropdown (for example, a
 * chat-only model into the Embeddings LLM Settings list).
 */
export function filterModelsForInstrument(
  models: LlmModelDTO[],
  type: LlmInstrumentType,
): LlmModelDTO[] {
  const required = INSTRUMENT_CAPABILITY[type]
  if (!required) return []
  return models.filter((model) => (model.capabilities ?? []).includes(required))
}
