import { useEffect, useId, useRef } from "react"
import type { ReactNode } from "react"
import { ChevronDown, ChevronUp } from "lucide-react"
import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { useTranslation } from "@/i18n"

export interface MobileDetailPanelProps {
  /** Whether the collapsible detail panel is expanded below `md`. */
  open: boolean
  /** Toggle handler; the panel is collapsed by default on mobile. */
  onOpenChange: (open: boolean) => void
  /** Panel content, unchanged across breakpoints. */
  children: ReactNode
  /** Additional classes merged onto the panel surface. */
  className?: string
}

/**
 * Shared lightbox detail panel that turns a fixed-width side column into a
 * collapsible overlay below the `md` breakpoint.
 *
 * Below `md` the panel is absolutely positioned (out of the flex flow) so
 * expanding it never resizes the image: a bottom sheet in portrait, a right
 * sheet in landscape. At `md` and above it resets to the existing static
 * fixed-width column, keeping the desktop layout unchanged.
 */
const PANEL_LAYOUT = [
  // Below `md`: overlay out of the flex flow so the image keeps its size.
  // Portrait is the base (bottom sheet) with a dynamic-viewport height and a
  // bottom safe-area inset.
  "absolute left-0 right-0 bottom-0 z-20 h-[80dvh] pb-[env(safe-area-inset-bottom)]",
  // Landscape re-anchors the sheet to the right edge with a right safe-area
  // inset and no bottom inset. The `max-md:` scope is mandatory: Tailwind
  // emits the orientation media block after the `md` block, so a bare
  // `landscape:` utility would beat `md:w-[400px]`/`md:h-full` on every
  // landscape desktop viewport and squeeze the image area.
  "max-md:landscape:top-0 max-md:landscape:left-auto max-md:landscape:right-0 max-md:landscape:h-auto max-md:landscape:w-[80%] max-md:landscape:pb-0 max-md:landscape:pr-[env(safe-area-inset-right)]",
  // Motion is opt-in (`motion-safe:`) so `prefers-reduced-motion` suppresses it.
  "motion-safe:transition-all motion-safe:duration-200 motion-safe:ease-out",
  // At `md` and above: reset to the current static fixed-width column.
  "md:static md:h-full md:w-[400px] md:min-w-[320px] md:pb-0 md:pr-0 lg:w-[450px]",
  "shrink-0 flex-col border-t bg-card md:border-t-0 md:border-l",
].join(" ")

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function MobileDetailPanel({ open, onOpenChange, children, className }: MobileDetailPanelProps) {
  const { t } = useTranslation()
  const panelId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const toggleRef = useRef<HTMLButtonElement>(null)
  const wasOpenRef = useRef(false)

  // Move focus into the panel on expand and back to the toggle on collapse.
  useEffect(() => {
    if (open && !wasOpenRef.current) {
      const panel = panelRef.current
      if (panel) {
        const focusable = panel.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
        ;(focusable ?? panel).focus()
      }
    } else if (!open && wasOpenRef.current) {
      toggleRef.current?.focus()
    }
    wasOpenRef.current = open
  }, [open])

  // Escape and the platform back gesture collapse the panel before the dialog.
  // The capture-phase listener stops the event reaching the Radix dialog while
  // the panel is open, so only a collapsed panel lets the dialog close.
  useEffect(() => {
    if (!open) return

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation()
        onOpenChange(false)
      }
    }
    const handlePopState = () => onOpenChange(false)

    document.addEventListener("keydown", handleKeyDown, true)
    window.addEventListener("popstate", handlePopState)
    return () => {
      document.removeEventListener("keydown", handleKeyDown, true)
      window.removeEventListener("popstate", handlePopState)
    }
  }, [open, onOpenChange])

  const panelClasses = cn(PANEL_LAYOUT, open ? "flex" : "hidden md:flex", className)

  return (
    <>
      {open && (
        <div
          data-testid="mobile-detail-scrim"
          aria-hidden="true"
          className="absolute inset-0 z-10 bg-background/60 md:hidden"
          onClick={() => onOpenChange(false)}
        />
      )}

      <div
        id={panelId}
        ref={panelRef}
        role="region"
        aria-label={t("lightbox.panel.details")}
        tabIndex={-1}
        data-testid="mobile-detail-panel"
        data-open={open ? "true" : "false"}
        className={panelClasses}
      >
        {children}
      </div>

      <Button
        ref={toggleRef}
        type="button"
        variant="ghost"
        data-testid="mobile-detail-toggle"
        aria-label={open ? t("lightbox.panel.hide") : t("lightbox.panel.show")}
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => onOpenChange(!open)}
        className="absolute left-2 top-2 z-30 flex h-11 min-w-11 items-center justify-center gap-1.5 rounded-full bg-background/80 px-3 text-xs font-medium text-foreground shadow hover:bg-muted/80 md:hidden"
      >
        {open ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />}
        <span>{t("lightbox.panel.details")}</span>
      </Button>
    </>
  )
}
