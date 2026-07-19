import { useState, useEffect, useRef, useCallback } from "react"
import { fetchOcrData, fetchLlmRecognition, recognizeWithLlm, fetchLlmRecognizeStatus } from "@/api/endpoints"
import type { OcrDataResponse, LlmOcrDataResponse } from "@/types"

interface UseOcrStateReturn {
  ocrData: OcrDataResponse | null
  llmData: LlmOcrDataResponse | null
  loading: boolean
  recognizing: boolean
  resetState: () => void
  handleRecognize: () => void
}

export function useOcrState(imagePath: string | null): UseOcrStateReturn {
  const [ocrData, setOcrData] = useState<OcrDataResponse | null>(null)
  const [llmData, setLlmData] = useState<LlmOcrDataResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [recognizing, setRecognizing] = useState(false)
  const prevImagePath = useRef<string | null>(null)
  const pollingRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const recognizingRef = useRef(false)

  const resetState = useCallback(() => {
    setOcrData(null)
    setLlmData(null)
    setLoading(false)
    prevImagePath.current = null
  }, [])

  // Load OCR data when lightbox opens
  useEffect(() => {
    if (!imagePath) return

    if (prevImagePath.current === imagePath) return
    prevImagePath.current = imagePath

    let isMounted = true
    setLoading(true)

    Promise.all([
      fetchOcrData(imagePath).catch(() => null),
      fetchLlmRecognition(imagePath).catch(() => null),
    ]).then(([ocr, llm]) => {
      if (isMounted) {
        setOcrData(ocr)
        setLlmData(llm)
        setLoading(false)
      }
    }).catch(() => {
      if (isMounted) {
        setLoading(false)
      }
    })

    return () => {
      isMounted = false
    }
  }, [imagePath])

  // Stop polling on unmount or image change
  useEffect(() => {
    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current)
        pollingRef.current = null
      }
    }
  }, [imagePath])

  const handleRecognize = useCallback(() => {
    if (!imagePath || recognizingRef.current) return

    recognizingRef.current = true
    const hasExistingResult = llmData?.found && llmData.success
    setRecognizing(true)

    if (pollingRef.current) {
      clearInterval(pollingRef.current)
      pollingRef.current = null
    }

    recognizeWithLlm({ imagePath, force: hasExistingResult || undefined })
      .then((result) => {
        if (result.status === "completed" && result.markdownContent) {
          setLlmData({
            found: true,
            markdownContent: result.markdownContent,
            language: result.language,
            provider: result.provider,
            model: result.model,
            processingTimeMs: result.processingTimeMs,
            success: true,
          })
          setRecognizing(false)
          recognizingRef.current = false
          return
        }

        const currentPath = imagePath
        pollingRef.current = setInterval(() => {
          fetchLlmRecognizeStatus(currentPath)
            .then((status) => {
              if (status.status === "completed") {
                if (pollingRef.current) {
                  clearInterval(pollingRef.current)
                  pollingRef.current = null
                }
                setLlmData({
                  found: true,
                  markdownContent: status.markdownContent,
                  language: status.language,
                  provider: status.provider,
                  model: status.model,
                  processingTimeMs: status.processingTimeMs,
                  success: true,
                })
                setRecognizing(false)
                recognizingRef.current = false
              } else if (status.status === "failed") {
                if (pollingRef.current) {
                  clearInterval(pollingRef.current)
                  pollingRef.current = null
                }
                setLlmData({
                  found: false,
                  error: status.error,
                  success: false,
                })
                setRecognizing(false)
                recognizingRef.current = false
              }
            })
            .catch((pollErr) => {
              if (pollingRef.current) {
                clearInterval(pollingRef.current)
                pollingRef.current = null
              }
              setLlmData({
                found: false,
                error: pollErr instanceof Error ? pollErr.message : String(pollErr),
                success: false,
              })
              setRecognizing(false)
              recognizingRef.current = false
            })
        }, 2000)
      })
      .catch((err) => {
        console.error("LLM recognition failed:", err)
        const errorMessage = err instanceof Error ? err.message : String(err)
        setLlmData({
          found: false,
          error: errorMessage,
          success: false,
        })
        setRecognizing(false)
        recognizingRef.current = false
      })
  }, [imagePath, llmData])

  return { ocrData, llmData, loading, recognizing, resetState, handleRecognize }
}
