import { createRef } from "react"
import { describe, it, expect, vi } from "vitest"
import { fireEvent, render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { PeopleImagePanel } from "./PeopleImagePanel"
import type { ImageFaceDTO } from "@/types"
import type { ImagePerson } from "@/hooks/useImageFaces"

const anna: ImagePerson = { number: 1, personId: 7, name: "Anna" }
const unnamed: ImagePerson = { number: 2, personId: 9, name: "" }

function face(overrides: Partial<ImageFaceDTO> = {}): ImageFaceDTO {
  return {
    id: 1,
    personId: 7,
    personName: "Anna",
    x: 50,
    y: 25,
    width: 40,
    height: 20,
    confidence: 0.9,
    ...overrides,
  }
}

interface RenderOptions {
  faces?: ImageFaceDTO[]
  people?: ImagePerson[]
  displayDimensions?: { width: number; height: number } | null
  sourceDimensions?: { width: number; height: number } | null
  imageLoaded?: boolean
  loading?: boolean
  selectedFaceId?: number | null
  onSelectFace?: (face: ImageFaceDTO) => void
}

function renderPanel({
  faces = [face()],
  people = [anna],
  displayDimensions = { width: 400, height: 200 },
  sourceDimensions = { width: 200, height: 100 },
  imageLoaded = true,
  loading = false,
  selectedFaceId = null,
  onSelectFace,
}: RenderOptions = {}) {
  return render(
    <I18nProvider language="en">
      <PeopleImagePanel
        imageUrl="/api/image?path=%2Fgallery%2Fgroup.jpg"
        faces={faces}
        people={people}
        loading={loading}
        imageRef={createRef<HTMLImageElement>()}
        displayDimensions={displayDimensions}
        sourceDimensions={sourceDimensions}
        imageLoaded={imageLoaded}
        handleImageLoad={vi.fn()}
        selectedFaceId={selectedFaceId}
        onSelectFace={onSelectFace}
      />
    </I18nProvider>
  )
}

describe("PeopleImagePanel", () => {
  it("draws one box per detected face", () => {
    renderPanel({
      faces: [face({ id: 1 }), face({ id: 2 }), face({ id: 3, personId: 0, personName: "" })],
      people: [anna],
    })

    expect(screen.getAllByTestId("people-face-box")).toHaveLength(3)
  })

  it("labels a person's face with the person's number", () => {
    renderPanel({
      faces: [face({ id: 1, personId: 7 }), face({ id: 2, personId: 9, personName: "" })],
      people: [anna, unnamed],
    })

    const numbers = screen.getAllByTestId("people-face-number").map((badge) => badge.textContent)
    expect(numbers).toEqual(["1", "2"])
  })

  it("draws an unassigned face without a number", () => {
    renderPanel({
      faces: [face({ id: 5, personId: 0, personName: "" })],
      people: [],
    })

    const box = screen.getByTestId("people-face-box")
    expect(box.getAttribute("data-numbered")).toBe("false")
    expect(screen.queryByTestId("people-face-number")).toBeNull()
  })

  it("turns every box into a button that reports the click and its selected state", () => {
    const onSelectFace = vi.fn()
    renderPanel({
      faces: [face({ id: 1 }), face({ id: 2 })],
      people: [anna],
      selectedFaceId: 2,
      onSelectFace,
    })

    const boxes = screen.getAllByRole("button")
    expect(boxes).toHaveLength(2)
    expect(screen.getAllByTestId("people-face-box")[1].getAttribute("aria-pressed")).toBe("true")

    fireEvent.click(boxes[0])
    expect(onSelectFace).toHaveBeenCalledWith(expect.objectContaining({ id: 1 }))
  })

  it("scales every box with the rendered image", () => {
    renderPanel({
      faces: [face({ x: 50, y: 25, width: 40, height: 20 })],
      sourceDimensions: { width: 200, height: 100 },
      displayDimensions: { width: 400, height: 200 },
    })

    // scaleX = 400 / 200 = 2, scaleY = 200 / 100 = 2
    const box = screen.getByTestId("people-face-box")
    expect(box.style.left).toBe("100px")
    expect(box.style.top).toBe("50px")
    expect(box.style.width).toBe("80px")
    expect(box.style.height).toBe("40px")
    expect(screen.getByTestId("people-face-overlay").style.width).toBe("400px")
  })
})
