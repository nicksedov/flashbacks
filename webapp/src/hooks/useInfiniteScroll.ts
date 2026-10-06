import { useCallback, useEffect, useMemo, useRef, useState } from "react"

export interface UseInfiniteScrollOptions<T, R> {
  /** Fetch function that takes page number and page size */
  fetchFn: (page: number, pageSize: number) => Promise<R>
  /** Page size for pagination */
  pageSize?: number
  /** Transform API response to array of items */
  transform: (response: R) => T[]
  /** Extract total count from response */
  responseTotal?: (response: R) => number
  /** Check if there's a next page */
  responseHasNext?: (response: R) => boolean
  /** Optional comparator to prevent duplicate items */
  compare?: (a: T, b: T) => boolean
  /** Optional key extractor for deduplication (default: "id") */
  keyExtractor?: (item: T) => string | number
}

export interface UseInfiniteScrollResult<T> {
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
  /** Load next page */
  loadMore: () => Promise<void>
  /** Reset state and invalidate in-flight requests */
  reset: () => void
  /** Remove a specific item from the items list */
  removeItem: (key: string | number) => void
}

/**
 * Generic infinite scroll hook.
 *
 * Handles pagination, duplicate prevention, and stale-request invalidation.
 *
 * Pagination is strictly on-demand: `loadMore()` fetches exactly one page
 * (starting at page 1) and only when explicitly called. There is intentionally
 * no eager next-page prefetch, so opening a view issues a single `page=1`
 * request and further pages are fetched only when the caller (e.g. an
 * IntersectionObserver sentinel) requests them.
 */
export function useInfiniteScroll<T, R>(
  options: UseInfiniteScrollOptions<T, R>
): UseInfiniteScrollResult<T> {
  const {
    fetchFn,
    pageSize = 50,
    transform,
    responseTotal,
    responseHasNext,
    compare,
    keyExtractor: optionsKeyExtractor,
  } = options

  // Stabilize keyExtractor to prevent downstream callback chain instability
  const keyExtractor = useMemo(
    () => optionsKeyExtractor ?? ((item: T) => (item as unknown as { id: string | number }).id),
    [optionsKeyExtractor]
  )

  const [items, setItems] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [hasMore, setHasMore] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const pageRef = useRef(1)
  const [initialized, setInitialized] = useState(false)
  // Generation counter to invalidate stale requests after reset
  const generationRef = useRef(0)

  // Synchronous re-entrancy guard. `isLoading` is React state and is therefore
  // not updated within the same tick, so relying on it lets concurrent triggers
  // (mount effect + IntersectionObserver, or React StrictMode double-invocation)
  // all pass the guard and issue duplicate fetches. A ref is updated immediately.
  const loadingRef = useRef(false)

  // Keep the latest options in refs so `loadMore` can stay referentially stable
  // without re-creating itself (and re-triggering dependents) on every render.
  const fetchFnRef = useRef(fetchFn)
  const pageSizeRef = useRef(pageSize)
  const transformRef = useRef(transform)
  const responseTotalRef = useRef(responseTotal)
  const responseHasNextRef = useRef(responseHasNext)
  const compareRef = useRef(compare)
  const keyExtractorRef = useRef(keyExtractor)
  useEffect(() => {
    fetchFnRef.current = fetchFn
    pageSizeRef.current = pageSize
    transformRef.current = transform
    responseTotalRef.current = responseTotal
    responseHasNextRef.current = responseHasNext
    compareRef.current = compare
    keyExtractorRef.current = keyExtractor
  })

  const isDuplicate = useCallback((existingItems: T[], newItem: T): boolean => {
    const compareFn = compareRef.current
    if (compareFn) {
      return existingItems.some((item) => compareFn(item, newItem))
    }
    const extract = keyExtractorRef.current
    const newKey = extract(newItem)
    return existingItems.some((item) => extract(item) === newKey)
  }, [])

  const loadMore = useCallback(async () => {
    // Synchronous guard: prevent concurrent / duplicate fetches.
    if (loadingRef.current) return
    loadingRef.current = true
    setIsLoading(true)
    setError(null)
    // Capture current generation to detect stale requests
    const currentGeneration = generationRef.current
    try {
      const currentPage = pageRef.current
      const result = await fetchFnRef.current(currentPage, pageSizeRef.current)

      // Abort if reset() was called during the fetch (generation changed)
      if (generationRef.current !== currentGeneration) {
        return
      }

      setItems((prev) => {
        const newItems = transformRef.current(result).filter((item) => !isDuplicate(prev, item))
        return [...prev, ...newItems]
      })

      if (responseTotalRef.current) {
        setTotal(responseTotalRef.current(result))
      }
      if (responseHasNextRef.current) {
        setHasMore(responseHasNextRef.current(result))
      }

      pageRef.current += 1
      setInitialized(true)
    } catch (err) {
      // Only set error if generation hasn't changed
      if (generationRef.current === currentGeneration) {
        setError(err instanceof Error ? err.message : "Failed to load data")
      }
    } finally {
      // Only clear loading if generation hasn't changed
      if (generationRef.current === currentGeneration) {
        loadingRef.current = false
        setIsLoading(false)
      }
    }
  }, [isDuplicate])

  const reset = useCallback(() => {
    setItems([])
    setTotal(0)
    setHasMore(true)
    setIsLoading(false)
    setError(null)
    pageRef.current = 1
    setInitialized(false)
    // Release the synchronous guard so a fresh load can start immediately.
    loadingRef.current = false
    // Increment generation to invalidate all in-flight requests
    generationRef.current += 1
  }, [])

  const removeItem = useCallback(
    (key: string | number) => {
      setItems((prev) => prev.filter((item) => keyExtractor(item) !== key))
      setTotal((prev) => Math.max(0, prev - 1))
    },
    [keyExtractor]
  )

  return {
    items,
    total,
    hasMore,
    isLoading,
    error,
    initialized,
    loadMore,
    reset,
    removeItem,
  }
}
