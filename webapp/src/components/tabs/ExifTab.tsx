import { useState } from "react"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import { ExifFoldersView } from "@/components/gallery/ExifFoldersView"
import type { GalleryImageDTO } from "@/types"

export function ExifTab() {
  const [selectedImagePath, setSelectedImagePath] = useState<string | null>(null)
  const [showGeoForm, setShowGeoForm] = useState(false)

  const handleImageClick = (image: GalleryImageDTO) => {
    setSelectedImagePath(image.path)
  }

  const handleAddGeo = (image: GalleryImageDTO) => {
    setSelectedImagePath(image.path)
    setShowGeoForm(true)
  }

  return (
    <>
      <ExifFoldersView
        onImageClick={handleImageClick}
        onImageDownload={(image) => {
          const link = document.createElement("a")
          link.href = `/api/image?path=${encodeURIComponent(image.path)}`
          link.download = image.fileName
          link.click()
        }}
        onImageDelete={(image, removeThumbnail) => {
          window.dispatchEvent(
            new CustomEvent("delete-image", {
              detail: { imagePath: image.path, removeThumbnail },
            })
          )
        }}
        onAddGeo={handleAddGeo}
      />

      <UnifiedLightbox
        imagePath={selectedImagePath}
        initialMode="exif"
        onClose={() => {
          setSelectedImagePath(null)
          setShowGeoForm(false)
        }}
        showGeoForm={showGeoForm}
        onShowGeoFormChange={setShowGeoForm}
      />
    </>
  )
}
