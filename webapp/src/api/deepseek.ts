/**
 * DeepSeek Client — frontend utilities for the DeepSeek LLM provider.
 *
 * Provides:
 * - A registry of known DeepSeek models with their context window sizes
 * - Helpers to parse DeepSeek's extended usage response (cache hits, reasoning tokens)
 * - A convenience `DirectDeepSeekClient` for browser-to-API calls (bypasses backend proxy)
 *
 * DeepSeek API extends OpenAI with:
 *   usage.prompt_cache_hit_tokens
 *   usage.prompt_cache_miss_tokens
 *   usage.completion_tokens_details.reasoning_tokens
 */

// ─── Model Registry ────────────────────────────────────────────────────────────

export interface DeepSeekModelEntry {
  /** Display name */
  name: string
  /** Maximum context window in tokens */
  contextWindow: number
  /** Maximum output tokens */
  maxOutput: number
  /** Whether the model supports reasoning (chain-of-thought) */
  supportsReasoning: boolean
}

/**
 * Known DeepSeek models and their context window sizes.
 * Source: https://api-docs.deepseek.com
 */
export const DEEPSEEK_MODELS: Record<string, DeepSeekModelEntry> = {
  "deepseek-chat": {
    name: "DeepSeek Chat",
    contextWindow: 128_000,
    maxOutput: 8_192,
    supportsReasoning: false,
  },
  "deepseek-reasoner": {
    name: "DeepSeek Reasoner",
    contextWindow: 128_000,
    maxOutput: 8_192,
    supportsReasoning: true,
  },
  "deepseek-v4-pro": {
    name: "DeepSeek V4 Pro",
    contextWindow: 1_000_000,
    maxOutput: 384_000,
    supportsReasoning: true,
  },
  "deepseek-v4-flash": {
    name: "DeepSeek V4 Flash",
    contextWindow: 1_000_000,
    maxOutput: 384_000,
    supportsReasoning: true,
  },
  "deepseek-v4-pro-0324": {
    name: "DeepSeek V4 Pro (0324)",
    contextWindow: 1_000_000,
    maxOutput: 384_000,
    supportsReasoning: true,
  },
  "deepseek-v4-flash-0324": {
    name: "DeepSeek V4 Flash (0324)",
    contextWindow: 1_000_000,
    maxOutput: 384_000,
    supportsReasoning: true,
  },
}

/**
 * Returns the known context window for a DeepSeek model, or a safe fallback.
 */
export function getContextWindow(model: string): number {
  // Exact match
  if (DEEPSEEK_MODELS[model]) {
    return DEEPSEEK_MODELS[model].contextWindow
  }
  // Prefix match (e.g. "deepseek-chat-v2" → "deepseek-chat")
  for (const [prefix, entry] of Object.entries(DEEPSEEK_MODELS)) {
    if (model.startsWith(prefix)) {
      return entry.contextWindow
    }
  }
  return 128_000 // safe fallback
}

/**
 * Returns the model info entry for a DeepSeek model, or undefined.
 */
export function getModelInfo(model: string): DeepSeekModelEntry | undefined {
  if (DEEPSEEK_MODELS[model]) {
    return DEEPSEEK_MODELS[model]
  }
  for (const [prefix, entry] of Object.entries(DEEPSEEK_MODELS)) {
    if (model.startsWith(prefix)) {
      return entry
    }
  }
  return undefined
}

// ─── Extended Usage Parsing ────────────────────────────────────────────────────

/**
 * Extended token usage from DeepSeek API.
 * These fields go beyond the standard OpenAI usage object.
 */
export interface DeepSeekUsage {
  promptTokens: number
  completionTokens: number
  totalTokens: number
  /** Tokens served from the context cache (reduced cost) */
  promptCacheHitTokens: number
  /** Tokens that missed the cache */
  promptCacheMissTokens: number
  /** Tokens used for reasoning/chain-of-thought (deepseek-reasoner / deepseek-v4) */
  reasoningTokens: number
}

/**
 * Parse a raw DeepSeek API usage object into a typed DeepSeekUsage.
 * Returns null if the input is empty or missing required fields.
 */
