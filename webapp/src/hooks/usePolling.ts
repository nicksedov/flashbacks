import { useCallback, useEffect, useRef, useState } from "react"

export interface UsePollingOptions<T> {
  /** Function to poll for data */
  pollFn: () => Promise<T>
  /** Polling interval in milliseconds */
  interval?: number
  /** Check if polling should stop */
  onCompleteCheck?: (data: T) => boolean
  /** Callback when polling completes */
  onComplete?: (data: T) => void
  /** Dependencies that trigger restart when changed */
  deps?: unknown[]
}

export interface UsePollingResult<T> {
  /** Latest polled data */
  data: T | null
  /** Whether currently polling */
  isPolling: boolean
  /** Error from last poll */
  error: string | null
  /** Start polling */
  start: () => void
  /** Stop polling */
  stop: () => void
  /** Set onComplete callback */
  setOnComplete: (callback: (data: T) => void) => void
}

/**
 * Generic polling hook with interval management.
 * Handles start/stop lifecycle and completion detection.
 */
export function usePolling<T>(options: UsePollingOptions<T>): UsePollingResult<T> {
  const { pollFn, interval = 2000, onCompleteCheck } = options

  const [data, setData] = useState<T | null>(null)
  const [isPolling, setIsPolling] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const onCompleteRef = useRef(options.onComplete ?? null)
  const wasCompleteRef = useRef(false)
  const isMountedRef = useRef(true)

  // Track mounted state
  useEffect(() => {
    isMountedRef.current = true
    return () => { isMountedRef.current = false }
  }, [])

  const poll = useCallback(async () => {
    try {
      const result = await pollFn()
      if (!isMountedRef.current) return
      setData(result)
      setError(null)

      if (onCompleteCheck && onCompleteCheck(result)) {
        if (!wasCompleteRef.current) {
          wasCompleteRef.current = true
          onCompleteRef.current?.(result)
        }
      }
    } catch (err) {
      if (!isMountedRef.current) return
      setError(err instanceof Error ? err.message : "Polling failed")
    }
  }, [pollFn, onCompleteCheck])

  const start = useCallback(() => {
    if (intervalRef.current) return
    setIsPolling(true)
    wasCompleteRef.current = false
    poll()
    intervalRef.current = setInterval(poll, interval)
  }, [poll, interval])

  const stop = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current)
      intervalRef.current = null
    }
    setIsPolling(false)
  }, [])

  const setOnComplete = useCallback((callback: (data: T) => void) => {
    onCompleteRef.current = callback
  }, [])

  // Auto-stop when complete
  useEffect(() => {
    if (isPolling && wasCompleteRef.current) {
      const timeout = setTimeout(stop, 2000)
      return () => clearTimeout(timeout)
    }
  }, [isPolling, stop])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
      }
    }
  }, [])

  return {
    data,
    isPolling,
    error,
    start,
    stop,
    setOnComplete,
  }
}
