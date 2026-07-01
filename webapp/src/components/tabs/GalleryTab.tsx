import { useCallback, useRef, useState } from "react"
import { GalleryFoldersView } from "@/components/gallery/GalleryFoldersView"
import { GalleryCalendarView } from "@/components/gallery/GalleryCalendarView"
import { GalleryGeolocationView } from "@/components/gallery/GalleryGeolocationView"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import { DeleteConfirmDialog } from "@/components/gallery/DeleteConfirmDialog"
import { BulkDeleteDialog } from "@/components/gallery/BulkDeleteDialog"
import type { LightboxMode } from "@/components/gallery/UnifiedLightbox"
import { deleteFiles } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"
import { downloadImage } from "@/lib/downloadImage"
import type { GalleryImageDTO } from "@/types"

interface GalleryTabProps {
  galleryMode: "folders" | "calendar" | "geolocation"
}

export function GalleryTab({ galleryMode }: GalleryTabProps) {
  const { trashDir } = useSettings()
  const { t } = useTranslation()
  const [lightboxImage, setLightboxImage] = useState<string | null>(null)
  const [lightboxMode, setLightboxMode] = useState<LightboxMode>("ai")
  const [showGeoForm, setShowGeoForm] = useState(false)

  // Single delete state
  const [deleteConfirm, setDeleteConfirm] = useState<{ fileName: string; path: string } | null>(null)
  const removeThumbnailRef = useRef<(() => void) | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  // Bulk delete state
  const [bulkDeleteImages, setBulkDeleteImages] = useState<GalleryImageDTO[] | null>(null)
  const [bulkDeleteCleanup, setBulkDeleteCleanup] = useState<(() => void) | null>(null)
  const [bulkUseTrash, setBulkUseTrash] = useState(true)

  const handleImageClick = useCallback((image: GalleryImageDTO) => {
    setLightboxImage(image.path)
    setLightboxMode("ai")
  }, [])

  const handleImageDownload = useCallback((image: GalleryImageDTO) => {
    downloadImage(image.path, image.fileName)
  }, [])

  const handleImageDelete = useCallback((image: GalleryImageDTO, removeThumbnail: () => void) => {
    setDeleteConfirm({ fileName: image.fileName, path: image.path })
    removeThumbnailRef.current = removeThumbnail
  }, [])

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteConfirm) return
    setIsDeleting(true)
    try {
      await deleteFiles({
        filePaths: [deleteConfirm.path],
        trashDir: trashDir || "",
      })
      removeThumbnailRef.current?.()
      removeThumbnailRef.current = null
    } catch (err) {
      console.error("Failed to delete file:", err)
      alert("Failed to delete file")
    } finally {
      setIsDeleting(false)
      setDeleteConfirm(null)
    }
  }, [deleteConfirm, trashDir])

  const handleBulkDeleteRequest = useCallback((selectedImages: GalleryImageDTO[], cleanup: () => void) => {
    setBulkDeleteImages(selectedImages)
    setBulkDeleteCleanup(() => cleanup)
    setBulkUseTrash(true)
  }, [])

  const handleConfirmBulkDelete = useCallback(async () => {
    if (!bulkDeleteImages || bulkDeleteImages.length === 0) return

    if (!bulkUseTrash || !trashDir) {
      if (!window.confirm(t("deleteFiles.confirmPermanent"))) {
        return
      }
    }

    setIsDeleting(true)
    try {
      const result = await deleteFiles({
        filePaths: bulkDeleteImages.map((img) => img.path),
        trashDir: bulkUseTrash ? trashDir : "",
      })
      setBulkDeleteImages(null)
      bulkDeleteCleanup?.()
      setBulkDeleteCleanup(null)
      if (result.failed > 0) {
        alert(t("deleteFiles.successWithFailed", { count: result.success, failed: result.failed }))
      }
    } catch (err) {
      console.error("Failed to delete files:", err)
      alert(t("deleteFiles.errorFailed"))
    } finally {
      setIsDeleting(false)
    }
  }, [bulkDeleteImages, bulkDeleteCleanup, bulkUseTrash, trashDir, t])

  return (
    <div className={galleryMode === "geolocation" ? "space-y-2" : "space-y-4"}>
      {galleryMode === "folders" ? (
        <GalleryFoldersView
          onImageClick={handleImageClick}
          onImageDownload={handleImageDownload}
          onImageDelete={handleImageDelete}
          onBulkDelete={handleBulkDeleteRequest}
        />
      ) : galleryMode === "calendar" ? (
        <GalleryCalendarView
          onImageClick={handleImageClick}
          onImageDownload={handleImageDownload}
          onImageDelete={handleImageDelete}
        />
      ) : (
        <GalleryGeolocationView
          onImageClick={handleImageClick}
          onImageDownload={handleImageDownload}
          onImageDelete={handleImageDelete}
        />
      )}

      <UnifiedLightbox
        imagePath={lightboxImage}
        initialMode={lightboxMode}
        onClose={() => {
          setLightboxImage(null)
          setShowGeoForm(false)
        }}
        showGeoForm={showGeoForm}
        onShowGeoFormChange={setShowGeoForm}
      />

      {/* Single delete confirmation dialog */}
      <DeleteConfirmDialog
        fileName={deleteConfirm?.fileName}
        open={!!deleteConfirm}
        onCancel={() => {
          setDeleteConfirm(null)
          removeThumbnailRef.current = null
        }}
        onConfirm={handleConfirmDelete}
        loading={isDeleting}
      />

      {/* Bulk delete dialog */}
      <BulkDeleteDialog
        count={bulkDeleteImages?.length ?? 0}
        open={!!bulkDeleteImages}
        onCancel={() => {
          setBulkDeleteImages(null)
          setBulkDeleteCleanup(null)
        }}
        onConfirm={handleConfirmBulkDelete}
        useTrash={bulkUseTrash}
        onUseTrashChange={setBulkUseTrash}
        trashDir={trashDir}
        loading={isDeleting}
      />
    </div>
  )
}
