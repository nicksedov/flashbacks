import { useState, useEffect, useCallback } from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { fetchSyncHistory } from "@/api/endpoints"
import { useTranslation } from "@/i18n"
import type { SyncHistoryEntry } from "@/types"
import { Loader2, History } from "lucide-react"

type PeriodPreset = "1d" | "7d" | "1m" | "custom"

function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return ""
  try {
    let d: Date
    if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$/.test(iso)) {
      const [datePart, timePart] = iso.split("T")
      const [y, m, day] = datePart.split("-").map(Number)
      const [h, min, s] = timePart.split(":").map(Number)
      d = new Date(y, m - 1, day, h, min, s)
    } else {
      d = new Date(iso)
    }
    return d.toLocaleString()
  } catch {
    return iso
  }
}

interface SyncHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function SyncHistoryDialog({ open, onOpenChange }: SyncHistoryDialogProps) {
  const { t } = useTranslation()
  const [entries, setEntries] = useState<SyncHistoryEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [periodPreset, setPeriodPreset] = useState<PeriodPreset>("7d")
  const [customFrom, setCustomFrom] = useState("")
  const [customTo, setCustomTo] = useState("")

  const getDateRange = useCallback((): { from: string; to: string } => {
    const now = new Date()
    const to = now.toISOString()
    let from: string

    switch (periodPreset) {
      case "1d": {
        const d = new Date(now)
        d.setDate(d.getDate() - 1)
        from = d.toISOString()
        break
      }
      case "7d": {
        const d = new Date(now)
        d.setDate(d.getDate() - 7)
        from = d.toISOString()
        break
      }
      case "1m": {
        const d = new Date(now)
        d.setMonth(d.getMonth() - 1)
        from = d.toISOString()
        break
      }
      case "custom": {
        from = customFrom ? new Date(customFrom).toISOString() : ""
        break
      }
      default:
        from = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString()
    }

    return { from, to: periodPreset === "custom" && customTo ? new Date(customTo).toISOString() : to }
  }, [periodPreset, customFrom, customTo])

  const loadHistory = useCallback(async () => {
    setLoading(true)
    try {
      const { from, to } = getDateRange()
      const data = await fetchSyncHistory(from, to)
      setEntries(data.entries)
    } catch {
      setEntries([])
    } finally {
      setLoading(false)
    }
  }, [getDateRange])

  useEffect(() => {
    if (open) {
      loadHistory()
    }
  }, [open, loadHistory])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-3xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <History className="h-5 w-5" />
            {t("settings.dailySync.historyTitle")}
          </DialogTitle>
          <DialogDescription>
            {t("settings.dailySync.description")}
          </DialogDescription>
        </DialogHeader>

        {/* Period selector */}
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1.5">
            <Label className="text-sm">{t("common.period")}</Label>
            <Select
              value={periodPreset}
              onValueChange={(v: PeriodPreset) => setPeriodPreset(v)}
            >
              <SelectTrigger className="w-[140px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="1d">{t("settings.dailySync.historyPeriod1d")}</SelectItem>
                <SelectItem value="7d">{t("settings.dailySync.historyPeriod7d")}</SelectItem>
                <SelectItem value="1m">{t("settings.dailySync.historyPeriod1m")}</SelectItem>
                <SelectItem value="custom">{t("settings.dailySync.historyPeriodCustom")}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {periodPreset === "custom" && (
            <>
              <div className="space-y-1.5">
                <Label className="text-sm">{t("settings.dailySync.historyFrom")}</Label>
                <Input
                  type="date"
                  value={customFrom}
                  onChange={(e) => setCustomFrom(e.target.value)}
                  className="w-[160px]"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-sm">{t("settings.dailySync.historyTo")}</Label>
                <Input
                  type="date"
                  value={customTo}
                  onChange={(e) => setCustomTo(e.target.value)}
                  className="w-[160px]"
                />
              </div>
            </>
          )}

          <Button onClick={loadHistory} disabled={loading} size="sm">
            {loading ? <Loader2 className="h-4 w-4 animate-spin mr-1" /> : null}
            {t("common.refresh")}
          </Button>
        </div>

        {/* History table */}
        <div className="mt-4 rounded-md border overflow-x-auto">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : entries.length === 0 ? (
            <div className="py-12 text-center text-sm text-muted-foreground">
              {t("settings.dailySync.historyEmpty")}
            </div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-2 text-left font-medium text-muted-foreground">
                    {t("settings.dailySync.historyDate")}
                  </th>
                  <th className="px-4 py-2 text-right font-medium text-muted-foreground">
                    {t("settings.dailySync.historyNew")}
                  </th>
                  <th className="px-4 py-2 text-right font-medium text-muted-foreground">
                    {t("settings.dailySync.historyUpdated")}
                  </th>
                  <th className="px-4 py-2 text-right font-medium text-muted-foreground">
                    {t("settings.dailySync.historyDeleted")}
                  </th>
                  <th className="px-4 py-2 text-right font-medium text-muted-foreground">
                    {t("settings.dailySync.historyThumbnails")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr key={entry.id} className="border-b last:border-0 hover:bg-muted/30">
                    <td className="px-4 py-2 text-left whitespace-nowrap">
                      {formatDateTime(entry.createdAt)}
                    </td>
                    <td className="px-4 py-2 text-right tabular-nums">{entry.newFiles}</td>
                    <td className="px-4 py-2 text-right tabular-nums">{entry.updatedFiles}</td>
                    <td className="px-4 py-2 text-right tabular-nums">{entry.deletedFiles}</td>
                    <td className="px-4 py-2 text-right tabular-nums">{entry.thumbnailsGenerated}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Summary */}
        {!loading && entries.length > 0 && (
          <div className="text-xs text-muted-foreground text-right">
            {t("common.total")}: {entries.length}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
