import { useCallback, useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { SearchableSelect } from "@/components/ui/searchable-select"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
import { ProgressIndicator } from "@/components/ui/progress"
import { ScheduleEditor } from "@/components/ui/schedule-editor"
import {
  fetchOCRStatus,
  startOcrClassification,
  startOcrClassificationChanges,
  stopOcrClassification,
  fetchOcrClassificationStatus,
  fetchLlmSettings,
  updateLlmSettings,
  fetchTagScanStatus,
  pauseTagScan,
  resumeTagScan,
  updateSettings,
  fetchSettings,
  probeEmbeddingDimension,
  fetchExifServiceStatus,
  fetchLlmModels,
} from "@/api/endpoints"
import { Shield, Loader2, Zap, Wand2, Play, Square, RefreshCw, Database } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { TranslationKey } from "@/i18n"
import { filterModelsForInstrument } from "@/lib/llm-model-filter"
import { daysToString, parseDaysString } from "@/lib/schedule"
import { deriveTagScanState } from "@/lib/tag-scan-status"
import type { TagScanState } from "@/lib/tag-scan-status"
import type {
  OCRStatus,
  OcrClassificationStatusResponse,
  LlmSettingsResponse,
  TagScanStatusResponse,
  LlmProviderType,
  LlmInstrumentType,
  LlmModelDTO,
  ExifServiceStatus,
} from "@/types"

const PROVIDER_LABELS: Record<LlmProviderType, string> = {
  ollama: "Ollama",
  ollama_cloud: "Ollama Cloud",
  openai: "OpenAI API compatible",
  deepseek: "DeepSeek",
  alibaba: "Alibaba Cloud",
}

const INSTRUMENT_LABELS: Record<LlmInstrumentType, string> = {
  chat: "llm_ocr.chatSettings",
  vl: "llm_ocr.vlSettings",
  embedding: "llm_ocr.embeddingSettings",
  image_edit: "llm_ocr.imgEditSettings",
  ocr: "llm_ocr.ocrSettings",
}

const INSTRUMENT_DESCRIPTIONS: Record<LlmInstrumentType, string> = {
  chat: "llm_ocr.chatSettingsDescription",
  vl: "llm_ocr.vlSettingsDescription",
  embedding: "llm_ocr.embeddingSettingsDescription",
  image_edit: "llm_ocr.imgEditSettingsDescription",
  ocr: "llm_ocr.ocrSettingsDescription",
}

/** Dropdown label for a model: its name plus the size suffix when the catalog reports one. */
function formatModelLabel(model: LlmModelDTO): string {
  return `${model.name}${model.size ? ` (${(model.size / 1073741824).toFixed(1)} GB)` : ""}`
}

const EMPTY_SETTINGS: LlmSettingsResponse = {
  instruments: [],
  tagScan: { enabled: false, days: "1,2,3,4,5", startHour: 23, startMinute: 0, endHour: 7, endMinute: 0, timezoneOffset: 0 },
  embedding: { dimension: 1024, batchSize: 50 },
  providers: [],
}

const TAG_SCAN_STATE_LABELS: Record<TagScanState, TranslationKey> = {
  running: "tagScan.running",
  paused: "tagScan.paused",
  waiting: "tagScan.waiting",
  stopped: "tagScan.stopped",
}

const TAG_SCAN_STATE_CLASSES: Record<TagScanState, string> = {
  running: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  paused: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  waiting: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  stopped: "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200",
}

/** Status badge of the automatic image tagging job. */
export function TagScanStatusBadge({ status }: { status: TagScanStatusResponse }) {
  const { t } = useTranslation()
  const state = deriveTagScanState(status)
  return (
    <span
      data-slot="tag-scan-status-badge"
      data-state={state}
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${TAG_SCAN_STATE_CLASSES[state]}`}
    >
      {t(TAG_SCAN_STATE_LABELS[state])}
    </span>
  )
}

export function AdminAnalysisTab() {
  const { t } = useTranslation()

  const [ocrStatus, setOcrStatus] = useState<OCRStatus | null>(null)
  const [isOcrLoading, setIsOcrLoading] = useState(false)
  const [ocrScanning, setOcrScanning] = useState(false)
  const [ocrScanStatus, setOcrScanStatus] = useState<OcrClassificationStatusResponse | null>(null)
  const [ocrConcurrentWorkers, setOcrConcurrentWorkers] = useState(4)
  const [isSavingWorkers, setIsSavingWorkers] = useState(false)

  const [exifStatus, setExifStatus] = useState<ExifServiceStatus | null>(null)
  const [isExifLoading, setIsExifLoading] = useState(false)

  const [llmSettings, setLlmSettings] = useState<LlmSettingsResponse>(EMPTY_SETTINGS)
  const [isLlmLoading, setIsLlmLoading] = useState(false)

  // Model cache per provider alias
  const [modelCache, setModelCache] = useState<Record<string, LlmModelDTO[]>>({})
  // Provider aliases whose last catalog refresh failed (backend reported `stale`)
  const [staleCatalogAliases, setStaleCatalogAliases] = useState<Set<string>>(new Set())
  const [loadingModelsAlias, setLoadingModelsAlias] = useState<string | null>(null)
  // Track which instrument has manual model input mode
  const [manualModelInputs, setManualModelInputs] = useState<Set<string>>(new Set())

  const [embeddingDimension, setEmbeddingDimension] = useState<number | null>(null)
  const [embeddingBatchSize, setEmbeddingBatchSize] = useState<number>(50)
  const [isEmbeddingProbing, setIsEmbeddingProbing] = useState(false)
  const [embeddingProbeError, setEmbeddingProbeError] = useState<string | null>(null)

  const isInitialLoad = useRef(true)
  const batchSizeDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const probeDimension = useCallback(async (alias: string, model: string) => {
    if (!alias || !model) return
    setIsEmbeddingProbing(true)
    setEmbeddingProbeError(null)
    try {
      const result = await probeEmbeddingDimension(alias, model)
      setEmbeddingDimension(result.dimension)
    } catch {
      setEmbeddingProbeError(t("llm_ocr.embeddingDimensionProbeFailed"))
    } finally {
      setIsEmbeddingProbing(false)
    }
  }, [t])

  // Tag Scan state
  const [tagScanEnabled, setTagScanEnabled] = useState(false)
  const [tagScanDays, setTagScanDays] = useState<boolean[]>([false, true, true, true, true, true, false]) // index 0=Sun,1=Mon..6=Sat
  const [tagScanStartHour, setTagScanStartHour] = useState(23)
  const [tagScanStartMinute, setTagScanStartMinute] = useState(0)
  const [tagScanEndHour, setTagScanEndHour] = useState(7)
  const [tagScanEndMinute, setTagScanEndMinute] = useState(0)
  const [tagScanStatus, setTagScanStatus] = useState<TagScanStatusResponse | null>(null)
  const [isTagScanSaving, setIsTagScanSaving] = useState(false)
  const [isTagScanPausing, setIsTagScanPausing] = useState(false)
  const [tagScanFormDirty, setTagScanFormDirty] = useState(false)

  const loadExifStatus = useCallback(async () => {
    try { setIsExifLoading(true); const status = await fetchExifServiceStatus(); setExifStatus(status) }
    catch { setExifStatus(null) } finally { setIsExifLoading(false) }
  }, [])

  const loadOCRStatus = useCallback(async () => {
    try { setIsOcrLoading(true); const r = await fetchOCRStatus(); setOcrStatus(r.status) }
    catch { setOcrStatus(null) } finally { setIsOcrLoading(false) }
  }, [])

  const loadSettings = useCallback(async () => {
    try { setIsLlmLoading(true); const s = await fetchLlmSettings(); setLlmSettings(s) }
    catch { setLlmSettings(EMPTY_SETTINGS) } finally { setIsLlmLoading(false) }
  }, [])

  // Load the provider catalog. Reading the stored catalog is the default; only
  // the refresh action asks the backend to re-collect it.
  const loadModels = useCallback(async (alias: string, refresh = false) => {
    if (!alias) return null
    if (!refresh && modelCache[alias]) return modelCache[alias]
    setLoadingModelsAlias(alias)
    try {
      const r = await fetchLlmModels(alias, refresh)
      if (r.success && r.models.length > 0) {
        setModelCache(p => ({ ...p, [alias]: r.models }))
        setStaleCatalogAliases(prev => {
          const next = new Set(prev)
          if (r.stale) next.add(alias)
          else next.delete(alias)
          return next
        })
        return r.models
      }
      return null
    } catch { return null } finally { setLoadingModelsAlias(null) }
  }, [modelCache])

  // Save instrument change: type + model + providerId
  const saveInstrument = useCallback(async (type: string, field: string, value: string | number) => {
    const req: Record<string, unknown> = { instrumentType: type }
    if (field === "model") req.instrumentModel = value
    if (field === "providerId") req.providerId = value
    try {
      await updateLlmSettings(req as Parameters<typeof updateLlmSettings>[0])
      toast.success(t("llm_ocr.settingsSaved"))
      await loadSettings()
    } catch { toast.error(t("llm_ocr.settingsSaveFailed")) }
  }, [loadSettings, t])

  // Poll OCR scan status
  useEffect(() => {
    if (!ocrScanning) return
    let cancelled = false
    const check = async () => {
      try { const s = await fetchOcrClassificationStatus(); if (!cancelled) { setOcrScanStatus(s); setOcrScanning(s.processing) } }
      catch { /* ignore */ }
    }
    check()
    const iv = setInterval(check, 2000)
    return () => { cancelled = true; clearInterval(iv) }
  }, [ocrScanning])

  const handleStartOcrScanAll = useCallback(async () => {
    try { await startOcrClassification(); setOcrScanning(true); toast.success(t("api.ocr.started")) }
    catch (err) { toast.error(err instanceof Error ? err.message : t("api.ocr.failed")) }
  }, [t])

  const handleStartOcrScanChanges = useCallback(async () => {
    try { await startOcrClassificationChanges(); setOcrScanning(true); toast.success(t("api.ocr.started")) }
    catch (err) { toast.error(err instanceof Error ? err.message : t("api.ocr.failed")) }
  }, [t])

  const handleStopOcrScan = useCallback(async () => {
    try { await stopOcrClassification(); toast.info("OCR scanning stopping...") }
    catch (err) { toast.error(err instanceof Error ? err.message : t("api.ocr.failed")) }
  }, [t])

  const handleSaveOcrWorkers = useCallback(async () => {
    setIsSavingWorkers(true)
    try { await updateSettings({ ocrConcurrentRequests: ocrConcurrentWorkers }); toast.success(t("adminPanel.ocr.concurrentWorkersSaved")) }
    catch (err) { toast.error(err instanceof Error ? err.message : t("adminPanel.ocr.concurrentWorkersSaveFailed")) }
    finally { setIsSavingWorkers(false) }
  }, [ocrConcurrentWorkers, t])

  const handleWorkersInputChange = useCallback((value: string) => {
    const n = parseInt(value, 10); if (!isNaN(n) && n >= 0) setOcrConcurrentWorkers(n); else if (value === "") setOcrConcurrentWorkers(0)
  }, [])

  // Tag Scan handlers
  const loadTagScanStatus = useCallback(async () => {
    try { const s = await fetchTagScanStatus(); setTagScanStatus(s) } catch { setTagScanStatus(null) }
  }, [])

  const handleSaveTagScanSchedule = useCallback(async () => {
    setIsTagScanSaving(true)
    try {
      await updateLlmSettings({ tagScanEnabled, tagScanDays: daysToString(tagScanDays), tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, tagScanTimezoneOffset: new Date().getTimezoneOffset() })
      toast.success(t("tagScan.saved"))
      setTagScanFormDirty(false); await loadTagScanStatus()
    } catch (err) { toast.error(err instanceof Error ? err.message : t("tagScan.saveFailed")) }
    finally { setIsTagScanSaving(false) }
  }, [tagScanEnabled, tagScanDays, tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, loadTagScanStatus, t])

  const toggleTagScanDay = useCallback((dayIndex: number) => {
    setTagScanDays(prev => {
      const next = [...prev]
      next[dayIndex] = !next[dayIndex]
      return next
    })
    setTagScanFormDirty(true)
  }, [])

  const handleTagScanFieldChange = useCallback((field: string, value: string | boolean | number) => {
    switch (field) {
      case "tagScanEnabled": setTagScanEnabled(value as boolean); break
      case "tagScanStartHour": setTagScanStartHour(value as number); break
      case "tagScanStartMinute": setTagScanStartMinute(value as number); break
      case "tagScanEndHour": setTagScanEndHour(value as number); break
      case "tagScanEndMinute": setTagScanEndMinute(value as number); break
    }
    setTagScanFormDirty(true)
  }, [])

  const handlePauseTagScan = useCallback(async () => {
    setIsTagScanPausing(true)
    try { await pauseTagScan(); toast.info(t("tagScan.paused")); await loadTagScanStatus() }
    catch (err) { toast.error(err instanceof Error ? err.message : t("tagScan.pauseFailed")) }
    finally { setIsTagScanPausing(false) }
  }, [loadTagScanStatus, t])

  const handleResumeTagScan = useCallback(async () => {
    setIsTagScanPausing(true)
    try { await resumeTagScan(); toast.info(t("tagScan.resumed")); await loadTagScanStatus() }
    catch (err) { toast.error(err instanceof Error ? err.message : t("tagScan.resumeFailed")) }
    finally { setIsTagScanPausing(false) }
  }, [loadTagScanStatus, t])

  // Poll tag scan status
  useEffect(() => {
    let cancelled = false; let timeoutId: ReturnType<typeof setTimeout> | null = null
    const scheduleNext = async () => {
      if (cancelled) return
      try { const s = await fetchTagScanStatus(); if (!cancelled) { setTagScanStatus(s); const d = s?.running && !s?.paused ? 10000 : 30000; timeoutId = setTimeout(scheduleNext, d) } }
      catch { if (!cancelled) { setTagScanStatus(null); timeoutId = setTimeout(scheduleNext, 30000) } }
    }
    scheduleNext()
    return () => { cancelled = true; if (timeoutId) clearTimeout(timeoutId) }
  }, [])

  // Debounced save for embedding batch size
  useEffect(() => {
    if (isInitialLoad.current) return
    if (batchSizeDebounceRef.current) clearTimeout(batchSizeDebounceRef.current)
    batchSizeDebounceRef.current = setTimeout(() => {
      updateLlmSettings({ embeddingBatchSize }).catch(() => toast.error(t("llm_ocr.settingsSaveFailed")))
    }, 800)
    return () => { if (batchSizeDebounceRef.current) clearTimeout(batchSizeDebounceRef.current) }
  }, [embeddingBatchSize, t])

  // Initial load
  useEffect(() => {
    const init = async () => {
      try { setIsOcrLoading(true); const r = await fetchOCRStatus(); setOcrStatus(r.status) }
      catch { setOcrStatus(null) } finally { setIsOcrLoading(false) }
      try {
        setIsLlmLoading(true)
        const s = await fetchLlmSettings()
        setLlmSettings(s)
        setTagScanEnabled(s.tagScan?.enabled ?? false)
        setTagScanDays(parseDaysString(s.tagScan?.days ?? "1,2,3,4,5"))
        setTagScanStartHour(s.tagScan?.startHour ?? 23)
        setTagScanStartMinute(s.tagScan?.startMinute ?? 0)
        setTagScanEndHour(s.tagScan?.endHour ?? 7)
        setTagScanEndMinute(s.tagScan?.endMinute ?? 0)
        const embInstr = s.instruments.find(i => i.type === "embedding")
        if (embInstr) {
          setEmbeddingDimension(s.embedding?.dimension || 1024)
          setEmbeddingBatchSize(s.embedding?.batchSize || 50)
          if (embInstr.providerAlias && embInstr.model) probeDimension(embInstr.providerAlias, embInstr.model)
        }
        // Populate every instrument dropdown from the stored provider catalog.
        // No refresh is forced here: the backend re-collects it on its own TTL.
        const aliases = Array.from(new Set(
          s.instruments.map(i => i.providerAlias).filter((a): a is string => Boolean(a))
        ))
        aliases.forEach(alias => { void loadModels(alias, false) })
      }
      catch { setLlmSettings(EMPTY_SETTINGS) }
      finally { isInitialLoad.current = false; setIsLlmLoading(false) }
      loadExifStatus()
      try { const st = await fetchOcrClassificationStatus(); if (st.processing) { setOcrScanning(true); setOcrScanStatus(st) } } catch { /* ignore */ }
    }
    init()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [probeDimension])

  // Load app settings
  useEffect(() => { fetchSettings().then(s => setOcrConcurrentWorkers(s.ocrConcurrentRequests ?? 4)).catch(() => {}) }, [])

  const getProviderLabel = (name: LlmProviderType): string => PROVIDER_LABELS[name] ?? name

  const chatInstrument = llmSettings.instruments.find(i => i.type === "chat")
  const vlInstrument = llmSettings.instruments.find(i => i.type === "vl")
  const ocrInstrument = llmSettings.instruments.find(i => i.type === "ocr")
  const imgEditInstrument = llmSettings.instruments.find(i => i.type === "image_edit")
  const embeddingInstrument = llmSettings.instruments.find(i => i.type === "embedding")

  // Render a single instrument card
  const renderInstrumentCard = (type: LlmInstrumentType, instrument: typeof chatInstrument) => {
    const alias = instrument?.providerAlias ?? ""
    const model = instrument?.model ?? ""
    const models = modelCache[alias] ?? []
    const compatibleModels = filterModelsForInstrument(models, type)
    const modelOptions = compatibleModels.map((m) => ({ value: m.id, label: formatModelLabel(m) }))
    const isManual = manualModelInputs.has(type)
    const isLoadingModelsVal = loadingModelsAlias === alias

    return (
      <Card key={type}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t(INSTRUMENT_LABELS[type] as never)}
          </CardTitle>
          <CardDescription>{t(INSTRUMENT_DESCRIPTIONS[type] as never)}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* Provider Selection */}
              <div className="space-y-2">
                <Label>{t("llm_ocr.provider")}</Label>
                <Select
                  value={alias}
                  onValueChange={(v) => {
                    const provider = llmSettings.providers.find(p => p.alias === v)
                    if (provider) { saveInstrument(type, "providerId", provider.id); void loadModels(v, false) }
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder={t("llm_providers.selectProvider")} />
                  </SelectTrigger>
                  <SelectContent>
                    {llmSettings.providers.map(p => (
                      <SelectItem key={p.alias} value={p.alias}>{p.alias} ({getProviderLabel(p.name)})</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Model Selection */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label>{t("llm_ocr.model")}</Label>
                  <Button variant="ghost" size="sm" onClick={() => loadModels(alias, true)} disabled={isLoadingModelsVal} className="h-6 px-2 text-xs">
                    {isLoadingModelsVal ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1 h-3 w-3" />}
                    {t("llm_providers.loadModels")}
                  </Button>
                </div>
                {compatibleModels.length > 0 && !isManual ? (
                  <div className="space-y-2">
                    <SearchableSelect
                      value={model}
                      onValueChange={(v) => saveInstrument(type, "model", v)}
                      options={modelOptions}
                      placeholder={t("llm_providers.selectModel")}
                      searchPlaceholder={t("llm_providers.searchModels")}
                      noResultsText={t("llm_providers.noMatchingModels")}
                    />
                    <Button variant="link" size="sm" className="px-0 h-auto text-xs" onClick={() => setManualModelInputs(s => new Set(s).add(type))}>
                      {t("llm_providers.enterModelManually")}
                    </Button>
                  </div>
                ) : (
                  <div className="space-y-2">
                    <Input value={model} onChange={(e) => saveInstrument(type, "model", e.target.value)} placeholder="minicpm-v" />
                    {models.length > 0 && compatibleModels.length === 0 && (
                      <p className="text-xs text-muted-foreground">{t("llm_providers.noCompatibleModels")}</p>
                    )}
                    {models.length > 0 && isManual && (
                      <Button variant="link" size="sm" className="px-0 h-auto text-xs" onClick={() => { const s = new Set(manualModelInputs); s.delete(type); setManualModelInputs(s) }}>
                        {t("llm_providers.selectFromModels")}
                      </Button>
                    )}
                  </div>
                )}
              </div>

              {staleCatalogAliases.has(alias) && (
                <p role="status" className="text-xs text-amber-600 dark:text-amber-400">
                  {t("llm_ocr.catalogStale")}
                </p>
              )}

              {!instrument && (
                <p className="text-sm text-muted-foreground text-center py-2">
                  {t("llm_providers.noModelsLoaded")}
                </p>
              )}

              {llmSettings.providers.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t("llm_providers.noProviders")}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    )
  }

  const tagState = tagScanStatus ? deriveTagScanState(tagScanStatus) : null

  return (
    <div className="space-y-6">
      {/* OCR Document Search */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            {t("adminPanel.ocr.title")}
          </CardTitle>
          <CardDescription>{t("adminPanel.ocr.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-lg bg-muted p-3">
            <div className="space-y-1">
              <div className="text-sm font-medium">{t("adminPanel.ocr.status")}</div>
              {isOcrLoading ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" />{t("common.loading")}</div>
              ) : (
                <div className="flex items-center gap-2">
                  <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                    ocrStatus?.enabled && ocrStatus?.health === "healthy" ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200" :
                    ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy") ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200" :
                    "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"}`}>
                    {ocrStatus?.enabled && ocrStatus?.health === "healthy" ? t("adminPanel.ocr.statusHealthy") :
                     ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy") ? t("adminPanel.ocr.statusError") :
                     t("adminPanel.ocr.statusDisabled")}
                  </span>
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={loadOCRStatus} disabled={isOcrLoading}>
              {isOcrLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </Button>
          </div>
          <div className="space-y-2">
            <Label htmlFor="ocr-workers-input">{t("adminPanel.ocr.concurrentWorkers")}</Label>
            <p className="text-xs text-muted-foreground">{t("adminPanel.ocr.concurrentWorkersDescription")}</p>
            <div className="flex gap-2">
              <Input id="ocr-workers-input" type="number" min={0} value={ocrConcurrentWorkers} onChange={e => handleWorkersInputChange(e.target.value)} className="w-24" />
              <Button onClick={handleSaveOcrWorkers} disabled={isSavingWorkers} size="default">{isSavingWorkers ? t("common.saving") : t("common.save")}</Button>
            </div>
          </div>
          {ocrScanning && ocrScanStatus && (
            <div className="p-4 bg-muted rounded-lg">
              <div className="flex items-center gap-2"><Loader2 className="h-4 w-4 animate-spin" /><span className="text-sm">{t("ocr.filesProcessed", { count: ocrScanStatus.filesProcessed, total: ocrScanStatus.totalFiles })}</span></div>
              <p className="text-xs text-muted-foreground mt-1">{ocrScanStatus.progress}</p>
            </div>
          )}
          <div className="flex gap-2">
            <Button onClick={handleStartOcrScanChanges} disabled={ocrScanning} variant="outline" size="sm"><Zap className={`mr-1.5 h-3.5 w-3.5 ${ocrScanning ? "animate-spin" : ""}`} />{t("adminPanel.ocr.scanChanges")}</Button>
            <Button onClick={handleStartOcrScanAll} disabled={ocrScanning} variant="outline" size="sm"><Play className={`mr-1.5 h-3.5 w-3.5 ${ocrScanning ? "animate-spin" : ""}`} />{t("adminPanel.ocr.scanAll")}</Button>
            {ocrScanning && <Button onClick={handleStopOcrScan} variant="destructive" size="sm"><Square className="mr-1.5 h-3.5 w-3.5" />{t("adminPanel.ocr.stopScanning")}</Button>}
          </div>
        </CardContent>
      </Card>

      {/* EXIF Metadata Service */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Database className="h-5 w-5" />{t("adminPanel.exif.title")}</CardTitle>
          <CardDescription>{t("adminPanel.exif.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-lg bg-muted p-3">
            <div className="space-y-1">
              <div className="text-sm font-medium">{t("adminPanel.exif.status")}</div>
              {isExifLoading ? <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2 className="h-4 w-4 animate-spin" />{t("common.loading")}</div> : (
                <div className="flex items-center gap-2">
                  <span className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                    exifStatus?.enabled && exifStatus?.health === "healthy" ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200" :
                    exifStatus?.health === "disabled" ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200" :
                    "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"}`}>
                    {exifStatus?.enabled && exifStatus?.health === "healthy" ? t("adminPanel.exif.statusHealthy") :
                     exifStatus?.health === "disabled" ? t("adminPanel.exif.statusDisabled") :
                     t("adminPanel.exif.statusUnhealthy")}
                  </span>
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={loadExifStatus} disabled={isExifLoading}>
              {isExifLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </Button>
          </div>
          {exifStatus?.error && <p className="text-xs text-destructive">{exifStatus.error}</p>}
        </CardContent>
      </Card>

      {/* Instrument Cards: Chat, VL, OCR, Image Edit */}
      {renderInstrumentCard("chat", chatInstrument)}
      {renderInstrumentCard("vl", vlInstrument)}
      {renderInstrumentCard("ocr", ocrInstrument)}
      {renderInstrumentCard("image_edit", imgEditInstrument)}

      {/* Embeddings LLM Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Wand2 className="h-5 w-5" />{t("llm_ocr.embeddingSettings")}</CardTitle>
          <CardDescription>{t("llm_ocr.embeddingSettingsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
          ) : (
            <div className="space-y-4">
              {/* Embedding provider + model (inline, not a nested card) */}
              <div className="space-y-3 rounded-lg bg-muted p-4">
                {(() => {
                  const alias = embeddingInstrument?.providerAlias ?? ""
                  const model = embeddingInstrument?.model ?? ""
                  const models = modelCache[alias] ?? []
                  const compatibleModels = filterModelsForInstrument(models, "embedding")
                  const modelOptions = compatibleModels.map((m) => ({ value: m.id, label: formatModelLabel(m) }))
                  const isManual = manualModelInputs.has("embedding")
                  const isLoadingModelsVal = loadingModelsAlias === alias
                  return (
                    <>
                      <div className="space-y-2">
                        <Label>{t("llm_ocr.provider")}</Label>
                        <Select value={alias} onValueChange={v => { const p = llmSettings.providers.find(pp => pp.alias === v); if (p) { saveInstrument("embedding", "providerId", p.id); void loadModels(v, false) } }}>
                          <SelectTrigger><SelectValue placeholder={t("llm_providers.selectProvider")} /></SelectTrigger>
                          <SelectContent>{llmSettings.providers.map(p => <SelectItem key={p.alias} value={p.alias}>{p.alias} ({getProviderLabel(p.name)})</SelectItem>)}</SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <Label>{t("llm_ocr.model")}</Label>
                          <Button variant="ghost" size="sm" onClick={() => loadModels(alias, true)} disabled={isLoadingModelsVal} className="h-6 px-2 text-xs">
                            {isLoadingModelsVal ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1 h-3 w-3" />}{t("llm_providers.loadModels")}
                          </Button>
                        </div>
                        {compatibleModels.length > 0 && !isManual ? (
                          <div className="space-y-2">
                            <SearchableSelect
                              value={model}
                              onValueChange={v => saveInstrument("embedding", "model", v)}
                              options={modelOptions}
                              placeholder={t("llm_providers.selectModel")}
                              searchPlaceholder={t("llm_providers.searchModels")}
                              noResultsText={t("llm_providers.noMatchingModels")}
                            />
                            <Button variant="link" size="sm" className="px-0 h-auto text-xs" onClick={() => setManualModelInputs(s => new Set(s).add("embedding"))}>{t("llm_providers.enterModelManually")}</Button>
                          </div>
                        ) : (
                          <div className="space-y-2">
                            <Input value={model} onChange={e => saveInstrument("embedding", "model", e.target.value)} placeholder="qwen3-embedding:4b" />
                            {models.length > 0 && compatibleModels.length === 0 && (
                              <p className="text-xs text-muted-foreground">{t("llm_providers.noCompatibleModels")}</p>
                            )}
                            {models.length > 0 && isManual && <Button variant="link" size="sm" className="px-0 h-auto text-xs" onClick={() => { const s = new Set(manualModelInputs); s.delete("embedding"); setManualModelInputs(s) }}>{t("llm_providers.selectFromModels")}</Button>}
                          </div>
                        )}
                      </div>
                      {staleCatalogAliases.has(alias) && (
                        <p role="status" className="text-xs text-amber-600 dark:text-amber-400">{t("llm_ocr.catalogStale")}</p>
                      )}
                    </>
                  )
                })()}
              </div>
              {/* Embedding Dimension */}
              <div className="space-y-2">
                <Label>{t("llm_ocr.embeddingDimension")}</Label>
                <div className="flex items-center gap-2">
                  {isEmbeddingProbing ? <><Loader2 className="h-4 w-4 animate-spin text-muted-foreground" /><span className="text-sm text-muted-foreground">{t("llm_ocr.embeddingDimensionProbing")}</span></> :
                   embeddingProbeError ? <span className="text-sm text-destructive">{embeddingProbeError}</span> :
                   embeddingDimension ? <span className="text-sm font-medium">{t("llm_ocr.embeddingDimensionDetected", { dimension: embeddingDimension })}</span> :
                   <span className="text-sm text-muted-foreground">{t("llm_ocr.embeddingDimensionUnknown")}</span>}
                </div>
                <p className="text-xs text-muted-foreground">{t("llm_ocr.embeddingDimensionDescription")}</p>
              </div>
              {/* Embedding Batch Size */}
              <div className="space-y-2">
                <Label htmlFor="embedding-batch-size">{t("llm_ocr.embeddingBatchSize")}</Label>
                <Input id="embedding-batch-size" type="number" min={1} max={500} value={embeddingBatchSize}
                  onChange={e => { const n = parseInt(e.target.value, 10); if (!isNaN(n) && n >= 1) setEmbeddingBatchSize(n); else if (e.target.value === "") setEmbeddingBatchSize(0) }}
                  className="w-32" />
                <p className="text-xs text-muted-foreground">{t("llm_ocr.embeddingBatchSizeDescription")}</p>
              </div>
              {llmSettings.providers.length === 0 && <p className="text-sm text-muted-foreground text-center py-4">{t("llm_providers.noProviders")}</p>}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Tag Scan Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><Wand2 className="h-5 w-5" />{t("tagScan.title")}</CardTitle>
          <CardDescription>{t("tagScan.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center space-x-2 rounded-lg bg-muted p-3">
            <Checkbox id="tag-scan-enabled" checked={tagScanEnabled} onCheckedChange={checked => handleTagScanFieldChange("tagScanEnabled", checked === true)} />
            <div className="space-y-0.5"><Label htmlFor="tag-scan-enabled">{t("tagScan.enabled")}</Label><p className="text-xs text-muted-foreground">{t("tagScan.description")}</p></div>
          </div>
          {tagScanEnabled && (
            <>
              <p className="text-xs text-muted-foreground">{t("tagScan.scheduleHint")}</p>
              <ScheduleEditor
                mode="window"
                idPrefix="tag-scan"
                days={tagScanDays}
                onToggleDay={toggleTagScanDay}
                startHour={tagScanStartHour}
                startMinute={tagScanStartMinute}
                endHour={tagScanEndHour}
                endMinute={tagScanEndMinute}
                onStartHourChange={v => handleTagScanFieldChange("tagScanStartHour", v)}
                onStartMinuteChange={v => handleTagScanFieldChange("tagScanStartMinute", v)}
                onEndHourChange={v => handleTagScanFieldChange("tagScanEndHour", v)}
                onEndMinuteChange={v => handleTagScanFieldChange("tagScanEndMinute", v)}
                action={
                  <Button onClick={handleSaveTagScanSchedule} disabled={isTagScanSaving || !tagScanFormDirty}>
                    {isTagScanSaving ? <><Loader2 className="mr-2 h-4 w-4 animate-spin" />{t("common.saving")}</> : t("common.save")}
                  </Button>
                }
              />
              {tagScanStatus && (
                <div className="space-y-2">
                  <Label>{t("tagScan.status")}</Label>
                  <div className="flex items-center gap-4 rounded-lg bg-muted p-3">
                    <div className="flex items-center gap-2">
                      <TagScanStatusBadge status={tagScanStatus} />
                    </div>
                    <div className="flex-1 text-sm text-muted-foreground">{tagScanStatus.scanned} {t("tagScan.of")} {tagScanStatus.total} {t("tagScan.images")}</div>
                    <div className="flex gap-2">
                      {tagState === "paused" ? (
                        <Button variant="outline" size="sm" onClick={handleResumeTagScan} disabled={isTagScanPausing || !tagScanStatus.running}><Play className="mr-1.5 h-3.5 w-3.5" />{isTagScanPausing ? t("common.saving") : t("tagScan.resume")}</Button>
                      ) : (
                        // Pause stays visible in every non-paused state and only
                        // becomes actionable while the job is actually scanning.
                        <Button variant="outline" size="sm" onClick={handlePauseTagScan} disabled={isTagScanPausing || tagState !== "running"}><Square className="mr-1.5 h-3.5 w-3.5" />{isTagScanPausing ? t("common.saving") : t("tagScan.pause")}</Button>
                      )}
                    </div>
                  </div>
                  {tagScanStatus.total > 0 && <TagScanProgress status={tagScanStatus} />}
                  {tagScanStatus.currentImage && <p className="text-xs text-muted-foreground">{t("tagScan.currentImage")}: {tagScanStatus.currentImage}</p>}
                  {tagScanStatus.lastError && <p className="text-xs text-destructive">{t("tagScan.lastError")}: {tagScanStatus.lastError}</p>}
                </div>
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

/**
 * Shared Automatic Image Tagging progress display. Renders through the common
 * {@link ProgressIndicator} primitive so the scan and tagging bars stay in sync.
 */
export function TagScanProgress({ status }: { status: TagScanStatusResponse }) {
  const { t } = useTranslation()
  const value =
    status.total > 0 ? Math.min(100, Math.round((status.scanned / status.total) * 100)) : undefined
  const statsText = `${status.scanned} ${t("tagScan.of")} ${status.total} ${t("tagScan.images")}`
  return <ProgressIndicator value={value} statsText={statsText} />
}
