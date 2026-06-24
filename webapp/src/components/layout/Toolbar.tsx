import { IconButton } from "@/components/ui/icon-button"
import { Badge } from "@/components/ui/badge"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { PAGE_SIZES } from "@/lib/constants"
import { RefreshCw, RotateCcw, Trash2, Layers } from "lucide-react"
import { useTranslation } from "@/i18n"

interface ToolbarProps {
  selectedCount: number
  pageSize: number
  onPageSizeChange: (size: number) => void
  onRescan: () => void
  onResetSelection: () => void
  onOpenDeleteFiles: () => void
  onOpenBatchDedup: () => void
  isScanning: boolean
}

export function Toolbar({
  selectedCount,
  pageSize,
  onPageSizeChange,
  onRescan,
  onResetSelection,
  onOpenDeleteFiles,
  onOpenBatchDedup,
  isScanning,
}: ToolbarProps) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-card p-3">
      <IconButton size="sm" icon={RefreshCw} onClick={onRescan} disabled={isScanning}>
        {t("toolbar.rescan")}
      </IconButton>
      <IconButton size="sm" variant="outline" icon={RotateCcw} onClick={onResetSelection}>
        {t("toolbar.resetSelection")}
      </IconButton>
      <IconButton size="sm" variant="destructive" icon={Trash2} onClick={onOpenDeleteFiles} disabled={selectedCount === 0}>
        {t("toolbar.deleteSelected")}
      </IconButton>
      <IconButton size="sm" variant="outline" icon={Layers} onClick={onOpenBatchDedup}>
        {t("toolbar.batchDedup")}
      </IconButton>

      <div className="ml-auto flex items-center gap-3">
        {selectedCount > 0 && (
          <Badge variant="secondary">
            {selectedCount === 1
              ? t("toolbar.filesSelectedOne", { count: selectedCount })
              : t("toolbar.filesSelected", { count: selectedCount })}
          </Badge>
        )}
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground whitespace-nowrap">{t("toolbar.groupsPerPage")}</span>
          <Select value={String(pageSize)} onValueChange={(v) => onPageSizeChange(Number(v))}>
            <SelectTrigger className="w-20 h-8 text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZES.map((size) => (
                <SelectItem key={size} value={String(size)}>{size}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
    </div>
  )
}
