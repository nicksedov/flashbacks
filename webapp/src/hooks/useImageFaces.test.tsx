import { describe, it, expect, vi, beforeEach } from "vitest"
import { act, renderHook, waitFor } from "@testing-library/react"
import { numberPeople, useImageFaces } from "./useImageFaces"
import { fetchImageFaces } from "@/api/endpoints"
import type { ImageFaceDTO } from "@/types"

vi.mock("@/api/endpoints", () => ({
  fetchImageFaces: vi.fn(),
}))

const mockFetchImageFaces = vi.mocked(fetchImageFaces)

function face(overrides: Partial<ImageFaceDTO> = {}): ImageFaceDTO {
  return {
    id: 1,
    personId: 0,
    personName: "",
    x: 0,
    y: 0,
    width: 10,
    height: 10,
    confidence: 0.9,
    ...overrides,
  }
}

describe("numberPeople", () => {
  it("numbers each person once by first appearance and skips unassigned faces", () => {
    const people = numberPeople([
      face({ id: 1, personId: 7, personName: "Anna" }),
      face({ id: 2, personId: 0 }),
      face({ id: 3, personId: 9 }),
      face({ id: 4, personId: 7, personName: "Anna" }),
    ])

    expect(people).toEqual([
      { number: 1, personId: 7, name: "Anna" },
      { number: 2, personId: 9, name: "" },
    ])
  })
})

describe("useImageFaces", () => {
  beforeEach(() => {
    mockFetchImageFaces.mockReset()
  })

  it("excludes faces that belong to no person from the derived people list", async () => {
    mockFetchImageFaces.mockResolvedValue({
      faces: [face({ id: 1, personId: 5, personName: "Anna" }), face({ id: 2, personId: 0 })],
    })

    const { result } = renderHook(() => useImageFaces("/gallery/a.jpg", true))
    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.faces).toHaveLength(2)
    expect(result.current.people).toEqual([{ number: 1, personId: 5, name: "Anna" }])
  })

  it("does not request faces while the mode is inactive", () => {
    const { result } = renderHook(() => useImageFaces("/gallery/a.jpg", false))

    expect(mockFetchImageFaces).not.toHaveBeenCalled()
    expect(result.current.loading).toBe(false)
    expect(result.current.people).toEqual([])
  })

  it("reports an error and no faces when the request fails", async () => {
    mockFetchImageFaces.mockRejectedValue(new Error("boom"))

    const { result } = renderHook(() => useImageFaces("/gallery/a.jpg", true))
    await waitFor(() => expect(result.current.error).toBe(true))

    expect(result.current.faces).toEqual([])
    expect(result.current.loading).toBe(false)
  })

  it("re-reads the photo's faces when refresh is called", async () => {
    mockFetchImageFaces
      .mockResolvedValueOnce({
        faces: [face({ id: 1, personId: 5, personName: "Anna" })],
      })
      .mockResolvedValueOnce({
        faces: [face({ id: 1, personId: 0 })],
      })

    const { result } = renderHook(() => useImageFaces("/gallery/a.jpg", true))
    await waitFor(() => expect(result.current.people).toHaveLength(1))

    act(() => result.current.refresh())

    await waitFor(() => expect(result.current.people).toEqual([]))
    expect(mockFetchImageFaces).toHaveBeenCalledTimes(2)
    expect(result.current.faces[0]?.personId).toBe(0)
  })
})
