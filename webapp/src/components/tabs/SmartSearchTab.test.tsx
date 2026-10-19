import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen } from "@testing-library/react"
import { SmartSearchTab } from "./SmartSearchTab"
import { hasClass } from "@/test/dom"
import type { EmbeddingBackfillStatus, SmartSearchResult } from "@/types"

// Mutable hook state: each test seeds the smart-search state it needs before
// rendering the tab, so one render helper covers every UI state.
const hookState = vi.hoisted(() => ({
  results: [] as SmartSearchResult[],
  total: 0,
  query: "",
  isLoading: false,
  error: null as string | null,
  searched: false,
  search: vi.fn(),
  removeResults: vi.fn(),
  reset: vi.fn(),
}))

// Mutable polling state drives the embedding-status banners.
const pollingState = vi.hoisted(() => ({
  data: null as EmbeddingBackfillStatus | null,
  start: vi.fn(),
  stop: vi.fn(),
  setOnComplete: vi.fn(),
}))

const endpoints = vi.hoisted(() => ({
  fetchEmbeddingStatus: vi.fn(),
  fetchThumbnail: vi.fn(() => Promise.resolve({ thumbnail: "" })),
  startEmbeddingBackfill: vi.fn(),
  stopEmbeddingBackfill: vi.fn(),
  deleteFiles: vi.fn(),
  moveFiles: vi.fn(),
}))

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/providers/useSettings", () => ({
  useSettings: () => ({ trashDir: "" }),
}))

vi.mock("@/providers/useGallerySelection", () => ({
  useGallerySelection: () => ({ registerActions: vi.fn() }),
}))

vi.mock("@/hooks/useSmartSearch", () => ({
  MIN_QUERY_LENGTH: 3,
  useSmartSearch: () => hookState,
}))

vi.mock("@/hooks/usePolling", () => ({
  usePolling: () => pollingState,
}))

vi.mock("@/api/endpoints", () => endpoints)

vi.mock("@/components/gallery/UnifiedLightbox", () => ({ UnifiedLightbox: () => null }))
vi.mock("@/components/gallery/DeleteConfirmDialog", () => ({ DeleteConfirmDialog: () => null }))
vi.mock("@/components/gallery/BulkDeleteDialog", () => ({ BulkDeleteDialog: () => null }))
vi.mock("@/components/gallery/BulkMoveDialog", () => ({ BulkMoveDialog: () => null }))
vi.mock("@/components/ui/confirm-dialog", () => ({ ConfirmDialog: () => null }))

const embeddingsAvailable: EmbeddingBackfillStatus = {
  running: false,
  progress: { total: 50, processed: 50, remaining: 0, lastError: "" },
}

beforeEach(() => {
  vi.clearAllMocks()
  Object.assign(hookState, {
    results: [],
    total: 0,
    query: "",
    isLoading: false,
    error: null,
    searched: false,
  })
  pollingState.data = null
  endpoints.fetchEmbeddingStatus.mockResolvedValue(embeddingsAvailable)
  endpoints.fetchThumbnail.mockResolvedValue({ thumbnail: "" })
})

describe("SmartSearchTab", () => {
  it("stacks the search input and limit control vertically on mobile", () => {
    render(<SmartSearchTab />)

    const searchInput = screen.getByPlaceholderText("smartSearch.placeholder")

    // The search field is full-width on mobile and flexible from sm up.
    expect(hasClass(searchInput.parentElement, "w-full", "sm:flex-1")).toBe(true)

    // The search bar stacks below sm and becomes a row from sm up.
    const bar = searchInput.closest(".flex-col")
    expect(bar).not.toBeNull()
    expect(hasClass(bar, "sm:flex-row")).toBe(true)
  })

  it("renders the result count and a tile per result", async () => {
    hookState.results = [
      {
        id: 1,
        path: "/photos/sunset.jpg",
        fileName: "sunset.jpg",
        similarity: 1,
        tags: [],
        matchType: "exact",
      },
      {
        id: 2,
        path: "/photos/mountain.jpg",
        fileName: "mountain.jpg",
        similarity: 0.8,
        tags: [],
        matchType: "embedding",
      },
    ]
    hookState.total = 2
    hookState.query = "sunset"
    hookState.searched = true

    render(<SmartSearchTab />)

    expect(screen.getByText(/smartSearch\.resultCount/)).toBeTruthy()
    expect(await screen.findByText("sunset.jpg")).toBeTruthy()
    expect(await screen.findByText("mountain.jpg")).toBeTruthy()
  })

  it("shows the empty state after a search with no results", () => {
    hookState.searched = true

    render(<SmartSearchTab />)

    expect(screen.getByText("smartSearch.empty")).toBeTruthy()
    expect(screen.getByText("smartSearch.emptyHint")).toBeTruthy()
  })

  it("shows the backend error message when the search fails", () => {
    hookState.searched = true
    hookState.error = "Embedding provider unavailable"

    render(<SmartSearchTab />)

    expect(screen.getByText("Embedding provider unavailable")).toBeTruthy()
  })

  it("shows the no-embeddings hint when no embeddings exist", () => {
    pollingState.data = {
      running: false,
      progress: { total: 0, processed: 0, remaining: 0, lastError: "" },
    }

    render(<SmartSearchTab />)

    expect(screen.getByText("smartSearch.noEmbeddings")).toBeTruthy()
    expect(screen.getByText("smartSearch.configureHint")).toBeTruthy()
  })
})
