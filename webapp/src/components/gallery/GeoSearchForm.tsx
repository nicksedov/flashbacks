import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Progress } from "@/components/ui/progress"
import { MapPin, Loader2, Check, X, StopCircle } from "lucide-react"
import { useTranslation } from "@/i18n"
import { useGeocodeSearch } from "@/hooks/useGeocodeSearch"
import { useBatchGps } from "@/hooks/useBatchGps"
import { updateImageGps, fetchLocationCandidatesByDate } from "@/api/endpoints"
import type { GeocodeSearchResult, LocationCandidate } from "@/types"

interface GeoSearchFormProps {
  /** For single-image mode (lightbox): the image path */
  imagePath?: string
  /** For batch mode (calendar): array of image paths to update */
  paths?: string[]
  /** Date string (YYYY-MM-DD) for fetching location candidates */
  date?: string
  /** For batch mode: number of photos that will be affected */
  affectedCount?: number
  /** Called after GPS is successfully saved */
  onGpsSaved: () => void
}

interface SelectedLocation {
  lat: number
  lng: number
  label: string
}

export function GeoSearchForm({ imagePath, paths, date, affectedCount, onGpsSaved }: GeoSearchFormProps) {
  const { t } = useTranslation()
  const { query, setQuery, results, isSearching } = useGeocodeSearch()
  const [candidates, setCandidates] = useState<LocationCandidate[]>([])
  const [selected, setSelected] = useState<SelectedLocation | null>(null)
  const [isSaving, setIsSaving] = useState(false)
  const [saveStatus, setSaveStatus] = useState<"idle" | "success" | "error">("idle")

  const { progress, run: runBatchGps, cancel: cancelBatchGps } = useBatchGps()

  const isBatchMode = paths != null && paths.length > 0

  // Load location candidates on mount (always by date)
  useEffect(() => {
    let cancelled = false

    const loadCandidates = async () => {
      if (!date) return
      try {
        const res = await fetchLocationCandidatesByDate(date)
        if (!cancelled && res && res.candidates.length > 0) {
          setCandidates(res.candidates)
        }
      } catch {
        // Silently ignore - candidates are optional
      }
    }

    loadCandidates()
    return () => { cancelled = true }
  }, [date])

  const handleSelectCandidate = (c: LocationCandidate) => {
    const label = [c.nameLocal, c.nameEng].filter(Boolean).join(", ")
    setSelected({ lat: c.lat, lng: c.lng, label })
    setQuery("")
  }

  const handleSelectSearchResult = (r: GeocodeSearchResult) => {
    setSelected({ lat: r.lat, lng: r.lon, label: r.displayName })
    setQuery("")
  }

  const handleSave = async () => {
    if (!selected) return

    if (isBatchMode) {
      // Use batched GPS updates for large sets
      const res = await runBatchGps(paths!, selected.lat, selected.lng)
      if (res.success > 0 || res.skipped > 0) {
        setSaveStatus("success")
        setTimeout(() => onGpsSaved(), 1200)
      } else {
        setSaveStatus("error")
      }
    } else if (imagePath) {
      // Single-image mode: direct call
      setIsSaving(true)
      setSaveStatus("idle")
      try {
        await updateImageGps({ path: imagePath, lat: selected.lat, lng: selected.lng })
        setSaveStatus("success")
        setTimeout(() => onGpsSaved(), 600)
      } catch {
        setSaveStatus("error")
      } finally {
        setIsSaving(false)
      }
    }
  }

  const isProcessing = progress.running

  // Calculate progress percentage
  const progressPercent = progress.total > 0
    ? Math.round((progress.processed / progress.total) * 100)
    : 0

  return (
    <div className="space-y-3 mt-2">
      {/* Location candidates */}
      {candidates.length > 0 && !selected && !isProcessing && (
        <div>
          <p className="text-[10px] text-muted-foreground mb-1.5">{t("geo.suggestedLocations")}</p>
          <div className="flex flex-wrap gap-1">
            {candidates.map((c, i) => {
              const label = [c.nameLocal, c.nameEng].filter(Boolean).join(", ")
              const coords = `${c.lat.toFixed(2)}, ${c.lng.toFixed(2)}`
              return (
                <button
                  key={i}
                  type="button"
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md text-[10px] border bg-card hover:bg-accent transition-colors"
                  onClick={() => handleSelectCandidate(c)}
                  title={`${label} (${c.photoCount})`}
                >
                  {label ? `${label} · ${coords}` : coords}
                  <span className="text-muted-foreground">({c.photoCount})</span>
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Search input - hidden during batch processing */}
      {!isProcessing && (
        <div className="relative">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("geo.searchPlaceholder")}
            className="h-8 text-xs"
          />
          {isSearching && (
            <Loader2 className="absolute right-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 animate-spin text-muted-foreground" />
          )}

          {/* Search results dropdown */}
          {results.length > 0 && !selected && (
            <div className="absolute z-50 top-full left-0 right-0 mt-1 max-h-48 overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-lg" style={{ backgroundColor: 'var(--color-popover)', color: 'var(--color-popover-foreground)' }}>
              {results.map((r, i) => (
                <button
                  key={i}
                  type="button"
                  className="w-full text-left px-2.5 py-1.5 text-xs hover:bg-accent transition-colors truncate"
                  onClick={() => handleSelectSearchResult(r)}
                  title={r.displayName}
                >
                  {r.displayName}
                </button>
              ))}
            </div>
          )}

          {query.length >= 2 && results.length === 0 && !isSearching && !selected && (
            <div className="absolute z-50 top-full left-0 right-0 mt-1 rounded-md border bg-popover text-popover-foreground shadow-lg px-2.5 py-2 text-xs text-muted-foreground" style={{ backgroundColor: 'var(--color-popover)', color: 'var(--color-popover-foreground)' }}>
              {t("geo.noResults")}
            </div>
          )}
        </div>
      )}

      {/* Selected location display */}
      {selected && !isProcessing && (
        <div className="rounded-md border bg-accent/50 p-2">
          <div className="flex items-start justify-between gap-2 mb-0.5">
            <p className="text-[10px] text-muted-foreground">{t("geo.selectedLocation")}</p>
            <button
              type="button"
              className="shrink-0 inline-flex items-center gap-0.5 text-[10px] text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => setSelected(null)}
              title={t("geo.changeSelection")}
            >
              <X className="h-3 w-3" />
              {t("geo.changeSelection")}
            </button>
          </div>
          <p className="text-xs font-medium break-words" title={selected.label}>{selected.label}</p>
          <p className="text-[10px] text-muted-foreground mt-0.5">
            {selected.lat.toFixed(4)}&deg;, {selected.lng.toFixed(4)}&deg;
          </p>
        </div>
      )}

      {/* Batch mode: affected count info (before processing starts) */}
      {isBatchMode && affectedCount != null && affectedCount > 0 && !isProcessing && progress.total === 0 && (
        <p className="text-[10px] text-muted-foreground">
          {t("geo.bulkSetDescription", { count: affectedCount })}
        </p>
      )}

      {/* Batch progress UI */}
      {(isProcessing || progress.total > 0) && (
        <div className="space-y-1.5">
          <Progress value={progressPercent} className="h-1.5" />
          <div className="flex items-center justify-between text-[10px] text-muted-foreground">
            {isProcessing ? (
              <span>{t("geo.batchProgress", { current: progress.processed, total: progress.total })}</span>
            ) : (
              <span>{t("geo.batchComplete", { success: progress.success, skipped: progress.skipped, failed: progress.failed })}</span>
            )}
            {isProcessing && (
              <button
                type="button"
                className="inline-flex items-center gap-0.5 text-destructive hover:text-destructive/80 transition-colors"
                onClick={cancelBatchGps}
              >
                <StopCircle className="h-3 w-3" />
                {t("common.cancel")}
              </button>
            )}
          </div>
        </div>
      )}

      {/* Save / Cancel button */}
      {!isProcessing && (
        <Button
          type="button"
          size="sm"
          className="w-full text-xs"
          disabled={!selected || isSaving}
          onClick={handleSave}
          variant={saveStatus === "success" ? "default" : saveStatus === "error" ? "destructive" : "default"}
        >
          {isSaving ? (
            <>
              <Loader2 className="h-3.5 w-3.5 mr-1.5 animate-spin" />
              {t("geo.savingLocation")}
            </>
          ) : saveStatus === "success" ? (
            <>
              <Check className="h-3.5 w-3.5 mr-1.5" />
              {t("geo.saveSuccess")}
            </>
          ) : saveStatus === "error" ? (
            t("geo.saveFailed")
          ) : (
            <>
              <MapPin className="h-3.5 w-3.5 mr-1.5" />
              {t("geo.saveLocation")}
            </>
          )}
        </Button>
      )}
    </div>
  )
}
