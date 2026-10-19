import { describe, it, expect, vi, beforeEach } from "vitest"
import { renderHook, act } from "@testing-library/react"
import { useSmartSearch, MIN_QUERY_LENGTH } from "./useSmartSearch"
import { smartSearch } from "@/api/endpoints"
import type { SmartSearchResponse } from "@/types"

vi.mock("@/api/endpoints", () => ({
  smartSearch: vi.fn(),
}))

interface PendingRequest {
  query: string
  signal?: AbortSignal
  resolve: (value: SmartSearchResponse) => void
}

const emptyResponse = (query: string): SmartSearchResponse => ({
  images: [],
  total: 0,
  query,
})

const mockSmartSearch = vi.mocked(smartSearch)

describe("useSmartSearch", () => {
  beforeEach(() => {
    mockSmartSearch.mockReset()
  })

  it("ignores queries shorter than the minimum length", async () => {
    const { result } = renderHook(() => useSmartSearch())

    const shortQuery = "a".repeat(MIN_QUERY_LENGTH - 1)

    await act(async () => {
      await result.current.search(shortQuery)
    })

    expect(mockSmartSearch).not.toHaveBeenCalled()
    expect(result.current.searched).toBe(false)
    expect(result.current.results).toEqual([])
    expect(result.current.total).toBe(0)
    expect(result.current.query).toBe("")
    expect(result.current.isLoading).toBe(false)
  })

  it("aborts the previous request when a new search starts", async () => {
    const pending: PendingRequest[] = []

    mockSmartSearch.mockImplementation((query, _limit, signal) => {
      return new Promise<SmartSearchResponse>((resolve) => {
        pending.push({ query, signal, resolve })
      })
    })

    const { result } = renderHook(() => useSmartSearch())

    act(() => {
      void result.current.search("first query")
    })

    expect(pending).toHaveLength(1)
    const firstSignal = pending[0].signal
    expect(firstSignal).toBeDefined()
    expect(firstSignal?.aborted).toBe(false)

    act(() => {
      void result.current.search("second query")
    })

    expect(pending).toHaveLength(2)
    expect(pending[1].signal).not.toBe(firstSignal)
    expect(firstSignal?.aborted).toBe(true)
    expect(pending[1].signal?.aborted).toBe(false)

    // Settle both requests so no state update escapes the test.
    await act(async () => {
      pending[0].resolve(emptyResponse("first query"))
      pending[1].resolve(emptyResponse("second query"))
    })
  })
})
