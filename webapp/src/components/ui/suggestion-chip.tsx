import * as React from "react"
import { cn } from "@/lib/utils"

export interface SuggestionChipProps {
  /** Visible text and, by default, the accessible name of the chip. */
  label: string
  /** Optional leading icon, matching the assistant's prompt chips. */
  icon?: React.ReactNode
  onClick: () => void
  className?: string
}

const CHIP_CLASSES =
  "flex items-center gap-1.5 px-2.5 py-1.5 rounded-full border border-border/70 text-xs text-muted-foreground hover:bg-muted hover:text-foreground transition-colors"

/**
 * One inline suggestion, wherever suggestions are offered.
 *
 * The AI assistant's prompt suggestions and the person-name suggestions in the
 * rename dialog both render through this primitive, so a suggestion looks the
 * same everywhere: a compact pill carrying only its own text — no group heading
 * and no secondary counter.
 */
export function SuggestionChip({ label, icon, onClick, className }: SuggestionChipProps) {
  return (
    <button
      type="button"
      data-slot="suggestion-chip"
      className={cn(CHIP_CLASSES, className)}
      onClick={onClick}
    >
      {icon}
      <span>{label}</span>
    </button>
  )
}
