import { useCallback, useState } from "react"
import { GalleryFoldersView } from "@/components/gallery/GalleryFoldersView"
import { GalleryCalendarView } from "@/components/gallery/GalleryCalendarView"
import { GalleryGeolocationView } from "@/components/gallery/GalleryGeolocationView"
import { UnifiedLightbox } from "@/components/gallery/UnifiedLightbox"
import type { LightboxMode } from "@/components/gallery/UnifiedLightbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { deleteFiles } from "@/api/endpoints"
import { API_BASE_URL } from "@/api/client"
import { useSettings } from "@/providers/useSettings"
import { useTranslation } from "@/i18n"
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
  const [deleteConfirm, setDeleteConfirm] = useState<{ image: GalleryImageDTO; removeThumbnail: () => void } | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const handleImageClick = useCallback((image: GalleryImageDTO) => {
    setLightboxImage(image.path)
    setLightboxMode("ai")
  }, [])

  const handleImageDownload = useCallback((image: GalleryImageDTO) => {
    const imageUrl = `${API_BASE_URL}/api/image?path=${encodeURIComponent(image.path)}`
    const a = document.createElement("a")
    a.href = imageUrl
    a.download = image.fileName
    a.click()
  }, [])

  const handleImageDelete = useCallback((image: GalleryImageDTO, removeThumbnail: () => void) => {
    setDeleteConfirm({ image, removeThumbnail })
  }, [])

  const handleConfirmDelete = useCallback(async () => {
    if (!deleteConfirm) return
    setIsDeleting(true)
    try {
      await deleteFiles({
        filePaths: [deleteConfirm.image.path],
        trashDir: trashDir || "",
      })
      deleteConfirm.removeThumbnail()
    } catch (err) {
      console.error("Failed to delete file:", err)
      alert("Failed to delete file")
    } finally {
      setIsDeleting(false)
      setDeleteConfirm(null)
    }
  }, [deleteConfirm, trashDir])

  return (
    <div className={galleryMode === "geolocation" ? "space-y-2" : "space-y-4"}>
      {galleryMode === "folders" ? (
        <GalleryFoldersView
          onImageClick={handleImageClick}
          onImageDownload={handleImageDownload}
          onImageDelete={handleImageDelete}
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

      <Dialog open={!!deleteConfirm} onOpenChange={(open) => !open && setDeleteConfirm(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("gallery.deleteConfirm.title")}</DialogTitle>
            <DialogDescription>
              {deleteConfirm && t("gallery.deleteConfirm.description", { fileName: deleteConfirm.image.fileName })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteConfirm(null)} disabled={isDeleting}>
              {t("gallery.deleteConfirm.cancel")}
            </Button>
            <Button variant="destructive" onClick={handleConfirmDelete} disabled={isDeleting}>
              {isDeleting ? t("gallery.deleteConfirm.deleting") : t("gallery.deleteConfirm.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
