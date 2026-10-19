import type { TranslationKey } from "@/i18n"

/**
 * Standardized LLM capability vocabulary, mirrored from the core service's
 * `normalizeCapabilities`. The first five values drive instrument filtering in
 * [`llm-model-filter.ts`](./llm-model-filter.ts); the remaining seven are
 * metadata shown to admins in the providers tab and the model details dialog.
 */
export const LLM_CAPABILITIES = [
  "chat",
  "tool_calling",
  "vision",
  "embedding",
  "image_edit",
  "reasoning",
  "video_generation",
  "text_to_speech",
  "speech_recognition",
  "realtime",
  "omni",
  "world_model",
] as const

export type LlmCapability = (typeof LLM_CAPABILITIES)[number]

const CAPABILITY_LABEL_KEYS: Record<LlmCapability, TranslationKey> = {
  chat: "llm_capabilities.chat",
  tool_calling: "llm_capabilities.toolCalling",
  vision: "llm_capabilities.vision",
  embedding: "llm_capabilities.embedding",
  image_edit: "llm_capabilities.imageEdit",
  reasoning: "llm_capabilities.reasoning",
  video_generation: "llm_capabilities.videoGeneration",
  text_to_speech: "llm_capabilities.textToSpeech",
  speech_recognition: "llm_capabilities.speechRecognition",
  realtime: "llm_capabilities.realtime",
  omni: "llm_capabilities.omni",
  world_model: "llm_capabilities.worldModel",
}

/** [TYPE GUARD] Returns true when a raw capability string is standardized. */
export function isLlmCapability(capability: string): capability is LlmCapability {
  return (LLM_CAPABILITIES as readonly string[]).includes(capability)
}

/**
 * Returns the i18n label key for a standardized capability, or `undefined` for
 * an unknown value so callers can fall back to the raw string.
 */
export function capabilityLabelKey(capability: string): TranslationKey | undefined {
  return isLlmCapability(capability) ? CAPABILITY_LABEL_KEYS[capability] : undefined
}
