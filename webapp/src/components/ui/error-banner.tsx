interface ErrorBannerProps {
  message: string
  className?: string
}

/** Shared destructive error banner used across all views/tabs. */
export function ErrorBanner({ message, className }: ErrorBannerProps) {
  return (
    <div
      className={`rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive ${className ?? ""}`}
    >
      {message}
    </div>
  )
}
