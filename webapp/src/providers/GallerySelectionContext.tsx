import { useState, useCallback } from "react"
import {
  GallerySelectionContext,
  type RegisteredActions,
} from "./gallerySelectionContext"

export function GallerySelectionProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [actions, setActions] = useState<RegisteredActions | null>(null)

  const registerActions = useCallback((act: RegisteredActions | null) => {
    setActions(act)
  }, [])

  const clearSelection = useCallback(() => {
    actions?.clear()
  }, [actions])

  const deleteSelected = useCallback(() => {
    actions?.del()
  }, [actions])

  const moveSelected = useCallback(() => {
    actions?.move()
  }, [actions])

  const selectedCount = actions?.count ?? 0
  const isActive = selectedCount > 0

  return (
    <GallerySelectionContext.Provider
      value={{
        selectedCount,
        isActive,
        clearSelection,
        deleteSelected,
        moveSelected,
        registerActions,
      }}
    >
      {children}
    </GallerySelectionContext.Provider>
  )
}
