import { cn } from "@/lib/utils"

interface SpinnerProps {
  /** Size of the spinner in Tailwind h-/w- classes. */
  className?: string
}

/** Shared loading spinner (primary ring style used across the app). */
export function Spinner({ className }: SpinnerProps) {
  return (
    <div
      className={cn(
        "h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent",
        className
      )}
    />
  )
}

/** Centered fallback used for lazy-loaded tab Suspense boundaries. */
export function TabLoading() {
  return (
    <div className="flex items-center justify-center py-20">
      <Spinner />
    </div>
  )
}
