import * as React from "react"
import * as ProgressPrimitive from "@radix-ui/react-progress"
import { Loader2 } from "lucide-react"
import { cn } from "@/lib/utils"

const Progress = React.forwardRef<
  React.ComponentRef<typeof ProgressPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof ProgressPrimitive.Root>
>(({ className, value, ...props }, ref) => {
  const indeterminate = value === undefined || value === null
  return (
    <ProgressPrimitive.Root
      ref={ref}
      className={cn("relative h-2 w-full overflow-hidden rounded-full bg-primary/20", className)}
      value={value}
      {...props}
    >
      <ProgressPrimitive.Indicator
        className={cn(
          "h-full bg-primary transition-all",
          indeterminate ? "w-1/3 motion-safe:animate-pulse" : "w-full flex-1"
        )}
        style={indeterminate ? undefined : { transform: `translateX(-${100 - (value ?? 0)}%)` }}
      />
    </ProgressPrimitive.Root>
  )
})
Progress.displayName = ProgressPrimitive.Root.displayName

interface ProgressIndicatorProps {
  /** Determinate progress 0-100; omit (or pass undefined) for an indeterminate bar. */
  value?: number
  /** Localized label shown beside the spinner above the bar. */
  label?: string
  /** Show a spinner while the operation is running. */
  loading?: boolean
  /** Optional localized text shown under the bar (for example the current directory). */
  statusText?: string
  /** Optional localized statistics shown on the right of the status text. */
  statsText?: string
  className?: string
}

/**
 * Shared background-progress indicator used by every scan-style progress display.
 *
 * Renders as a borderless inset fill (`bg-muted`) using semantic tokens only — no
 * raw palette literals — so it follows the active theme, and supports both a
 * determinate bar (when `value` is known) and an indeterminate bar (when it is
 * not). The label and text props are expected to be already localized by the
 * caller.
 */
export function ProgressIndicator({
  value,
  label,
  loading = false,
  statusText,
  statsText,
  className,
}: ProgressIndicatorProps) {
  const indeterminate = value === undefined
  return (
    <div
      data-slot="progress-indicator"
      data-state={indeterminate ? "indeterminate" : "determinate"}
      className={cn("space-y-1.5 rounded-lg bg-muted p-3", className)}
    >
      {label && (
        <div className="flex items-center gap-2 text-sm font-medium text-foreground">
          {loading && (
            <Loader2 className="h-4 w-4 motion-safe:animate-spin" aria-hidden="true" />
          )}
          <span>{label}</span>
        </div>
      )}
      <Progress value={value} className="h-1.5" />
      {(statusText || statsText) && (
        <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
          <span className="truncate">{statusText}</span>
          {statsText && <span className="shrink-0">{statsText}</span>}
        </div>
      )}
    </div>
  )
}

export { Progress }
