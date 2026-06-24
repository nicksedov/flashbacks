import { useCallback, useState, useRef, useEffect } from "react"

export interface UseAsyncOperationOptions {
  /** Whether to auto-execute on mount */
  autoExecute?: boolean
  /** Initial error state */
  initialError?: string | null
}

export interface UseAsyncOperationResult<T> {
  /** Result data */
  data: T | null
  /** Whether currently executing */
  isLoading: boolean
  /** Error message if any */
  error: string | null
  /** Execute the operation */
  execute: (...args: unknown[]) => Promise<T | void>
  /** Reset state */
  reset: () => void
}

/**
 * Hook for managing async operation state (loading, error, data).
 * Provides execute function with try-catch-finally pattern.
 */
export function useAsyncOperation<T>(
  asyncFn: (...args: unknown[]) => Promise<T>,
  options: UseAsyncOperationOptions = {}
): UseAsyncOperationResult<T> {
  const { autoExecute = false, initialError = null } = options

  const [data, setData] = useState<T | null>(null)
  const [isLoading, setIsLoading] = useState(autoExecute)
  const [error, setError] = useState<string | null>(initialError)

  const execute = useCallback(
    async (...args: unknown[]) => {
      setIsLoading(true)
      setError(null)
      try {
        const result = await asyncFn(...args)
        setData(result)
        return result
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Operation failed"
        setError(errorMessage)
        throw err
      } finally {
        setIsLoading(false)
      }
    },
    [asyncFn]
  )

  const reset = useCallback(() => {
    setData(null)
    setIsLoading(false)
    setError(null)
  }, [])

  return {
    data,
    isLoading,
    error,
    execute,
    reset,
  }
}

/**
 * Hook for synchronizing a value with a ref.
 * Useful for avoiding stale closures in callbacks.
 */
export function useSyncRef<T>(value: T): React.MutableRefObject<T> {
  const ref = useRef(value)

  useEffect(() => {
    ref.current = value
  }, [value])

  return ref
}

/**
 * Hook for managing array item removal.
 */
export function useArrayRemoval<T extends { id: string | number }>(
  setItems: (items: T[] | ((prev: T[]) => T[])) => void,
  keyExtractor: (item: T) => string | number = (item) => item.id
) {
  const removeItem = useCallback(
    (key: string | number) => {
      setItems((prev) => prev.filter((item) => keyExtractor(item) !== key))
    },
    [setItems, keyExtractor]
  )

  const removeItems = useCallback(
    (keys: (string | number)[]) => {
      const keySet = new Set(keys)
      setItems((prev) => prev.filter((item) => !keySet.has(keyExtractor(item))))
    },
    [setItems, keyExtractor]
  )

  return { removeItem, removeItems }
}
