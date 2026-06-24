import { useState, useEffect, useRef, useCallback } from "react"
import { searchGeocodeLocations } from "@/api/endpoints"
import type { GeocodeSearchResult } from "@/types"

export function useGeocodeSearch() {
  const [query, setQuery] = useState("")
  const [results, setResults] = useState<GeocodeSearchResult[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  useEffect(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
    }
    abortControllerRef.current?.abort()

    if (query.length < 2) {
      return
    }

    timerRef.current = setTimeout(async () => {
      const controller = new AbortController()
      abortControllerRef.current = controller

      setIsSearching(true)
      try {
        const response = await searchGeocodeLocations(query, controller.signal)
        setResults(response.results)
      } catch (err) {
        if (err instanceof Error && err.name === "AbortError") return
        setResults([])
      } finally {
        setIsSearching(false)
      }
    }, 300)

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current)
      }
      abortControllerRef.current?.abort()
    }
  }, [query])

  const clearResults = useCallback(() => {
    setResults([])
    setQuery("")
  }, [])

  return { query, setQuery, results, isSearching, clearResults }
}
