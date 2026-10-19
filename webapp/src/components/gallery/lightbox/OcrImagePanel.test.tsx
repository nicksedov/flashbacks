import { createRef } from "react"
import { describe, it, expect, vi } from "vitest"
import { render, screen } from "@testing-library/react"
import { I18nProvider } from "@/i18n/I18nProvider"
import { OcrImagePanel } from "./OcrImagePanel"
import { hasClass } from "@/test/dom"
import type { OcrDataResponse } from "@/types"

const textData: OcrDataResponse = {
  imagePath: "/gallery/scan.jpg",
  angle: 0,
  scaleFactor: 1,
  isTextDocument: true,
  boundingBoxWidth: 200,
  boundingBoxHeight: 100,
  boxes: [{ x: 20, y: 10, width: 40, height: 20, word: "hi", confidence: 0.9 }],
}

interface RenderOptions {
  ocrData?: OcrDataResponse | null
  isRotated?: boolean
  displayDimensions?: { width: number; height: number } | null
  imageLoaded?: boolean
}

function renderPanel({
  ocrData = textData,
  isRotated = false,
  displayDimensions = { width: 100, height: 50 },
  imageLoaded = true,
}: RenderOptions = {}) {
  const panel = (dimensions = displayDimensions) => (
    <I18nProvider language="en">
      <OcrImagePanel
        imageUrl="/gallery/scan.jpg"
        ocrData={ocrData}
        isTextDocument={ocrData?.isTextDocument ?? false}
        loading={false}
        isRotated={isRotated}
        imageRef={createRef<HTMLImageElement>()}
        displayDimensions={dimensions}
        imageLoaded={imageLoaded}
        handleImageLoad={vi.fn()}
      />
    </I18nProvider>
  )

  return render(panel())
}

describe("OcrImagePanel", () => {
  it("fits an image with recognized text entirely within the visible area", () => {
    const { container } = renderPanel()

    const image = screen.getByAltText("Full size preview")
    // max-width/max-height bound the image to the stage, object-contain keeps
    // the aspect ratio, so no part of it can extend past the visible area.
    expect(hasClass(image, "max-w-full", "max-h-full", "object-contain")).toBe(true)

    const stage = container.firstElementChild
    expect(hasClass(stage, "h-full")).toBe(true)
    expect(hasClass(stage, "relative")).toBe(true)
  })

  it("fits an image without recognized text entirely within the visible area", () => {
    renderPanel({ ocrData: { ...textData, isTextDocument: false, boxes: [] } })

    const image = screen.getByAltText("Full size preview")
    expect(hasClass(image, "max-w-full", "max-h-full", "object-contain")).toBe(true)
    expect(screen.queryByTitle(/hi \(90%\)/)).toBeNull()
  })

  it("fits a rotated image entirely within the visible area", () => {
    renderPanel({ isRotated: true })

    const image = screen.getByAltText("Full size preview")
    expect(hasClass(image, "max-w-full", "max-h-full", "object-contain")).toBe(true)
    expect(image.getAttribute("src")).toBe("/gallery/scan.jpg")
  })

  it("keeps the bounding boxes aligned with the scaled image", () => {
    renderPanel({ displayDimensions: { width: 100, height: 50 } })

    const stage = screen.getByTitle("hi (90%)").parentElement as HTMLElement
    expect(stage.style.width).toBe("100px")
    expect(stage.style.height).toBe("50px")

    const box = screen.getByTitle("hi (90%)") as HTMLElement
    // scaleX = 100 / 200 = 0.5, scaleY = 50 / 100 = 0.5
    expect(box.style.left).toBe("10px")
    expect(box.style.top).toBe("5px")
    expect(box.style.width).toBe("20px")
    expect(box.style.height).toBe("10px")
  })

  it("keeps the bounding boxes aligned after a resize", () => {
    const view = renderPanel({ displayDimensions: { width: 100, height: 50 } })
    expect((screen.getByTitle("hi (90%)") as HTMLElement).style.left).toBe("10px")

    view.rerender(
      <I18nProvider language="en">
        <OcrImagePanel
          imageUrl="/gallery/scan.jpg"
          ocrData={textData}
          isTextDocument
          loading={false}
          imageRef={createRef<HTMLImageElement>()}
          displayDimensions={{ width: 300, height: 150 }}
          imageLoaded
          handleImageLoad={vi.fn()}
        />
      </I18nProvider>
    )

    const stage = screen.getByTitle("hi (90%)").parentElement as HTMLElement
    expect(stage.style.width).toBe("300px")
    expect(stage.style.height).toBe("150px")

    const box = screen.getByTitle("hi (90%)") as HTMLElement
    // scaleX = 300 / 200 = 1.5, scaleY = 150 / 100 = 1.5
    expect(box.style.left).toBe("30px")
    expect(box.style.top).toBe("15px")
    expect(box.style.width).toBe("60px")
    expect(box.style.height).toBe("30px")
  })
})
