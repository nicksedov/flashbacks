import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import { UnifiedLightbox } from "./UnifiedLightbox"
import { hasClass } from "@/test/dom"

interface MockToolCall {
  name: string
  result: string
}
interface MockMessage {
  id: number
  role: string
  toolCalls: MockToolCall[]
}

const chatState = vi.hoisted(() => ({ messages: [] as MockMessage[] }))

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key, language: "en" }),
}))

vi.mock("@/hooks/useOcrState", () => ({
  useOcrState: () => ({
    ocrData: null,
    llmData: null,
    loading: false,
    recognizing: false,
    resetState: vi.fn(),
    handleRecognize: vi.fn(),
  }),
}))

vi.mock("@/hooks/useTagsState", () => ({
  useTagsState: () => ({
    tagsData: null,
    loading: false,
    generating: false,
    error: null,
    resetState: vi.fn(),
    handleGenerate: vi.fn(),
  }),
}))

vi.mock("@/hooks/useImageDimensions", () => ({
  useImageDimensions: () => ({
    imageRef: { current: null },
    displayDimensions: null,
    imageLoaded: true,
    handleImageLoad: vi.fn(),
  }),
}))

vi.mock("@/hooks/useFileExport", () => ({
  useFileExport: () => ({ getFileName: () => "file", handleSaveMd: vi.fn(), handleSaveHtml: vi.fn() }),
}))

vi.mock("@/hooks/useImageMetadata", () => ({
  useImageMetadata: () => ({ metadata: null, isLoading: false, reload: vi.fn() }),
}))

vi.mock("@/hooks/useChatAgent", () => ({
  useChatAgent: () => ({
    conversation: null,
    conversations: [],
    messages: chatState.messages,
    isStreaming: false,
    error: null,
    tokenCount: 0,
    maxTokens: 0,
    isTokenLimitReached: false,
    currentImagePath: null,
    promptTokens: 0,
    completionTokens: 0,
    promptCacheHitTokens: 0,
    promptCacheMissTokens: 0,
    reasoningTokens: 0,
    createNewConversation: vi.fn(),
    loadConversation: vi.fn(),
    loadConversations: vi.fn(),
    removeConversation: vi.fn(),
    sendMessage: vi.fn(),
    abortStream: vi.fn(),
    resetForImage: vi.fn(),
  }),
}))

vi.mock("@/api/endpoints", () => ({
  replaceEnhancement: vi.fn(),
  saveCopyEnhancement: vi.fn(),
  rejectEnhancement: vi.fn(),
}))

function enhancementMessage(): MockMessage {
  return {
    id: 1,
    role: "assistant",
    toolCalls: [
      {
        name: "enhance_image_quality",
        result: JSON.stringify({ enhancedPath: "enhanced.jpg", status: "pending_approval" }),
      },
    ],
  }
}

describe("UnifiedLightbox", () => {
  beforeEach(() => {
    chatState.messages = []
    // jsdom does not implement scrollIntoView, which ChatPanel calls on mount.
    Element.prototype.scrollIntoView = vi.fn()
  })

  it("renders the detail panel collapsed by default and toggles it open and closed", () => {
    render(<UnifiedLightbox imagePath="a.jpg" initialMode="tags" onClose={vi.fn()} />)

    const panel = screen.getByTestId("mobile-detail-panel")
    const toggle = screen.getByTestId("mobile-detail-toggle")

    expect(panel.getAttribute("data-open")).toBe("false")

    fireEvent.click(toggle)
    expect(panel.getAttribute("data-open")).toBe("true")

    fireEvent.click(toggle)
    expect(panel.getAttribute("data-open")).toBe("false")
  })

  it("keeps the compare-slider branch full-area with no side column below md", async () => {
    chatState.messages = [enhancementMessage()]
    render(<UnifiedLightbox imagePath="a.jpg" initialMode="tags" onClose={vi.fn()} />)

    const enhanced = await screen.findByAltText("Enhanced")
    const slider = enhanced.closest("div.w-full.h-full")
    const branch = slider?.parentElement ?? null

    // The compare slider lives in the flexible, full-area image region…
    expect(branch).not.toBeNull()
    expect(hasClass(branch, "flex-1")).toBe(true)
    // …and the detail panel is an overlay below md, so it adds no side column.
    expect(hasClass(screen.getByTestId("mobile-detail-panel"), "absolute")).toBe(true)
  })

  it("renders the image stage consistently across all lightbox modes", () => {
    const modes = ["ai", "exif", "ocr", "tags"] as const
    // Surface and layout classes every image-stage branch must share.
    const stageClasses = ["flex-1", "items-center", "justify-center", "bg-black", "relative", "h-full", "min-w-0"]

    const stages = modes.map((mode) => {
      const { unmount } = render(<UnifiedLightbox imagePath="a.jpg" initialMode={mode} onClose={vi.fn()} />)
      const stage = screen.getByAltText("lightbox.alt").parentElement
      expect(stage).not.toBeNull()
      const classes = stage!.className
      unmount()
      return { mode, classes }
    })

    for (const { mode, classes } of stages) {
      // Every mode renders the image on the same dark, flexible stage.
      expect(classes.split(/\s+/), `image stage of mode "${mode}"`).toEqual(
        expect.arrayContaining(stageClasses)
      )
    }

    // No branch may introduce its own background utility.
    const backgrounds = stages.map(({ classes }) => classes.split(/\s+/).filter((c) => c.startsWith("bg-")))
    expect(backgrounds.every((bg) => bg.length === 1 && bg[0] === "bg-black")).toBe(true)
  })

  it("collapses compare-slider actions to icons below md", async () => {
    chatState.messages = [enhancementMessage()]
    render(<UnifiedLightbox imagePath="a.jpg" initialMode="tags" onClose={vi.fn()} />)

    for (const name of ["enhance.reject", "enhance.saveCopy", "enhance.replace"]) {
      const action = await screen.findByRole("button", { name })
      // Icon stays, text label is hidden below md, 44px touch target on mobile.
      expect(action.querySelector("svg")).not.toBeNull()
      expect(hasClass(action.querySelector("span"), "hidden", "md:inline")).toBe(true)
      expect(hasClass(action, "h-11", "min-w-11")).toBe(true)
    }
  })
})
