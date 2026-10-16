import { ProgressIndicator } from "@/components/ui/progress"
import { useTranslation } from "@/i18n"
import type { TranslationKey } from "@/i18n"
import type { ScanPhase, ScanStatusResponse } from "@/types"

const PHASE_KEYS: Record<ScanPhase, TranslationKey> = {
  cleanup: "scanProgress.phase.cleanup",
  scanning: "scanProgress.phase.scanning",
  complete: "scanProgress.phase.complete",
}

interface ScanProgressBannerProps {
  status: ScanStatusResponse
}

export function ScanProgressBanner({ status }: ScanProgressBannerProps) {
  const { t } = useTranslation()

  if (!status.scanning) return null

  const label = status.phase
    ? t(PHASE_KEYS[status.phase])
    : t("scanProgress.phase.scanning")

  // Determinate once the pre-count has reported a total; indeterminate before.
  const value =
    status.totalFiles > 0
      ? Math.min(100, Math.round((status.filesProcessed / status.totalFiles) * 100))
      : undefined

  const statsParts: string[] = [
    status.totalFiles > 0
      ? t("scanProgress.stats", { processed: status.filesProcessed, total: status.totalFiles })
      : t("scanProgress.filesProcessed", { count: status.filesProcessed }),
  ]
  if (status.filesAdded > 0) statsParts.push(t("scanProgress.added", { count: status.filesAdded }))
  if (status.filesDeleted > 0) statsParts.push(t("scanProgress.deleted", { count: status.filesDeleted }))

  return (
    <ProgressIndicator
      value={value}
      label={label}
      loading
      statusText={status.currentDirectory || undefined}
      statsText={statsParts.join(" · ")}
    />
  )
}
