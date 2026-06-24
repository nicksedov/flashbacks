import { useState, useEffect, useRef, useCallback } from "react"
import { fetchImageTags, startAiAction, fetchAiActionStatus } from "@/api/endpoints"

export interface TagsStateData {
  tags: string[]
  provider?: string
  model?: string
  processingTimeMs?: number
}

interface UseTagsStateReturn {
  tagsData: TagsStateData | null
  loading: boolean
  generating: boolean
  error: string | null
  resetState: () => void
  handleGenerate: () => void
}

export function useTagsState(imagePath: string | null): UseTagsStateReturn {
  const [tagsData, setTagsData] = useState<TagsStateData | null>(null)
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const prevImagePath = useRef<string | null>(null)
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const generatingRef = useRef(false)

  const resetState = useCallback(() => {
    setTagsData(null)
    setLoading(false)
    setGenerating(false)
    setError(null)
    prevImagePath.current = null
    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }
    generatingRef.current = false
  }, [])

  // Check for existing tags when image changes (read-only, no generation)
  useEffect(() => {
    if (!imagePath) return

    if (prevImagePath.current === imagePath) return
    prevImagePath.current = imagePath

    setLoading(true)
    setError(null)
    setTagsData(null)

    fetchImageTags(imagePath)
      .then((response) => {
        if (response.tags.length > 0) {
          setTagsData({ tags: response.tags })
        }
        setLoading(false)
      })
      .catch(() => {
        setLoading(false)
      })
  }, [imagePath])

  // Cleanup polling on unmount
  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
        pollingRef.current = null
      }
    }
  }, [])

  const handleGenerate = useCallback(() => {
    if (!imagePath || generatingRef.current) return

    generatingRef.current = true
    setGenerating(true)
    setError(null)

    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }

    startAiAction({ imagePath, action: "tags", force: true })
      .then((startResponse) => {
        const taskId = startResponse.taskId

        pollingRef.current = setInterval(() => {
          fetchAiActionStatus(taskId)
            .then((status) => {
              if (status.status === "completed") {
                if (pollingRef.current) {
                  clearInterval(pollingRef.current)
                  pollingRef.current = null
                }
                const tags = status.tags ?? []
                setTagsData({
                  tags,
                  provider: status.provider,
                  model: status.model,
                  processingTimeMs: status.processingTimeMs,
                })
                setGenerating(false)
                generatingRef.current = false
              } else if (status.status === "failed") {
                if (pollingRef.current) {
                  clearInterval(pollingRef.current)
                  pollingRef.current = null
                }
                setError(status.error ?? "Tag generation failed")
                setGenerating(false)
                generatingRef.current = false
              }
              // "processing" status — keep polling
            })
            .catch(() => {
              if (pollingRef.current) {
                clearInterval(pollingRef.current)
                pollingRef.current = null
              }
              setError("Failed to check tag generation status")
              setGenerating(false)
              generatingRef.current = false
            })
        }, 2000)
      })
      .catch((err) => {
        setError(err.message ?? "Failed to start tag generation")
        setGenerating(false)
        generatingRef.current = false
      })
  }, [imagePath])

  return { tagsData, loading, generating, error, resetState, handleGenerate }
}
