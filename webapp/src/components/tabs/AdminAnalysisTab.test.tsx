import { describe, it, expect, vi, beforeEach } from "vitest"
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react"
import { AdminAnalysisTab } from "./AdminAnalysisTab"
import type { LlmModelDTO, LlmSettingsResponse } from "@/types"

// Controls which locale the mocked translator resolves.
const languageState = vi.hoisted(() => ({ language: "en" as "en" | "ru" }))

const endpoints = vi.hoisted(() => ({
  fetchLlmSettings: vi.fn(),
  fetchLlmModels: vi.fn(),
  updateLlmSettings: vi.fn(() => Promise.resolve({ message: "ok" })),
  fetchOCRStatus: vi.fn(() => Promise.resolve({ status: { enabled: false, health: "unknown" } })),
  fetchOcrClassificationStatus: vi.fn(() =>
    Promise.resolve({ processing: false, progress: "", filesProcessed: 0, totalFiles: 0 })
  ),
  startOcrClassification: vi.fn(),
  startOcrClassificationChanges: vi.fn(),
  stopOcrClassification: vi.fn(),
  fetchSettings: vi.fn(() => Promise.resolve({ ocrConcurrentRequests: 4 })),
  fetchExifServiceStatus: vi.fn(() => Promise.resolve({ enabled: false, health: "unknown" })),
  fetchTagScanStatus: vi.fn(() => Promise.resolve(null)),
  pauseTagScan: vi.fn(),
  resumeTagScan: vi.fn(),
  probeEmbeddingDimension: vi.fn(() => Promise.resolve({ dimension: 1024 })),
}))

vi.mock("@/api/endpoints", () => endpoints)

// Resolve real strings so the stale-notice assertions check the shipped copy.
// `translate` is created once so `t` keeps a stable identity across renders;
// an unstable `t` would re-trigger the tab's initial-load effect.
vi.mock("@/i18n", async () => {
  const en = await import("@/i18n/translations.en")
  const ru = await import("@/i18n/translations.ru")
  const translate = (key: string, vars?: Record<string, unknown>) => {
    const dictionary = (
      languageState.language === "ru" ? ru.translationsRu : en.translationsEn
    ) as Record<string, string>
    const text = dictionary[key] ?? key
    if (!vars) return text
    return text.replace(/\{\{?\s*(\w+)\s*\}?\}/g, (_match, name: string) => String(vars[name] ?? ""))
  }
  const useTranslation = () => ({ language: languageState.language, t: translate })
  return { useTranslation }
})

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}))

const chatModel: LlmModelDTO = {
  id: "minicpm-v",
  name: "MiniCPM-V",
  capabilities: ["chat", "tool_calling", "vision"],
  inputModalities: ["text", "image"],
  outputModalities: ["text"],
}

const embeddingOnlyModel: LlmModelDTO = {
  id: "nomic-embed-text",
  name: "Nomic Embed Text",
  capabilities: ["embedding"],
  inputModalities: ["text"],
  outputModalities: ["text"],
}

const settings: LlmSettingsResponse = {
  instruments: [
    { type: "chat", providerId: 1, model: "minicpm-v", providerAlias: "ollama_1", providerName: "ollama" },
    { type: "vl", providerId: 1, model: "minicpm-v", providerAlias: "ollama_1", providerName: "ollama" },
    { type: "ocr", providerId: 1, model: "minicpm-v", providerAlias: "ollama_1", providerName: "ollama" },
    { type: "image_edit", providerId: 1, model: "minicpm-v", providerAlias: "ollama_1", providerName: "ollama" },
    { type: "embedding", providerId: 1, model: "qwen3-embedding:4b", providerAlias: "ollama_1", providerName: "ollama" },
  ],
  tagScan: { enabled: false, startHour: 23, startMinute: 0, endHour: 7, endMinute: 0, timezoneOffset: 0 },
  embedding: { dimension: 1024, batchSize: 50 },
  providers: [
    { id: 1, alias: "ollama_1", name: "ollama", apiUrl: "http://localhost:11434", apiKey: "", cachedModels: null },
  ],
}

// modelsResponse builds a models payload.
function modelsResponse(models: LlmModelDTO[], stale = false) {
  return { success: true, provider: "ollama", models, stale, refreshError: stale ? "provider unreachable" : undefined }
}

// instrumentCard returns the card element that owns an instrument heading, so
// assertions can target one instrument instead of the whole tab.
function instrumentCard(heading: string): HTMLElement {
  const card = screen.getByText(heading).closest(".rounded-lg")
  if (!(card instanceof HTMLElement)) {
    throw new Error(`instrument card ${heading} not found`)
  }
  return card
}

beforeEach(() => {
  vi.clearAllMocks()
  languageState.language = "en"
  endpoints.fetchLlmSettings.mockResolvedValue(settings)
  endpoints.fetchLlmModels.mockResolvedValue(modelsResponse([chatModel]))
})

describe("AdminAnalysisTab", () => {
  it("loads models on mount without forcing a refresh", async () => {
    render(<AdminAnalysisTab />)

    await waitFor(() => expect(endpoints.fetchLlmModels).toHaveBeenCalled())
    expect(endpoints.fetchLlmModels).toHaveBeenCalledWith("ollama_1", false)

    // The instrument dropdowns are filled from the stored catalog without the
    // admin pressing "load models".
    await waitFor(() =>
      expect(screen.getAllByText("Enter model name manually").length).toBeGreaterThan(0)
    )
  })

  it("refresh action replaces the dropdown models", async () => {
    endpoints.fetchLlmModels
      .mockResolvedValueOnce(modelsResponse([chatModel]))
      .mockResolvedValue(modelsResponse([embeddingOnlyModel]))

    render(<AdminAnalysisTab />)

    await waitFor(() =>
      expect(screen.getAllByText("Enter model name manually")).toHaveLength(3)
    )

    fireEvent.click(screen.getAllByRole("button", { name: /Load models/i })[0])

    await waitFor(() => expect(endpoints.fetchLlmModels).toHaveBeenCalledWith("ollama_1", true))

    // The refreshed catalog replaces the previous list: only the embedding
    // instrument still finds a compatible model, so a merge cannot explain this.
    await waitFor(() =>
      expect(screen.getAllByText("Enter model name manually")).toHaveLength(1)
    )
  })

  it("renders the stale notice in English", async () => {
    endpoints.fetchLlmModels.mockResolvedValue(modelsResponse([chatModel], true))

    render(<AdminAnalysisTab />)

    await screen.findAllByText("The model list may be outdated")
    // The instrument card itself must carry the notice, not only the
    // embeddings section.
    expect(
      within(instrumentCard("Chat LLM Settings")).queryByText("The model list may be outdated")
    ).not.toBeNull()
  })

  it("renders the stale notice in Russian", async () => {
    languageState.language = "ru"
    endpoints.fetchLlmModels.mockResolvedValue(modelsResponse([chatModel], true))

    render(<AdminAnalysisTab />)

    await screen.findAllByText("Список моделей может быть устаревшим")
    expect(
      within(instrumentCard("Настройки чат LLM")).queryByText("Список моделей может быть устаревшим")
    ).not.toBeNull()
  })
})
