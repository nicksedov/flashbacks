import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ThumbnailImage } from "./ThumbnailImage"
import { FileItem } from "./FileItem"
import { useTranslation } from "@/i18n"
import type { DuplicateGroupDTO, FileDTO } from "@/types"

interface DuplicateGroupCardProps {
  group: DuplicateGroupDTO
  isSelected: (path: string) => boolean
  onToggleFile: (path: string) => void
  onSelectFolder: (dirPath: string) => void
}

export function DuplicateGroupCard({
  group,
  isSelected,
  onToggleFile,
  onSelectFolder,
}: DuplicateGroupCardProps) {
  const allFiles: FileDTO[] = group.files
  const { t } = useTranslation()

  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex flex-wrap items-center gap-2">
          <CardTitle className="text-sm">{t("duplicateGroup.title", { index: group.index })}</CardTitle>
          <Badge variant="secondary" className="text-xs">{t("duplicateGroup.files", { count: group.files.length })}</Badge>
          <Badge variant="outline" className="text-xs">{t("duplicateGroup.sizeEach", { size: group.sizeHuman })}</Badge>
          <span className="text-xs text-muted-foreground font-mono">{t("duplicateGroup.md5", { hash: group.hash })}</span>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex gap-4">
          <div className="shrink-0">
            <ThumbnailImage src={group.thumbnail} />
          </div>
          <div className="min-w-0 flex-1 space-y-1">
            {allFiles.map((file) => (
              <FileItem
                key={file.id}
                file={file}
                isSelected={isSelected(file.path)}
                onToggle={onToggleFile}
                onSelectFolder={(dirPath) => onSelectFolder(dirPath)}
              />
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
