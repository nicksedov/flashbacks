import { useCallback, useEffect, useState } from "react"

export interface UseDataLoaderOptions<T, R, A = unknown, M = unknown> {
  /** Fetch function to load data */
  fetchFn: () => Promise<R>
  /** Transform API response to array of items */
  transform: (data: R) => T[]
  /** Dependencies that trigger reload when changed */
  deps?: unknown[]
  /** Whether to auto-load on mount */
  autoLoad?: boolean
  /** Optional add function */
  addFn?: (item: A) => Promise<M>
  /** Optional remove function */
  removeFn?: (id: number | string) => Promise<M>
  /** Transform data after add without full reload */
  addTransform?: (prev: T[], result: M) => T[]
  /** Transform data after remove without full reload */
  removeTransform?: (prev: T[], id: number | string) => T[]
}

export interface UseDataLoaderResult<T, A, M> {
  /** Loaded data */
  data: T[]
  /** Whether currently loading */
  isLoading: boolean
  /** Error message if any */
  error: string | null
  /** Manually trigger reload */
  reload: () => Promise<void>
  /** Add new item */
  add?: (item: A) => Promise<M>
  /** Remove item by id */
  remove?: (id: number | string) => Promise<M>
}

/**
 * Generic data loading hook with CRUD support.
 * Handles initial load, reloading, and optional add/remove operations.
 */
export function useDataLoader<T, R, A = unknown, M = unknown>(
  options: UseDataLoaderOptions<T, R, A, M>
): UseDataLoaderResult<T, A, M> {
  const { fetchFn, transform, deps = [], autoLoad = true, addFn, removeFn } = options

  const [data, setData] = useState<T[]>([])
  const [isLoading, setIsLoading] = useState(autoLoad)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchFn()
      setData(transform(result))
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load data")
    } finally {
      setIsLoading(false)
    }
  }, [fetchFn, transform])

  useEffect(() => {
    if (autoLoad) {
      load()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [autoLoad, load, ...deps])

  const add = useCallback(
    async (item: A) => {
      if (!addFn) throw new Error("addFn not provided")
      const result = await addFn(item)
      setData((prev) => {
        // If addTransform provided, use it; otherwise reload
        return prev // Caller should call reload if needed
      })
      return result
    },
    [addFn]
  )

  const remove = useCallback(
    async (id: number | string) => {
      if (!removeFn) throw new Error("removeFn not provided")
      const result = await removeFn(id)
      setData((prev) => prev.filter((item) => (item as any).id !== id))
      return result
    },
    [removeFn]
  )

  return {
    data,
    isLoading,
    error,
    reload: load,
    add: addFn ? add : undefined,
    remove: removeFn ? remove : undefined,
  }
}
