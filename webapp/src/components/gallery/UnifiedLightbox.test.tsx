import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react"
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

const dimensions = vi.hoisted(() => ({
  value: null as { width: number; height: number } | null,
}))

const endpoints = vi.hoisted(() => ({
  replaceEnhancement: vi.fn(),
  saveCopyEnhancement: vi.fn(),
  rejectEnhancement: vi.fn(),
  fetchImageFaces: vi.fn(),
  fetchPeople: vi.fn(),
  fetchPersonSuggestions: vi.fn(),
  renamePerson: vi.fn(),
  mergePeople: vi.fn(),
  assignFaceToPerson: vi.fn(),
  createPerson: vi.fn(),
}))

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
    displayDimensions: dimensions.value,
    naturalDimensions: dimensions.value,
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

vi.mock("@/api/endpoints", () => endpoints)

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
    dimensions.value = null
    endpoints.fetchImageFaces.mockReset()
    endpoints.fetchImageFaces.mockResolvedValue({ faces: [] })
    endpoints.fetchPeople.mockReset()
    endpoints.fetchPeople.mockResolvedValue({ people: [] })
    endpoints.fetchPersonSuggestions.mockReset()
    endpoints.fetchPersonSuggestions.mockResolvedValue({ people: [] })
    endpoints.renamePerson.mockReset()
    endpoints.mergePeople.mockReset()
    endpoints.assignFaceToPerson.mockReset()
    endpoints.createPerson.mockReset()
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

  it("places the People tab immediately before the Tags tab", () => {
    render(<UnifiedLightbox imagePath="a.jpg" initialMode="ai" onClose={vi.fn()} />)

    const labels = within(screen.getByTestId("lightbox-tab-bar"))
      .getAllByRole("tab")
      .map((tab) => tab.textContent)

    expect(labels).toContain("lightbox.tab.people")
    expect(labels.indexOf("lightbox.tab.people")).toBe(labels.indexOf("lightbox.tab.tags") - 1)
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
    const modes = ["ai", "exif", "ocr", "people", "tags"] as const
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

  it("renames a person from the lightbox people list and refreshes the photo", async () => {
    endpoints.fetchImageFaces
      .mockResolvedValueOnce({ faces: [personFace(1, 7, "Anna")] })
      .mockResolvedValueOnce({ faces: [personFace(1, 7, "Василиса")] })
    endpoints.renamePerson.mockResolvedValue({
      id: 7,
      name: "Василиса",
      photoCount: 1,
      coverThumbnail: "",
    })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText("Anna")).toBeTruthy())
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getByTestId("people-list-rename"))

    await waitFor(() => expect(screen.getByLabelText("people.renameNameLabel")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("people.renameNameLabel"), {
      target: { value: "Василиса" },
    })
    fireEvent.click(document.querySelector('[data-slot="rename-save"]') as HTMLElement)

    await waitFor(() => expect(endpoints.renamePerson).toHaveBeenCalledWith(7, "Василиса"))
    await waitFor(() => expect(endpoints.fetchImageFaces).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.getByText("Василиса")).toBeTruthy())
  })

  it("reassigns a face and refreshes the boxes and the people list", async () => {
    dimensions.value = { width: 200, height: 100 }
    endpoints.fetchImageFaces
      .mockResolvedValueOnce({ faces: [personFace(1, 7, "Anna"), personFace(2, 8, "Boris")] })
      .mockResolvedValueOnce({
        faces: [personFace(1, 9, "Василиса"), personFace(2, 8, "Boris")],
      })
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 7, name: "Anna", photoCount: 1, coverThumbnail: "" },
        { id: 8, name: "Boris", photoCount: 1, coverThumbnail: "" },
        { id: 9, name: "Василиса", photoCount: 1, coverThumbnail: "" },
      ],
    })
    endpoints.assignFaceToPerson.mockResolvedValue(personFace(1, 9, "Василиса"))

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getAllByTestId("people-face-box")).toHaveLength(2))
    fireEvent.click(screen.getAllByTestId("people-face-box")[0])

    await waitFor(() => expect(screen.getByText("lightbox.people.faceTitle")).toBeTruthy())
    fireEvent.click(document.querySelector('[data-slot="face-assign"]') as HTMLElement)

    // The picker lists the whole catalog, including a person absent from the photo.
    await waitFor(() => expect(screen.getByText("Василиса")).toBeTruthy())
    fireEvent.click(screen.getByText("Василиса"))

    await waitFor(() => expect(endpoints.assignFaceToPerson).toHaveBeenCalledWith(1, 9))
    await waitFor(() => expect(endpoints.fetchImageFaces).toHaveBeenCalledTimes(2))
    await waitFor(() => expect(screen.queryByText("Anna")).toBeNull())
    expect(screen.getByText("Василиса")).toBeTruthy()
  })

  it("detaches a face, mutes its box and compacts the people list", async () => {
    dimensions.value = { width: 200, height: 100 }
    endpoints.fetchImageFaces
      .mockResolvedValueOnce({ faces: [personFace(1, 7, "Anna"), personFace(2, 8, "Boris")] })
      .mockResolvedValueOnce({ faces: [personFace(1, 0, ""), personFace(2, 8, "Boris")] })
    endpoints.assignFaceToPerson.mockResolvedValue(personFace(1, 0, ""))

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getAllByTestId("people-face-box")).toHaveLength(2))
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getAllByTestId("people-face-box")[0])

    await waitFor(() => expect(screen.getByText("lightbox.people.faceTitle")).toBeTruthy())
    fireEvent.click(document.querySelector('[data-slot="face-detach"]') as HTMLElement)

    await waitFor(() => expect(endpoints.assignFaceToPerson).toHaveBeenCalledWith(1, 0))
    await waitFor(() =>
      expect(
        screen.getAllByTestId("people-face-box")[0].getAttribute("data-numbered"),
      ).toBe("false"),
    )

    const items = screen.getAllByTestId("people-list-item")
    expect(items).toHaveLength(1)
    expect(items[0].textContent).toContain("Boris")
    expect(items[0].textContent?.charAt(0)).toBe("1")
  })

  it("attaches an unassigned face to an existing person", async () => {
    dimensions.value = { width: 200, height: 100 }
    endpoints.fetchImageFaces
      .mockResolvedValueOnce({ faces: [personFace(1, 0, "")] })
      .mockResolvedValueOnce({ faces: [personFace(1, 7, "Anna")] })
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 7, name: "Anna", photoCount: 1, coverThumbnail: "" }],
    })
    endpoints.assignFaceToPerson.mockResolvedValue(personFace(1, 7, "Anna"))

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getAllByTestId("people-face-box")).toHaveLength(1))
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getAllByTestId("people-face-box")[0])

    await waitFor(() => expect(screen.getByText("lightbox.people.faceTitle")).toBeTruthy())
    fireEvent.click(document.querySelector('[data-slot="face-assign"]') as HTMLElement)

    await waitFor(() => expect(screen.getByText("Anna")).toBeTruthy())
    fireEvent.click(screen.getByText("Anna"))

    await waitFor(() => expect(endpoints.assignFaceToPerson).toHaveBeenCalledWith(1, 7))
    await waitFor(() =>
      expect(screen.getAllByTestId("people-face-box")[0].getAttribute("data-numbered")).toBe("true"),
    )
    expect(screen.getAllByTestId("people-list-item")[0].textContent).toContain("Anna")
  })

  it("creates a new person from an unassigned face in the lightbox", async () => {
    dimensions.value = { width: 200, height: 100 }
    endpoints.fetchImageFaces
      .mockResolvedValueOnce({ faces: [personFace(1, 0, "")] })
      .mockResolvedValueOnce({ faces: [personFace(1, 9, "Василиса")] })
    endpoints.createPerson.mockResolvedValue({
      id: 9,
      name: "Василиса",
      photoCount: 1,
      coverThumbnail: "",
    })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getAllByTestId("people-face-box")).toHaveLength(1))
    fireEvent.click(screen.getAllByTestId("people-face-box")[0])

    await waitFor(() => expect(screen.getByText("lightbox.people.faceTitle")).toBeTruthy())
    fireEvent.click(document.querySelector('[data-slot="face-create"]') as HTMLElement)

    await waitFor(() => expect(screen.getByLabelText("people.createNameLabel")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("people.createNameLabel"), {
      target: { value: "Василиса" },
    })
    fireEvent.click(document.querySelector('[data-slot="create-confirm"]') as HTMLElement)

    await waitFor(() =>
      expect(endpoints.createPerson).toHaveBeenCalledWith({
        name: "Василиса",
        faceIds: [1],
      }),
    )
    await waitFor(() =>
      expect(screen.getAllByTestId("people-face-box")[0].getAttribute("data-numbered")).toBe("true"),
    )
    expect(screen.getAllByTestId("people-list-item")[0].textContent).toContain("Василиса")
  })

  it("refuses to create a person with a name another person already has", async () => {
    dimensions.value = { width: 200, height: 100 }
    endpoints.fetchImageFaces.mockResolvedValue({ faces: [personFace(1, 0, "")] })
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 3, name: "Василиса", photoCount: 12, coverThumbnail: "" }],
    })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(endpoints.fetchPeople).toHaveBeenCalled())
    await waitFor(() => expect(screen.getAllByTestId("people-face-box")).toHaveLength(1))
    fireEvent.click(screen.getAllByTestId("people-face-box")[0])

    await waitFor(() => expect(screen.getByText("lightbox.people.faceTitle")).toBeTruthy())
    fireEvent.click(document.querySelector('[data-slot="face-create"]') as HTMLElement)

    await waitFor(() => expect(screen.getByLabelText("people.createNameLabel")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("people.createNameLabel"), {
      target: { value: "василиса" },
    })
    fireEvent.click(document.querySelector('[data-slot="create-confirm"]') as HTMLElement)

    await waitFor(() => expect(document.querySelector('[data-slot="name-conflict"]')).toBeTruthy())
    expect(endpoints.createPerson).not.toHaveBeenCalled()
  })

  it("asks before renaming a person to a name another person already has", async () => {
    endpoints.fetchImageFaces.mockResolvedValue({ faces: [personFace(1, 7, "Anna")] })
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 3, name: "Василиса", photoCount: 12, coverThumbnail: "" }],
    })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText("Anna")).toBeTruthy())
    await waitFor(() => expect(endpoints.fetchPeople).toHaveBeenCalled())
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getByTestId("people-list-rename"))

    await waitFor(() => expect(screen.getByLabelText("people.renameNameLabel")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("people.renameNameLabel"), {
      target: { value: " василиса " },
    })
    fireEvent.click(document.querySelector('[data-slot="rename-save"]') as HTMLElement)

    await waitFor(() => expect(document.querySelector('[data-slot="name-conflict"]')).toBeTruthy())
    expect(endpoints.renamePerson).not.toHaveBeenCalled()
  })

  it("shows suggestion chips as soon as the rename dialog opens", async () => {
    // The full listing never resolves: the suggestions must not wait for it.
    endpoints.fetchPeople.mockReturnValue(new Promise(() => {}))
    endpoints.fetchPersonSuggestions.mockResolvedValue({
      people: [
        { id: 20, name: "Иван", photoCount: 30, coverThumbnail: "" },
        { id: 21, name: "Анна", photoCount: 25, coverThumbnail: "" },
      ],
    })
    endpoints.fetchImageFaces.mockResolvedValue({ faces: [personFace(1, 7, "")] })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText("people.unnamed")).toBeTruthy())
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getByTestId("people-list-rename"))

    await waitFor(() =>
      expect(document.querySelectorAll('[data-slot="suggestion-chip"]')).toHaveLength(2),
    )
    expect(endpoints.fetchPersonSuggestions).toHaveBeenCalledTimes(1)
  })

  it("offers the merge when a suggested name is saved", async () => {
    // An unnamed person, so the dialog opens with an empty input and the
    // suggestions start from the most-photographed catalog names.
    endpoints.fetchImageFaces.mockResolvedValue({ faces: [personFace(1, 7, "")] })
    endpoints.fetchPeople.mockResolvedValue({
      people: [
        { id: 20, name: "Иван", photoCount: 30, coverThumbnail: "" },
        { id: 21, name: "Анна", photoCount: 25, coverThumbnail: "" },
        { id: 22, name: "Борис", photoCount: 20, coverThumbnail: "" },
        { id: 23, name: "Вера", photoCount: 15, coverThumbnail: "" },
        { id: 24, name: "Глеб", photoCount: 10, coverThumbnail: "" },
        { id: 25, name: "Дина", photoCount: 5, coverThumbnail: "" },
        { id: 26, name: "Егор", photoCount: 3, coverThumbnail: "" },
        { id: 27, name: "Жанна", photoCount: 1, coverThumbnail: "" },
      ],
    })
    endpoints.mergePeople.mockResolvedValue({
      id: 20,
      name: "Иван",
      photoCount: 31,
      coverThumbnail: "",
    })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText("people.unnamed")).toBeTruthy())
    await waitFor(() => expect(endpoints.fetchPeople).toHaveBeenCalled())
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getByTestId("people-list-rename"))

    await waitFor(() => expect(screen.getByLabelText("people.renameNameLabel")).toBeTruthy())
    await waitFor(() =>
      expect(document.querySelectorAll('[data-slot="suggestion-chip"]')).toHaveLength(7),
    )
    const suggestions = document.querySelectorAll('[data-slot="suggestion-chip"]')
    // Seven of the eight catalog people, the one with the most photos first.
    expect(suggestions[0].textContent).toContain("Иван")
    fireEvent.click(suggestions[0] as HTMLElement)

    expect((screen.getByLabelText("people.renameNameLabel") as HTMLInputElement).value).toBe("Иван")
    expect(endpoints.renamePerson).not.toHaveBeenCalled()

    fireEvent.click(document.querySelector('[data-slot="rename-save"]') as HTMLElement)

    await waitFor(() => expect(document.querySelector('[data-slot="name-conflict"]')).toBeTruthy())
    expect(endpoints.renamePerson).not.toHaveBeenCalled()
    fireEvent.click(document.querySelector('[data-slot="name-conflict-merge"]') as HTMLElement)

    await waitFor(() =>
      expect(endpoints.mergePeople).toHaveBeenCalledWith({ targetPersonId: 20, sourcePersonIds: [7] }),
    )
  })

  it("folds the renamed cluster into the holder from the conflict dialog", async () => {
    endpoints.fetchImageFaces.mockResolvedValue({ faces: [personFace(1, 7, "Anna")] })
    endpoints.fetchPeople.mockResolvedValue({
      people: [{ id: 3, name: "Василиса", photoCount: 12, coverThumbnail: "" }],
    })
    endpoints.mergePeople.mockResolvedValue({
      id: 3,
      name: "Василиса",
      photoCount: 13,
      coverThumbnail: "",
    })

    render(<UnifiedLightbox imagePath="a.jpg" initialMode="people" onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText("Anna")).toBeTruthy())
    await waitFor(() => expect(endpoints.fetchPeople).toHaveBeenCalled())
    fireEvent.click(screen.getByTestId("mobile-detail-toggle"))
    fireEvent.click(screen.getByTestId("people-list-rename"))

    await waitFor(() => expect(screen.getByLabelText("people.renameNameLabel")).toBeTruthy())
    fireEvent.change(screen.getByLabelText("people.renameNameLabel"), {
      target: { value: "василиса" },
    })
    fireEvent.click(document.querySelector('[data-slot="rename-save"]') as HTMLElement)

    await waitFor(() => expect(document.querySelector('[data-slot="name-conflict"]')).toBeTruthy())
    fireEvent.click(document.querySelector('[data-slot="name-conflict-merge"]') as HTMLElement)

    await waitFor(() =>
      expect(endpoints.mergePeople).toHaveBeenCalledWith({ targetPersonId: 3, sourcePersonIds: [7] }),
    )
    expect(endpoints.renamePerson).not.toHaveBeenCalled()
    await waitFor(() => expect(document.querySelector('[data-slot="name-conflict"]')).toBeNull())
  })
})

function personFace(id: number, personId: number, personName: string) {
  return {
    id,
    personId,
    personName,
    x: 10,
    y: 10,
    width: 50,
    height: 50,
    confidence: 0.9,
  }
}
