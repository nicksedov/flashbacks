import { useContext } from "react"
import { GallerySelectionContext } from "./gallerySelectionContext"

export function useGallerySelection() {
  const context = useContext(GallerySelectionContext)
  if (!context) {
    throw new Error(
      "useGallerySelection must be used within a GallerySelectionProvider"
    )
  }
  return context
}
