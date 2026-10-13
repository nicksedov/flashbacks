import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { SmartSearchTab } from "./SmartSearchTab"
import { hasClass } from "@/test/dom"

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
  useSmartSearch: () => ({
    results: [],
    total: 0,
    query: "",
    isLoading: false,
    error: null,
    searched: false,
    search: vi.fn(),
    removeResults: vi.fn(),
    reset: vi.fn(),
  }),
}))

vi.mock("@/hooks/usePolling", () => ({
  usePolling: () => ({ data: null, start: vi.fn(), stop: vi.fn(), setOnComplete: vi.fn() }),
}))

vi.mock("@/api/endpoints", () => ({
  fetchEmbeddingStatus: vi
    .fn()
    .mockResolvedValue({ running: false, progress: { total: 0, remaining: 0, processed: 0 } }),
  startEmbeddingBackfill: vi.fn(),
  stopEmbeddingBackfill: vi.fn(),
  deleteFiles: vi.fn(),
  moveFiles: vi.fn(),
}))

vi.mock("@/components/gallery/UnifiedLightbox", () => ({ UnifiedLightbox: () => null }))
vi.mock("@/components/gallery/DeleteConfirmDialog", () => ({ DeleteConfirmDialog: () => null }))
vi.mock("@/components/gallery/BulkDeleteDialog", () => ({ BulkDeleteDialog: () => null }))
vi.mock("@/components/gallery/BulkMoveDialog", () => ({ BulkMoveDialog: () => null }))
vi.mock("@/components/ui/confirm-dialog", () => ({ ConfirmDialog: () => null }))

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
})
