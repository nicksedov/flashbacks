import { useCallback, useEffect, useRef, useState } from "react"

export interface UseCursorInfiniteScrollOptions<T, R> {
  /** Fetch function that takes optional cursor and returns response */
  fetchFn: (cursor?: string) => Promise<R>
  /** Transform API response to array of items */
  transform: (response: R) => T[]
  /** Extract next cursor from response */
  responseNextCursor: (response: R) => string | null
  /** Extract total count from response */
  responseTotal?: (response: R) => number
  /** Optional merge function to combine new items with existing ones (e.g. merge same-date groups) */
  mergeFn?: (existing: T[], incoming: T[]) => T[]
}

export interface UseCursorInfiniteScrollResult<T> {
  /** Accumulated items */
  items: T[]
  /** Total count from last response */
  total: number
  /** Whether there are more pages */
  hasMore: boolean
  /** Whether currently loading */
  isLoading: boolean
  /** Error message if any */
  error: string | null
  /** Whether initial load completed */
  initialized: boolean
  /** Current cursor for next fetch */
  nextCursor: string | null
  /** Load next page */
  loadMore: () => Promise<void>
  /** Reset state */
  reset: () => void
  /** Remove a specific item from the items list */
  removeItem: (key: string | number) => void
  /** Set items directly (for advanced use cases like removal) */
  setItems: (updater: (prev: T[]) => T[]) => void
  /** Set total directly */
  setTotal: (updater: (prev: number) => number) => void
}

/**
 * Simplified infinite scroll hook using cursor-based pagination.
 * Uses ref-based loading guard and generation counter to prevent duplicate
 * page loads from stale closures and to invalidate in-flight requests on reset.
 */
export function useCursorInfiniteScroll<T, R>(
  options: UseCursorInfiniteScrollOptions<T, R>
): UseCursorInfiniteScrollResult<T> {
  const {
    fetchFn,
    transform,
    responseNextCursor,
    responseTotal,
    mergeFn,
  } = options

  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [initialized, setInitialized] = useState(false)
  const [nextCursor, setNextCursor] = useState<string | null>(null)
  const nextCursorRef = useRef<string | null>(null)

  // Synchronous loading guard — ref writes are immediate (no React batching),
  // so rapid consecutive calls correctly see the updated value.
  const loadingRef = useRef(false)
  // Generation counter to invalidate stale requests after reset().
  const generationRef = useRef(0)

  // Store options in refs so loadMore can access the latest version without
  // depending on unstable function references.
  const fetchFnRef = useRef(fetchFn)
  const transformRef = useRef(transform)
  const responseNextCursorRef = useRef(responseNextCursor)
  const responseTotalRef = useRef(responseTotal)
  const mergeFnRef = useRef(mergeFn)
  useEffect(() => {
    fetchFnRef.current = fetchFn
    transformRef.current = transform
    responseNextCursorRef.current = responseNextCursor
    responseTotalRef.current = responseTotal
    mergeFnRef.current = mergeFn
  })

  const loadMore = useCallback(async () => {
    if (loadingRef.current) return
    loadingRef.current = true
    setIsLoading(true)
    setError(null)
    const gen = generationRef.current
    try {
      const cursorArg = nextCursorRef.current ?? undefined
      const result = await fetchFnRef.current(cursorArg)

      // Discard response if reset() was called during the fetch
      if (generationRef.current !== gen) return

      const newItems = transformRef.current(result)
      if (mergeFnRef.current) {
        setItems((prev) => mergeFnRef.current!(prev, newItems))
      } else {
        setItems((prev) => [...prev, ...newItems])
      }

      const cursor = responseNextCursorRef.current(result)
      nextCursorRef.current = cursor
      setNextCursor(cursor)
      setHasMore(cursor !== null)

      if (responseTotalRef.current) {
        setTotal(responseTotalRef.current(result))
      }

      setInitialized(true)
    } catch (err) {
      if (generationRef.current === gen) {
        setError(err instanceof Error ? err.message : "Failed to load data")
      }
    } finally {
      if (generationRef.current === gen) {
        loadingRef.current = false
        setIsLoading(false)
      }
    }
  }, [])

  const reset = useCallback(() => {
    generationRef.current += 1
    loadingRef.current = false
    setItems([])
    setTotal(0)
    setHasMore(true)
    setIsLoading(false)
    setError(null)
    setInitialized(false)
    nextCursorRef.current = null
    setNextCursor(null)
  }, [])

  const removeItem = useCallback(
    (key: string | number) => {
      setItems((prev) => prev.filter((item) => (item as unknown as { id: string | number }).id !== key))
      setTotal((prev) => Math.max(0, prev - 1))
    },
    []
  )

  return {
    items,
    total,
    hasMore,
    isLoading,
    error,
    initialized,
    nextCursor,
    loadMore,
    reset,
    removeItem,
    setItems,
    setTotal,
  }
}
