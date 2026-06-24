import { Checkbox } from "@/components/ui/checkbox"
import { useTranslation } from "@/i18n"
import type { FileDTO } from "@/types"
import { Folder } from "lucide-react"

interface FileItemProps {
  file: FileDTO
  isSelected: boolean
  onToggle: (path: string) => void
  onSelectFolder: (dirPath: string) => void
}

export function FileItem({ file, isSelected, onToggle, onSelectFolder }: FileItemProps) {
  const { t } = useTranslation()

  return (
    <div
      className={`flex items-start gap-3 rounded-md px-3 py-2 transition-colors ${
        isSelected ? "bg-primary/5 border border-primary/20" : "hover:bg-muted/50"
      }`}
    >
      <Checkbox
        checked={isSelected}
        onCheckedChange={() => onToggle(file.path)}
        className="mt-0.5"
      />
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium truncate">{file.fileName}</div>
        <button
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-primary transition-colors truncate max-w-full text-left"
          onClick={() => onSelectFolder(file.dirPath)}
          title={t("fileItem.selectFolder")}
          type="button"
        >
          <Folder className="h-3 w-3 shrink-0" />
          <span className="truncate">{file.dirPath}</span>
        </button>
        <div className="text-xs text-muted-foreground mt-0.5">{t("fileItem.modified", { date: file.modTime })}</div>
      </div>
    </div>
  )
}
