import { useEffect, useRef } from "react"

export interface UseIntersectionObserverOptions {
  /** Callback when element intersects */
  onIntersect: () => void
  /** Whether to observe (can disable temporarily) */
  enabled?: boolean
  /** IntersectionObserver threshold */
  threshold?: number
  /** Additional dependencies that should reset the observer */
  dependencies?: unknown[]
}

/**
 * Hook for creating an intersection observer with sentinel element.
 * Returns a ref that should be attached to the sentinel element.
 */
export function useIntersectionObserver({
  onIntersect,
  enabled = true,
  threshold = 0.1,
  dependencies = [],
}: UseIntersectionObserverOptions) {
  const sentinelRef = useRef<HTMLDivElement>(null)
  const onIntersectRef = useRef(onIntersect)

  // Keep ref in sync
  useEffect(() => {
    onIntersectRef.current = onIntersect
  }, [onIntersect])

  useEffect(() => {
    if (!enabled) return

    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          onIntersectRef.current()
        }
      },
      { threshold }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, threshold, ...dependencies])

  return sentinelRef
}

/**
 * Hook for managing pagination footer state.
 * Provides loading and "all loaded" indicators.
 */
export interface PaginationFooterProps {
  isLoading: boolean
  hasMore: boolean
  totalCount: number
  loadingText?: string
  allLoadedText?: string
}

export function usePaginationFooter({
  isLoading,
  hasMore,
  totalCount,
}: Pick<PaginationFooterProps, 'isLoading' | 'hasMore' | 'totalCount'>) {
  const showLoading = isLoading
  const showAllLoaded = !hasMore && totalCount > 0

  return { showLoading, showAllLoaded }
}
