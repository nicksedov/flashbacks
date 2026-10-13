import { useCallback, useEffect, useRef, useState } from "react"
import { Info } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** How long the popup stays visible before auto-hiding. */
export const INFO_POPOVER_AUTO_HIDE_MS = 3000

export interface InfoPopoverProps {
  /**
   * Full informational text. Used as the popup content and as the accessible
   * name of the icon trigger (so the label survives when only the icon shows).
   */
  text: string
  /** Additional classes for the icon trigger button. */
  className?: string
  /** Additional classes for the popup surface. */
  popoverClassName?: string
}

/**
 * A compact, dependency-free info affordance: an icon button that reveals the
 * full informational text in a small popup which auto-hides after 3 seconds.
 * Used to collapse long labels (for example an image count) on mobile.
 */
export function InfoPopover({ text, className, popoverClassName }: InfoPopoverProps) {
  const [open, setOpen] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  const handleToggle = useCallback(() => {
    setOpen((prev) => {
      const next = !prev
      clearTimer()
      if (next) {
        timerRef.current = setTimeout(() => setOpen(false), INFO_POPOVER_AUTO_HIDE_MS)
      }
      return next
    })
  }, [clearTimer])

  // Clear any pending auto-hide timer on unmount.
  useEffect(() => clearTimer, [clearTimer])

  return (
    <div className="relative">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className={cn("h-9 w-9 text-muted-foreground", className)}
        onClick={handleToggle}
        aria-label={text}
        title={text}
      >
        <Info className="h-5 w-5" />
      </Button>
      {open && (
        <div
          role="status"
          className={cn(
            "absolute right-0 top-full z-20 mt-1 max-w-[min(16rem,80vw)] rounded-md bg-popover px-3 py-2 text-xs text-popover-foreground shadow-md ring-1 ring-border",
            popoverClassName
          )}
        >
          {text}
        </div>
      )}
    </div>
  )
}
