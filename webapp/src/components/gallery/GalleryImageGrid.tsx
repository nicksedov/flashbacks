import { useMemo } from "react"
import { useTranslation } from "@/i18n"
import { Folder } from "lucide-react"
import type { GalleryImageDTO, GalleryFolderDTO } from "@/types"
import { ImageTile } from "./ImageTile"

interface GalleryImageGridProps {
  images: GalleryImageDTO[]
  onImageClick: (image: GalleryImageDTO) => void
  onImageDownload?: (image: GalleryImageDTO) => void
  onImageDelete?: (image: GalleryImageDTO) => void
  rootFolders?: GalleryFolderDTO[]
}

function normalizePath(p: string): string {
  return p.replace(/\\/g, "/").replace(/\/+$/, "")
}

function getRelativeFolderName(dirPath: string, rootFolders?: GalleryFolderDTO[]): string {
  const normalized = normalizePath(dirPath)

  if (rootFolders?.length) {
    // Find the matching root folder (longest match first)
    const sorted = [...rootFolders].sort((a, b) => b.path.length - a.path.length)
    for (const root of sorted) {
      const rootNorm = normalizePath(root.path)
      if (normalized === rootNorm) {
        // Image is directly in the root folder — show root folder name
        return rootNorm.substring(rootNorm.lastIndexOf("/") + 1)
      }
      if (normalized.startsWith(rootNorm + "/")) {
        // Image is in a subfolder — show relative path including root name
        const rootName = rootNorm.substring(rootNorm.lastIndexOf("/") + 1)
        const relative = normalized.substring(rootNorm.length + 1)
        return rootName + "/" + relative
      }
    }
  }

  // Fallback: return last segment
  const lastSlash = normalized.lastIndexOf("/")
  return lastSlash >= 0 ? normalized.substring(lastSlash + 1) : normalized
}

export function GalleryImageGrid({ images, onImageClick, onImageDownload, onImageDelete, rootFolders }: GalleryImageGridProps) {
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
        folderName: getRelativeFolderName(dir, rootFolders),
        images: map.get(dir)!,
      })
    }

    return groups
  }, [images, rootFolders])

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
              <ImageTile
                key={image.id}
                image={image}
                onClick={onImageClick}
                onImageDownload={onImageDownload}
                onImageDelete={onImageDelete}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
