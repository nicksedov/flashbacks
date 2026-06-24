import { useState, useCallback, useRef } from "react"
import { smartSearch } from "@/api/endpoints"
import type { SmartSearchResult } from "@/types"

export function useSmartSearch() {
  const [results, setResults] = useState<SmartSearchResult[]>([])
  const [total, setTotal] = useState(0)
  const [query, setQuery] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searched, setSearched] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  const search = useCallback(async (q: string, limit = 20) => {
    if (!q.trim()) {
      setResults([])
      setTotal(0)
      setQuery("")
      setSearched(false)
      return
    }

    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller

    setIsLoading(true)
    setError(null)
    setQuery(q.trim())
    setSearched(true)

    try {
      const response = await smartSearch(q.trim(), limit, controller.signal)
      setResults(response.images)
      setTotal(response.total)
    } catch (err) {
      if (err instanceof DOMException && err.name === "AbortError") {
        return
      }
      setError(err instanceof Error ? err.message : null)
      setResults([])
      setTotal(0)
    } finally {
      setIsLoading(false)
    }
  }, [])

  const reset = useCallback(() => {
    abortRef.current?.abort()
    setResults([])
    setTotal(0)
    setQuery("")
    setIsLoading(false)
    setError(null)
    setSearched(false)
  }, [])

  return {
    results,
    total,
    query,
    isLoading,
    error,
    searched,
    search,
    reset,
  }
}
