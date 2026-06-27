import { createContext } from "react"

export interface RegisteredActions {
  count: number
  clear: () => void
  del: () => void
}

export interface GallerySelectionState {
  selectedCount: number
  isActive: boolean
  clearSelection: () => void
  deleteSelected: () => void
  registerActions: (actions: RegisteredActions | null) => void
}

export const GallerySelectionContext =
  createContext<GallerySelectionState | null>(null)
