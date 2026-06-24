import { useCallback, useEffect, useState } from "react"
import { fetchFolders, addFolder, removeFolder } from "@/api/endpoints"
import type { GalleryFolderDTO, AddFolderResponse, RemoveFolderResponse } from "@/types"

export function useGalleryFolders() {
  const [folders, setFolders] = useState<GalleryFolderDTO[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

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

  useEffect(() => {
    load()
  }, [load])

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
