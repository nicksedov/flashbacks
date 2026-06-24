import { useMemo } from "react"
import { useTranslation } from "@/i18n"
import { Folder } from "lucide-react"
import type { GalleryImageDTO } from "@/types"
import { ExifImageTile } from "./ExifImageTile"

interface ExifImageGridProps {
  images: GalleryImageDTO[]
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  onAddGeo?: (image: GalleryImageDTO) => void
}

function getFolderName(dirPath: string): string {
  const normalized = dirPath.replace(/\\/g, "/").replace(/\/+$/, "")
  const lastSlash = normalized.lastIndexOf("/")
  return lastSlash >= 0 ? normalized.substring(lastSlash + 1) : normalized
}

export function ExifImageGrid({ images, onImageClick, onImageDownload, onImageDelete, onAddGeo }: ExifImageGridProps) {
  const { t } = useTranslation()

  const groupedByFolder = useMemo(() => {
    const groups: { dirPath: string; folderName: string; images: GalleryImageDTO[] }[] = []
    const map = new Map<string, GalleryImageDTO[]>()
    const order: string[] = []

    for (const image of images) {
      const dir = image.dirPath
      if (!map.has(dir)) {
        map.set(dir, [])
        order.push(dir)
      }
      map.get(dir)!.push(image)
    }

    for (const dir of order) {
      groups.push({
        dirPath: dir,
        folderName: getFolderName(dir),
        images: map.get(dir)!,
      })
    }

    return groups
  }, [images])

  return (
    <div className="space-y-5">
      {groupedByFolder.map((group) => (
        <div key={group.dirPath}>
          <div className="flex items-center gap-2 mb-2 px-0.5">
            <Folder className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm font-medium truncate" title={group.dirPath}>
              {group.folderName}
            </span>
            <span className="text-xs text-muted-foreground shrink-0">
              {t("gallery.folderImageCount", { count: group.images.length.toString() })}
            </span>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
            {group.images.map((image) => (
              <ExifImageTile
                key={image.id}
                image={image}
                onClick={onImageClick}
                onImageDownload={onImageDownload}
                onImageDelete={onImageDelete}
                onAddGeo={onAddGeo}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
