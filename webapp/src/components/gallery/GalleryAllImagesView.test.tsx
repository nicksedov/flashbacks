import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { GalleryAllImagesView } from "./GalleryAllImagesView"
import { hasClass } from "@/test/dom"

vi.mock("@/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock("@/api/endpoints", () => ({
  moveFiles: vi.fn(),
}))

vi.mock("@/hooks/useGalleryImages", () => ({
  useGalleryImages: () => ({
    images: [],
    totalImages: 0,
    hasMore: false,
    isLoading: false,
    error: null,
    initialized: false,
    loadMore: vi.fn(),
    removeImage: vi.fn(),
  }),
}))

vi.mock("@/hooks/useGalleryFolders", () => ({
  useGalleryFolders: () => ({ folders: [] }),
}))

vi.mock("@/hooks/useIntersectionObserver", () => ({
  useIntersectionObserver: () => ({ current: null }),
}))

vi.mock("@/providers/useGallerySelection", () => ({
  useGallerySelection: () => ({ registerActions: vi.fn() }),
}))

vi.mock("@/components/gallery/GalleryImageGrid", () => ({
  GalleryImageGrid: () => null,
}))

vi.mock("@/components/gallery/BulkMoveDialog", () => ({
  BulkMoveDialog: () => null,
}))

describe("GalleryAllImagesView", () => {
  it("stacks the search and sort controls vertically on mobile", () => {
    render(<GalleryAllImagesView onImageClick={vi.fn()} />)

    // The toolbar row stacks below md and becomes a row from md up.
    const searchInput = screen.getByPlaceholderText("gallery.search.placeholder")
    const headerRow = searchInput.closest(".flex-col")
    expect(headerRow).not.toBeNull()
    expect(hasClass(headerRow, "md:flex-row")).toBe(true)

    // The search input is full-width on mobile, fixed width from sm up.
    expect(hasClass(searchInput, "w-full", "sm:w-70")).toBe(true)

    // The sort control keeps an accessible name with its label hidden below md.
    const sortButton = screen.getByRole("button", { name: "gallery.sortNewest" })
    expect(hasClass(sortButton.querySelector("span"), "hidden", "md:inline")).toBe(true)
  })

  it("collapses the image count to an info icon on mobile", () => {
    render(<GalleryAllImagesView onImageClick={vi.fn()} />)

    // The info icon carries the full count text and is shown only below md.
    const infoButton = screen.getByRole("button", { name: "gallery.imageCount" })
    expect(infoButton).toBeTruthy()
    expect(infoButton.closest(".md\\:hidden")).not.toBeNull()
  })
})
