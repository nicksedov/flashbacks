import { useCallback, useEffect, useState } from "react"
import { fetchFolders, addFolder, removeFolder } from "@/api/endpoints"
import type { GalleryFolderDTO, AddFolderResponse, RemoveFolderResponse } from "@/types"

export function useGalleryFolders() {
  const [folders, setFolders] = useState<GalleryFolderDTO[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const result = await fetchFolders()
        if (cancelled) return
        setFolders(result.folders)
        setError(null)
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : "Failed to load folders")
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const load = useCallback(async () => {
    setIsLoading(true)
    setError(null)
    try {
      const result = await fetchFolders()
      setFolders(result.folders)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load folders")
    } finally {
      setIsLoading(false)
    }
  }, [])

  const add = useCallback(
    async (path: string): Promise<AddFolderResponse> => {
      const result = await addFolder({ path })
      await load()
      return result
    },
    [load]
  )

  const remove = useCallback(
    async (id: number): Promise<RemoveFolderResponse> => {
      const result = await removeFolder(id)
      await load()
      return result
    },
    [load]
  )

  return { folders, isLoading, error, refetch: load, add, remove }
}
