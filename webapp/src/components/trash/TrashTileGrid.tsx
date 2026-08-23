import { CalendarDays } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { TrashDateGroup, TrashItemDTO } from "@/types"
import { TrashTile } from "./TrashTile"

interface TrashTileGridProps {
  groups: TrashDateGroup[]
  onView: (item: TrashItemDTO) => void
  onRestore: (item: TrashItemDTO) => void
  /** Image loading strategy: "lazy" (default) or "eager". */
  loading?: "lazy" | "eager"
}

/**
 * Responsive thumbnail grid for the Trash view, grouped by deletion date
 * (newest first), using the same breakpoints as the gallery grid.
 */
export function TrashTileGrid({ groups, onView, onRestore, loading = "lazy" }: TrashTileGridProps) {
  const { t } = useTranslation()

  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <div key={group.date} id={`trash-date-group-${group.date}`}>
          <div className="flex items-center gap-2 mb-2 px-0.5">
            <CalendarDays className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm font-medium truncate" title={group.date}>
              {group.label}
            </span>
            <span className="text-xs text-muted-foreground shrink-0">
              {t("trashTab.itemCount", { count: group.itemCount.toString() })}
            </span>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 lg:grid-cols-7 xl:grid-cols-8 gap-1.5">
            {group.items.map((item) => (
              <TrashTile
                key={item.id}
                item={item}
                onView={onView}
                onRestore={onRestore}
                loading={loading}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}
