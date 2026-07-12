import { useCallback, useMemo, useRef, useState } from "react"

export interface PrefetchBuffer<R> {
  page: number
  promise: Promise<R> | null
  data: R | null
}

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
  /** Reset state and invalidate prefetch */
  reset: () => void
  /** Remove a specific item from the items list */
  removeItem: (key: string | number) => void
}

/**
 * Generic infinite scroll hook with prefetch support.
 * Handles pagination, duplicate prevention, and background prefetching.
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

  const prefetchRef = useRef<PrefetchBuffer<R>>({
    page: 0,
    promise: null,
    data: null,
  })

  const startPrefetch = useCallback(
    (page: number) => {
      const buf = prefetchRef.current
      if (buf.page === page && (buf.data || buf.promise)) {
        return // already prefetching/prefetched this page
      }
      buf.page = page
      buf.data = null
      buf.promise = fetchFn(page, pageSize)
        .then((result) => {
          if (prefetchRef.current.page === page) {
            prefetchRef.current.data = result
          }
          return result
        })
        .catch(() => {
          prefetchRef.current.promise = null
          return null as unknown as R
        })
    },
    [fetchFn, pageSize]
  )

  const consumePrefetch = useCallback((): R | null => {
    const buf = prefetchRef.current
    if (buf.page === pageRef.current && buf.data) {
      const data = buf.data
      buf.page = 0
      buf.data = null
      buf.promise = null
      return data
    }
    return null
  }, [])

  const isDuplicate = useCallback(
    (existingItems: T[], newItem: T): boolean => {
      if (compare) {
        return existingItems.some((item) => compare(item, newItem))
      }
      const newKey = keyExtractor(newItem)
      return existingItems.some((item) => keyExtractor(item) === newKey)
    },
    [compare, keyExtractor]
  )

  const loadMore = useCallback(async () => {
    if (isLoading) return
    setIsLoading(true)
    setError(null)
    // Capture current generation to detect stale requests
    const currentGeneration = generationRef.current
    try {
      const currentPage = pageRef.current
      const prefetched = consumePrefetch()
      const result = prefetched ?? (await fetchFn(currentPage, pageSize))

      // Abort if reset() was called during the fetch (generation changed)
      if (generationRef.current !== currentGeneration) {
        return
      }

      setItems((prev) => {
        const newItems = transform(result).filter((item) => !isDuplicate(prev, item))
        return [...prev, ...newItems]
      })

      if (responseTotal) {
        setTotal(responseTotal(result))
      }
      if (responseHasNext) {
        setHasMore(responseHasNext(result))
      }

      pageRef.current += 1
      setInitialized(true)

      if (!responseHasNext || responseHasNext(result)) {
        startPrefetch(pageRef.current)
      }
    } catch (err) {
      // Only set error if generation hasn't changed
      if (generationRef.current === currentGeneration) {
        setError(err instanceof Error ? err.message : "Failed to load data")
      }
    } finally {
      // Only clear loading if generation hasn't changed
      if (generationRef.current === currentGeneration) {
        setIsLoading(false)
      }
    }
  }, [isLoading, consumePrefetch, startPrefetch, fetchFn, pageSize, transform, responseTotal, responseHasNext, isDuplicate])

  const reset = useCallback(() => {
    setItems([])
    setTotal(0)
    setHasMore(true)
    setIsLoading(false)
    setError(null)
    pageRef.current = 1
    setInitialized(false)
    prefetchRef.current = { page: 0, promise: null, data: null }
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
