import { Check } from "lucide-react"
import { useTranslation } from "@/i18n"

interface SelectionCheckboxProps {
  selected: boolean
  visible: boolean
  onToggle: (e: React.MouseEvent | React.KeyboardEvent) => void
}

/** Shared selection checkbox shown in the top-right corner of a tile. */
export function SelectionCheckbox({ selected, visible, onToggle }: SelectionCheckboxProps) {
  const { t } = useTranslation()

  if (!visible) return null

  return (
    <button
      type="button"
      className={`absolute top-1 right-1 flex h-5 w-5 items-center justify-center rounded-full border-2 transition-colors z-10 ${
        selected
          ? "bg-primary border-primary text-primary-foreground"
          : "bg-background/80 border-muted-foreground/30 hover:border-primary/60"
      }`}
      onClick={(e) => {
        e.stopPropagation()
        onToggle(e)
      }}
      title={selected ? t("gallery.selection.unselect") : t("gallery.selection.select")}
    >
      {selected && <Check className="h-3 w-3" />}
    </button>
  )
}
