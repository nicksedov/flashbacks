import { useCallback, useRef, useState } from "react"
import { toast } from "sonner"
import { GalleryAllImagesView } from "@/components/gallery/GalleryAllImagesView"
import { GalleryCalendarView } from "@/components/gallery/GalleryCalendarView"
import { GalleryGeolocationView } from "@/components/gallery/GalleryGeolocationView"
import { GalleryFoldersView } from "@/components/gallery/GalleryFoldersView"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import { DeleteConfirmDialog } from "@/components/gallery/DeleteConfirmDialog"
import { BulkDeleteDialog } from "@/components/gallery/BulkDeleteDialog"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import type { LightboxMode } from "@/components/gallery/UnifiedLightbox"
import { deleteFiles } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"
import { downloadImage } from "@/lib/downloadImage"
import type { GalleryImageDTO } from "@/types"

interface GalleryTabProps {
  galleryMode: "allImages" | "calendar" | "geolocation" | "folders"
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
  const [permanentConfirmOpen, setPermanentConfirmOpen] = useState(false)

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
      toast.error(t("deleteFiles.errorFailed"))
    } finally {
      setIsDeleting(false)
      setDeleteConfirm(null)
    }
  }, [deleteConfirm, trashDir, t])

  const handleBulkDeleteRequest = useCallback((selectedImages: GalleryImageDTO[], cleanup: () => void) => {
    setBulkDeleteImages(selectedImages)
    setBulkDeleteCleanup(() => cleanup)
    setBulkUseTrash(true)
  }, [])

  const executeBulkDelete = useCallback(async () => {
    if (!bulkDeleteImages || bulkDeleteImages.length === 0) return
    setPermanentConfirmOpen(false)
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
        toast.info(t("deleteFiles.successWithFailed", { count: result.success, failed: result.failed }))
      }
    } catch (err) {
      console.error("Failed to delete files:", err)
      toast.error(t("deleteFiles.errorFailed"))
    } finally {
      setIsDeleting(false)
    }
  }, [bulkDeleteImages, bulkDeleteCleanup, bulkUseTrash, trashDir, t])

  const handleConfirmBulkDelete = useCallback(() => {
    if (!bulkDeleteImages || bulkDeleteImages.length === 0) return

    if (!bulkUseTrash || !trashDir) {
      setPermanentConfirmOpen(true)
      return
    }

    void executeBulkDelete()
  }, [bulkDeleteImages, bulkUseTrash, trashDir, executeBulkDelete])

  return (
    <div className={galleryMode === "geolocation" ? "space-y-2" : "space-y-4"}>
      {galleryMode === "allImages" ? (
        <GalleryAllImagesView
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
      ) : galleryMode === "folders" ? (
        <GalleryFoldersView
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

      {/* Permanent delete confirmation dialog (shown when trash is disabled) */}
      <Dialog open={permanentConfirmOpen} onOpenChange={(open) => !open && setPermanentConfirmOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("deleteFiles.title")}</DialogTitle>
            <DialogDescription>{t("deleteFiles.confirmPermanent")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPermanentConfirmOpen(false)} disabled={isDeleting}>
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={() => void executeBulkDelete()} disabled={isDeleting}>
              {isDeleting ? t("deleteFiles.deleting") : t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