export function parseDeepSeekUsage(raw: Partial<DeepSeekUsage> | null | undefined): DeepSeekUsage | null {
  if (!raw || typeof raw.promptTokens !== "number") return null
  return {
    promptTokens: raw.promptTokens ?? 0,
    completionTokens: raw.completionTokens ?? 0,
    totalTokens: raw.totalTokens ?? 0,
    promptCacheHitTokens: raw.promptCacheHitTokens ?? 0,
    promptCacheMissTokens: raw.promptCacheMissTokens ?? 0,
    reasoningTokens: raw.reasoningTokens ?? 0,
  }
}

// ─── Direct API Client (browser → DeepSeek) ────────────────────────────────────

export interface DirectChatMessage {
  role: "system" | "user" | "assistant"
  content: string
}

export interface DirectChatRequest {
  model: string
  messages: DirectChatMessage[]
  maxTokens?: number
}

export interface DirectChatResponse {
  id: string
  model: string
  choices: Array<{
    index: number
    message: {
      role: string
      content: string
    }
    finishReason: string
  }>
  usage: DeepSeekUsage
}

/**
 * DirectDeepSeekClient makes API calls directly from the browser to the DeepSeek API.
 *
 * NOTE: This requires the API key to be exposed in the browser context.
 * Use only if your backend proxy does not support DeepSeek's extended fields.
 * The recommended approach is to use the backend proxy which now passes through
 * DeepSeek's extended usage data via SSE events.
 */
export class DirectDeepSeekClient {
  private baseURL: string
  private apiKey: string

  constructor(apiKey: string, baseURL = "https://api.deepseek.com") {
    this.apiKey = apiKey
    this.baseURL = baseURL.replace(/\/+$/, "").replace(/\/v1$/, "")
  }

  /**
   * Send a chat completion request. Returns the full response including extended usage.
   */
  async chat(req: DirectChatRequest): Promise<DirectChatResponse> {
    const response = await fetch(`${this.baseURL}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${this.apiKey}`,
      },
      body: JSON.stringify({
        model: req.model,
        messages: req.messages,
        max_tokens: req.maxTokens ?? 4_000,
      }),
    })

    if (!response.ok) {
      const errorBody = await response.text()
      throw new Error(`DeepSeek API error ${response.status}: ${errorBody}`)
    }

    const data = await response.json()
    return {
      id: data.id,
      model: data.model,
      choices: (data.choices ?? []).map((c: Record<string, unknown>) => ({
        index: c.index as number,
        message: c.message as DirectChatResponse["choices"][0]["message"],
        finishReason: (c.finish_reason ?? c.finishReason) as string,
      })),
      usage: parseDeepSeekUsage(data.usage as Partial<DeepSeekUsage>) ?? {
        promptTokens: 0,
        completionTokens: 0,
        totalTokens: 0,
        promptCacheHitTokens: 0,
        promptCacheMissTokens: 0,
        reasoningTokens: 0,
      },
    }
  }

  /**
   * Fetch available models from the DeepSeek API.
   * Each model is annotated with its known context window.
   */
  async listModels(): Promise<Array<{ id: string; contextWindow: number }>> {
    const response = await fetch(`${this.baseURL}/v1/models`, {
      headers: {
        Authorization: `Bearer ${this.apiKey}`,
      },
    })

    if (!response.ok) {
      throw new Error(`DeepSeek API error ${response.status}`)
    }

    const data: { data: Array<{ id: string }> } = await response.json()
    return (data.data ?? []).map((m) => ({
      id: m.id,
      contextWindow: getContextWindow(m.id),
    }))
  }
}

// ─── Formatting Helpers ────────────────────────────────────────────────────────

/**
 * Format a number for display (e.g. 128000 → "128K").
 */
export function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`
  return String(n)
}

/**
 * Calculate cache hit ratio as a percentage string.
 */
export function cacheHitPercent(hit: number, miss: number): string {
  const total = hit + miss
  if (total === 0) return "0%"
  return `${((hit / total) * 100).toFixed(0)}%`
}
