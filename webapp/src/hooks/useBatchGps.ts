import { useState, useCallback, useRef } from "react"
import { batchUpdateGps } from "@/api/endpoints"
import type { BatchUpdateGpsResponse } from "@/types"

const BATCH_SIZE = 10

export interface BatchGpsProgress {
  /** Total number of images to process */
  total: number
  /** Number of images processed so far */
  processed: number
  /** Whether a batch operation is in progress */
  running: boolean
  /** Aggregated success count */
  success: number
  /** Aggregated failed count */
  failed: number
  /** Aggregated skipped count */
  skipped: number
  /** Last error message, if any */
  lastError?: string
}

interface BatchGpsResult {
  response: BatchUpdateGpsResponse | null
  /** Location name in local language (set once from first batch) */
  nameLocal: string
  /** Location name in English (set once from first batch) */
  nameEng: string
}

/**
 * Hook that performs batched GPS updates on the client side.
 * Splits the full path list into chunks of BATCH_SIZE,
 * calls batchUpdateGps for each chunk sequentially,
 * and reports progress along the way.
 */
export function useBatchGps() {
  const [progress, setProgress] = useState<BatchGpsProgress>({
    total: 0,
    processed: 0,
    running: false,
    success: 0,
    failed: 0,
    skipped: 0,
  })
  const [result, setResult] = useState<BatchGpsResult>({
    response: null,
    nameLocal: "",
    nameEng: "",
  })
  const abortRef = useRef(false)

  const run = useCallback(
    async (paths: string[], lat: number, lng: number) => {
      abortRef.current = false

      const total = paths.length
      setProgress({
        total,
        processed: 0,
        running: true,
        success: 0,
        failed: 0,
        skipped: 0,
      })
      setResult({ response: null, nameLocal: "", nameEng: "" })

      let success = 0
      let failed = 0
      let skipped = 0
      let nameLocal = ""
      let nameEng = ""
      let processed = 0
      let lastError: string | undefined

      // Split paths into batches
      for (let i = 0; i < total; i += BATCH_SIZE) {
        if (abortRef.current) {
          lastError = "Cancelled"
          break
        }

        const batch = paths.slice(i, i + BATCH_SIZE)
        try {
          const res = await batchUpdateGps({ paths: batch, lat, lng })
          success += res.success
          failed += res.failed
          skipped += res.skipped

          // Capture location names from first batch response
          if (i === 0) {
            nameLocal = res.nameLocal
            nameEng = res.nameEng
          }
        } catch (err) {
          failed += batch.length
          lastError = err instanceof Error ? err.message : String(err)
        }

        processed = Math.min(i + BATCH_SIZE, total)
        setProgress({
          total,
          processed,
          running: true,
          success,
          failed,
          skipped,
          lastError,
        })
      }

      // Final state
      setProgress({
        total,
        processed,
        running: false,
        success,
        failed,
        skipped,
        lastError,
      })
      setResult({
        response: { success, failed, skipped, nameLocal, nameEng, lat, lng },
        nameLocal,
        nameEng,
      })

      return { success, failed, skipped }
    },
    []
  )

  const cancel = useCallback(() => {
    abortRef.current = true
  }, [])

  const reset = useCallback(() => {
    setProgress({
      total: 0,
      processed: 0,
      running: false,
      success: 0,
      failed: 0,
      skipped: 0,
    })
    setResult({ response: null, nameLocal: "", nameEng: "" })
  }, [])

  return { progress, result, run, cancel, reset }
}
