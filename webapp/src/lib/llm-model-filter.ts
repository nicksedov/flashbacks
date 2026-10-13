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
 */
export function filterModelsForInstrument(
  models: LlmModelDTO[],
  type: LlmInstrumentType,
): LlmModelDTO[] {
  const required = INSTRUMENT_CAPABILITY[type]
  if (!required) return models
  return models.filter((model) => (model.capabilities ?? []).includes(required))
}
