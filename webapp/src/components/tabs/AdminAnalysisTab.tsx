import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Checkbox } from "@/components/ui/checkbox"
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
} from "@/api/endpoints"
import { Shield, Loader2, Zap, Wand2, Play, Square, RefreshCw, Database } from "lucide-react"
import { useTranslation } from "@/i18n"
import type {
  OCRStatus,
  OcrClassificationStatusResponse,
  LlmSettingsResponse,
  TagScanStatusResponse,
  LlmProviderType,
  ExifServiceStatus,
} from "@/types"

// Provider type display labels
const PROVIDER_LABELS: Record<LlmProviderType, string> = {
  ollama: "Ollama",
  ollama_cloud: "Ollama Cloud",
  openai: "OpenAI API compatible",
  deepseek: "DeepSeek",
  alibaba: "Alibaba Cloud",
}

const EMPTY_SETTINGS: LlmSettingsResponse = {
  id: 0,
  activeProvider: "",
  vlProvider: "",
  imgEditProvider: "",
  tagScanEnabled: false,
  tagScanStartHour: 23,
  tagScanStartMinute: 0,
  tagScanEndHour: 7,
  tagScanEndMinute: 0,
  embeddingProviderAlias: "",
  embeddingModel: "qwen3-embedding:4b",
  embeddingDimension: 1024,
  embeddingBatchSize: 50,
  providers: [],
}

export function AdminAnalysisTab() {
  const { t } = useTranslation()

  const [ocrStatus, setOcrStatus] = useState<OCRStatus | null>(null)
  const [isOcrLoading, setIsOcrLoading] = useState(false)
  const [ocrScanning, setOcrScanning] = useState(false)
  const [ocrScanStatus, setOcrScanStatus] = useState<OcrClassificationStatusResponse | null>(null)
  const [ocrConcurrentWorkers, setOcrConcurrentWorkers] = useState(4)
  const [isSavingWorkers, setIsSavingWorkers] = useState(false)

  // EXIF service status state
  const [exifStatus, setExifStatus] = useState<ExifServiceStatus | null>(null)
  const [isExifLoading, setIsExifLoading] = useState(false)

  // LLM Settings state - provider selections only (no CRUD)
  const [llmSettings, setLlmSettings] = useState<LlmSettingsResponse>(EMPTY_SETTINGS)
  const [isLlmLoading, setIsLlmLoading] = useState(false)

  // Embedding LLM Settings state
  const [embeddingProviderAlias, setEmbeddingProviderAlias] = useState("")
  const [embeddingDimension, setEmbeddingDimension] = useState<number | null>(null)
  const [embeddingBatchSize, setEmbeddingBatchSize] = useState<number>(50)
  const [isEmbeddingProbing, setIsEmbeddingProbing] = useState(false)
  const [embeddingProbeError, setEmbeddingProbeError] = useState<string | null>(null)

  // Refs for auto-save logic
  const isInitialLoad = useRef(true)
  const batchSizeDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Probe embedding dimension by calling the backend probe endpoint.
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
  const [tagScanStartHour, setTagScanStartHour] = useState(23)
  const [tagScanStartMinute, setTagScanStartMinute] = useState(0)
  const [tagScanEndHour, setTagScanEndHour] = useState(7)
  const [tagScanEndMinute, setTagScanEndMinute] = useState(0)
  const [tagScanStatus, setTagScanStatus] = useState<TagScanStatusResponse | null>(null)
  const [isTagScanSaving, setIsTagScanSaving] = useState(false)
  const [isTagScanPausing, setIsTagScanPausing] = useState(false)
  const [tagScanFormDirty, setTagScanFormDirty] = useState(false)

  const loadExifStatus = useCallback(async () => {
    try {
      setIsExifLoading(true)
      const status = await fetchExifServiceStatus()
      setExifStatus(status)
    } catch {
      setExifStatus(null)
    } finally {
      setIsExifLoading(false)
    }
  }, [])

  const loadOCRStatus = useCallback(async () => {
    try {
      setIsOcrLoading(true)
      const response = await fetchOCRStatus()
      setOcrStatus(response.status)
    } catch {
      setOcrStatus(null)
    } finally {
      setIsOcrLoading(false)
    }
  }, [])

  // Poll OCR scan status when scanning
  useEffect(() => {
    if (!ocrScanning) return

    let cancelled = false

    const checkStatus = async () => {
      try {
        const status = await fetchOcrClassificationStatus()
        if (cancelled) return
        setOcrScanStatus(status)
        setOcrScanning(status.processing)
      } catch (err) {
        console.error("Failed to check OCR scan status:", err)
      }
    }

    checkStatus()
    const interval = setInterval(() => {
      if (!cancelled) {
        checkStatus()
      }
    }, 2000)

    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [ocrScanning])

  const handleStartOcrScanAll = useCallback(async () => {
    try {
      await startOcrClassification()
      setOcrScanning(true)
      toast.success(t("api.ocr.started"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }, [t])

  const handleStartOcrScanChanges = useCallback(async () => {
    try {
      await startOcrClassificationChanges()
      setOcrScanning(true)
      toast.success(t("api.ocr.started"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }, [t])

  const handleStopOcrScan = useCallback(async () => {
    try {
      await stopOcrClassification()
      toast.info("OCR scanning stopping...")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("api.ocr.failed"))
    }
  }, [t])

  const handleSaveOcrWorkers = useCallback(async () => {
    setIsSavingWorkers(true)
    try {
      await updateSettings({ ocrConcurrentRequests: ocrConcurrentWorkers })
      toast.success(t("adminPanel.ocr.concurrentWorkersSaved"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("adminPanel.ocr.concurrentWorkersSaveFailed"))
    } finally {
      setIsSavingWorkers(false)
    }
  }, [ocrConcurrentWorkers, t])

  const handleWorkersInputChange = useCallback((value: string) => {
    const num = parseInt(value, 10)
    if (!isNaN(num) && num >= 0) {
      setOcrConcurrentWorkers(num)
    } else if (value === "") {
      setOcrConcurrentWorkers(0)
    }
  }, [])

  // Auto-save Chat LLM provider on change
  const handleActiveProviderChange = useCallback((value: string) => {
    if (isInitialLoad.current) {
      setLlmSettings((prev) => ({ ...prev, activeProvider: value }))
      return
    }
    setLlmSettings((prev) => {
      const updated = { ...prev, activeProvider: value }
      updateLlmSettings({
        activeProvider: value,
        vlProvider: updated.vlProvider,
      }).then(() => {
        toast.success(t("llm_ocr.settingsSaved"))
      }).catch(() => {
        toast.error(t("llm_ocr.settingsSaveFailed"))
      })
      return updated
    })
  }, [t])

  // Auto-save VL LLM provider on change
  const handleVlProviderChange = useCallback((value: string) => {
    if (isInitialLoad.current) {
      setLlmSettings((prev) => ({ ...prev, vlProvider: value }))
      return
    }
    setLlmSettings((prev) => {
      const updated = { ...prev, vlProvider: value }
      updateLlmSettings({
        activeProvider: updated.activeProvider,
        vlProvider: value,
      }).then(() => {
        toast.success(t("llm_ocr.settingsSaved"))
      }).catch(() => {
        toast.error(t("llm_ocr.settingsSaveFailed"))
      })
      return updated
    })
  }, [t])

  // Auto-save Image Edit LLM provider on change
  const handleImgEditProviderChange = useCallback((value: string) => {
    if (isInitialLoad.current) {
      setLlmSettings((prev) => ({ ...prev, imgEditProvider: value }))
      return
    }
    setLlmSettings((prev) => {
      const updated = { ...prev, imgEditProvider: value }
      updateLlmSettings({
        imgEditProvider: value,
      }).then(() => {
        toast.success(t("llm_ocr.settingsSaved"))
      }).catch(() => {
        toast.error(t("llm_ocr.settingsSaveFailed"))
      })
      return updated
    })
  }, [t])

  // Auto-save Embedding provider on change
  const handleEmbeddingProviderChange = useCallback((value: string) => {
    if (isInitialLoad.current) {
      setEmbeddingProviderAlias(value)
      return
    }
    setEmbeddingProviderAlias(value)
    setEmbeddingDimension(null)
    const selectedProvider = llmSettings.providers.find((p) => p.alias === value)
    if (selectedProvider?.model) {
      probeDimension(value, selectedProvider.model)
    }
    updateLlmSettings({
      embeddingProviderAlias: value,
    }).then(() => {
      toast.success(t("llm_ocr.settingsSaved"))
    }).catch(() => {
      toast.error(t("llm_ocr.settingsSaveFailed"))
    })
  }, [llmSettings.providers, probeDimension, t])

  // Tag Scan handlers
  const loadTagScanStatus = useCallback(async () => {
    try {
      const status = await fetchTagScanStatus()
      setTagScanStatus(status)
    } catch {
      setTagScanStatus(null)
    }
  }, [])

  const handleSaveTagScanSchedule = useCallback(async () => {
    setIsTagScanSaving(true)
    try {
      await updateLlmSettings({
        tagScanEnabled,
        tagScanStartHour,
        tagScanStartMinute,
        tagScanEndHour,
        tagScanEndMinute,
        tagScanTimezoneOffset: new Date().getTimezoneOffset(),
      })
      toast.success(t("tagScan.saved"))
      setTagScanFormDirty(false)
      setLlmSettings((prev) => ({
        ...prev,
        tagScanEnabled,
        tagScanStartHour,
        tagScanStartMinute,
        tagScanEndHour,
        tagScanEndMinute,
      }))
      await loadTagScanStatus()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("tagScan.saveFailed"))
    } finally {
      setIsTagScanSaving(false)
    }
  }, [tagScanEnabled, tagScanStartHour, tagScanStartMinute, tagScanEndHour, tagScanEndMinute, loadTagScanStatus, t])

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
    try {
      await pauseTagScan()
      toast.info(t("tagScan.paused"))
      await loadTagScanStatus()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("tagScan.pauseFailed"))
    } finally {
      setIsTagScanPausing(false)
    }
  }, [loadTagScanStatus, t])

  const handleResumeTagScan = useCallback(async () => {
    setIsTagScanPausing(true)
    try {
      await resumeTagScan()
      toast.info(t("tagScan.resumed"))
      await loadTagScanStatus()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("tagScan.resumeFailed"))
    } finally {
      setIsTagScanPausing(false)
    }
  }, [loadTagScanStatus, t])

  // Poll tag scan status periodically with adaptive interval
  useEffect(() => {
    let cancelled = false
    let timeoutId: ReturnType<typeof setTimeout> | null = null

    const scheduleNext = async () => {
      if (cancelled) return
      try {
        const status = await fetchTagScanStatus()
        if (cancelled) return
        setTagScanStatus(status)

        const isActive = status?.running && !status?.paused
        const nextDelay = isActive ? 10000 : 30000
        timeoutId = setTimeout(scheduleNext, nextDelay)
      } catch {
        if (!cancelled) {
          setTagScanStatus(null)
        }
        timeoutId = setTimeout(scheduleNext, 30000)
      }
    }

    scheduleNext()

    return () => {
      cancelled = true
      if (timeoutId) clearTimeout(timeoutId)
    }
  }, [])

  // Debounced auto-save for embedding batch size
  useEffect(() => {
    if (isInitialLoad.current) return

    if (batchSizeDebounceRef.current) {
      clearTimeout(batchSizeDebounceRef.current)
    }

    batchSizeDebounceRef.current = setTimeout(() => {
      updateLlmSettings({ embeddingBatchSize }).catch(() => {
        toast.error(t("llm_ocr.settingsSaveFailed"))
      })
    }, 800)

    return () => {
      if (batchSizeDebounceRef.current) {
        clearTimeout(batchSizeDebounceRef.current)
      }
    }
  }, [embeddingBatchSize, t])

  useEffect(() => {
    const init = async () => {
      // Load OCR status
      try {
        setIsOcrLoading(true)
        const response = await fetchOCRStatus()
        setOcrStatus(response.status)
      } catch {
        setOcrStatus(null)
      } finally {
        setIsOcrLoading(false)
      }

      // Load LLM settings
      try {
        setIsLlmLoading(true)
        const settings = await fetchLlmSettings()
        setLlmSettings(settings)
        setTagScanEnabled(settings.tagScanEnabled ?? false)
        setTagScanStartHour(settings.tagScanStartHour ?? 23)
        setTagScanStartMinute(settings.tagScanStartMinute ?? 0)
        setTagScanEndHour(settings.tagScanEndHour ?? 7)
        setTagScanEndMinute(settings.tagScanEndMinute ?? 0)
        const embAlias = settings.embeddingProviderAlias || settings.activeProvider
        setEmbeddingProviderAlias(embAlias)
        setEmbeddingDimension(settings.embeddingDimension || 1024)
        setEmbeddingBatchSize(settings.embeddingBatchSize || 50)
        // Probe dimension using the provider's configured model
        const embProvider = settings.providers.find((p) => p.alias === embAlias)
        if (embAlias && embProvider?.model) {
          probeDimension(embAlias, embProvider.model)
        }
      } catch {
        setLlmSettings(EMPTY_SETTINGS)
      } finally {
        isInitialLoad.current = false
        setIsLlmLoading(false)
      }

      // Load EXIF status
      loadExifStatus()

      // Check initial OCR classification status
      try {
        const status = await fetchOcrClassificationStatus()
        if (status.processing) {
          setOcrScanning(true)
          setOcrScanStatus(status)
        }
      } catch {
        // Ignore errors on initial check
      }
    }

    init()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [probeDimension])

  // Load app settings to sync ocrConcurrentWorkers
  useEffect(() => {
    fetchSettings().then((settings) => {
      setOcrConcurrentWorkers(settings.ocrConcurrentRequests ?? 4)
    }).catch(() => {
      // Use local state values
    })
  }, [])

  // Provider type display name lookup
  const getProviderLabel = (name: LlmProviderType): string => PROVIDER_LABELS[name] ?? name

  // Get current provider info for display
  const currentProvider = useMemo(
    () => llmSettings.providers.find((p) => p.alias === llmSettings.activeProvider),
    [llmSettings.providers, llmSettings.activeProvider],
  )

  const currentVLProvider = useMemo(
    () => llmSettings.providers.find((p) => p.alias === llmSettings.vlProvider),
    [llmSettings.providers, llmSettings.vlProvider],
  )

  const currentImgEditProvider = useMemo(
    () => llmSettings.providers.find((p) => p.alias === llmSettings.imgEditProvider),
    [llmSettings.providers, llmSettings.imgEditProvider],
  )

  const currentEmbeddingProvider = useMemo(
    () => llmSettings.providers.find((p) => p.alias === embeddingProviderAlias),
    [llmSettings.providers, embeddingProviderAlias],
  )

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
          {/* OCR Service Status */}
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-1">
              <div className="text-sm font-medium">{t("adminPanel.ocr.status")}</div>
              {isOcrLoading ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t("common.loading")}
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      ocrStatus?.enabled && ocrStatus?.health === "healthy"
                        ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                        : ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy")
                        ? "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                        : "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                    }`}
                  >
                    {ocrStatus?.enabled && ocrStatus?.health === "healthy"
                      ? t("adminPanel.ocr.statusHealthy")
                      : ocrStatus?.error || (ocrStatus?.enabled && ocrStatus?.health !== "healthy")
                      ? t("adminPanel.ocr.statusError")
                      : t("adminPanel.ocr.statusDisabled")}
                  </span>
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={loadOCRStatus} disabled={isOcrLoading}>
              {isOcrLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </Button>
          </div>

          {/* Concurrent Workers */}
          <div className="space-y-2">
            <Label htmlFor="ocr-workers-input">{t("adminPanel.ocr.concurrentWorkers")}</Label>
            <p className="text-xs text-muted-foreground">{t("adminPanel.ocr.concurrentWorkersDescription")}</p>
            <div className="flex gap-2">
              <Input
                id="ocr-workers-input"
                type="number"
                min={0}
                value={ocrConcurrentWorkers}
                onChange={(e) => handleWorkersInputChange(e.target.value)}
                className="w-24"
              />
              <Button
                onClick={handleSaveOcrWorkers}
                disabled={isSavingWorkers}
                size="default"
              >
                {isSavingWorkers ? t("common.saving") : t("common.save")}
              </Button>
            </div>
          </div>

          {/* OCR Scan Progress */}
          {ocrScanning && ocrScanStatus && (
            <div className="p-4 bg-muted rounded-lg">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span className="text-sm">
                  {t("ocr.filesProcessed", {
                    count: ocrScanStatus.filesProcessed,
                    total: ocrScanStatus.totalFiles,
                  })}
                </span>
              </div>
              <p className="text-xs text-muted-foreground mt-1">{ocrScanStatus.progress}</p>
            </div>
          )}

          {/* Scan Buttons */}
          <div className="flex gap-2">
            <Button
              onClick={handleStartOcrScanChanges}
              disabled={ocrScanning}
              variant="outline"
              size="sm"
            >
              <Zap className={`mr-1.5 h-3.5 w-3.5 ${ocrScanning ? "animate-spin" : ""}`} />
              {t("adminPanel.ocr.scanChanges")}
            </Button>
            <Button
              onClick={handleStartOcrScanAll}
              disabled={ocrScanning}
              variant="outline"
              size="sm"
            >
              <Play className={`mr-1.5 h-3.5 w-3.5 ${ocrScanning ? "animate-spin" : ""}`} />
              {t("adminPanel.ocr.scanAll")}
            </Button>
            {ocrScanning && (
              <Button
                onClick={handleStopOcrScan}
                variant="destructive"
                size="sm"
              >
                <Square className="mr-1.5 h-3.5 w-3.5" />
                {t("adminPanel.ocr.stopScanning")}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* EXIF Metadata Service */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Database className="h-5 w-5" />
            {t("adminPanel.exif.title")}
          </CardTitle>
          <CardDescription>{t("adminPanel.exif.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-1">
              <div className="text-sm font-medium">{t("adminPanel.exif.status")}</div>
              {isExifLoading ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  {t("common.loading")}
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                      exifStatus?.enabled && exifStatus?.health === "healthy"
                        ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                        : exifStatus?.health === "disabled"
                        ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                        : "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200"
                    }`}
                  >
                    {exifStatus?.enabled && exifStatus?.health === "healthy"
                      ? t("adminPanel.exif.statusHealthy")
                      : exifStatus?.health === "disabled"
                      ? t("adminPanel.exif.statusDisabled")
                      : t("adminPanel.exif.statusUnhealthy")}
                  </span>
                </div>
              )}
            </div>
            <Button variant="outline" size="sm" onClick={loadExifStatus} disabled={isExifLoading}>
              {isExifLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            </Button>
          </div>

          {exifStatus?.error && (
            <p className="text-xs text-destructive">{exifStatus.error}</p>
          )}
        </CardContent>
      </Card>

      {/* Chat LLM Settings — provider selection only */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("llm_ocr.chatSettings")}
          </CardTitle>
          <CardDescription>{t("llm_ocr.chatSettingsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* Chat Provider Selection */}
              <div className="space-y-2">
                <Label htmlFor="chat-llm-provider">{t("llm_ocr.provider")}</Label>
                <Select
                  value={llmSettings.activeProvider}
                  onValueChange={handleActiveProviderChange}
                >
                  <SelectTrigger id="chat-llm-provider">
                    <SelectValue placeholder={t("llm_providers.selectProvider")} />
                  </SelectTrigger>
                  <SelectContent>
                    {llmSettings.providers.map((p) => (
                      <SelectItem key={p.alias} value={p.alias}>
                        {p.alias} ({getProviderLabel(p.name)})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Current provider info */}
              {currentProvider && (
                <div className="rounded-lg border p-3 text-sm text-muted-foreground">
                  <span className="font-medium text-foreground">
                    {currentProvider.alias}
                  </span>{" "}
                  ({getProviderLabel(currentProvider.name)}) — {t("llm_ocr.model")}:{" "}
                  <span className="font-mono text-xs">{currentProvider.model}</span>
                  <span className="block mt-1 text-xs text-muted-foreground">
                    {t("llm_providers.manageInProvidersTab")}
                  </span>
                </div>
              )}

              {/* No providers message */}
              {llmSettings.providers.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t("llm_providers.noProviders")}
                </p>
              )}

            </div>
          )}
        </CardContent>
      </Card>

      {/* VL LLM Settings — provider selection only */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("llm_ocr.vlSettings")}
          </CardTitle>
          <CardDescription>{t("llm_ocr.vlSettingsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* VL Provider Selection */}
              <div className="space-y-2">
                <Label htmlFor="vl-llm-provider">{t("llm_ocr.provider")}</Label>
                <Select
                  value={llmSettings.vlProvider}
                  onValueChange={handleVlProviderChange}
                >
                  <SelectTrigger id="vl-llm-provider">
                    <SelectValue placeholder={t("llm_providers.selectProvider")} />
                  </SelectTrigger>
                  <SelectContent>
                    {llmSettings.providers.map((p) => (
                      <SelectItem key={p.alias} value={p.alias}>
                        {p.alias} ({getProviderLabel(p.name)})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* VL Provider info */}
              {currentVLProvider && currentVLProvider.alias !== currentProvider?.alias && (
                <div className="rounded-lg border p-3 text-sm text-muted-foreground">
                  <span className="font-medium text-foreground">
                    {currentVLProvider.alias}
                  </span>{" "}
                  ({getProviderLabel(currentVLProvider.name)}) — {t("llm_ocr.model")}:{" "}
                  <span className="font-mono text-xs">{currentVLProvider.model}</span>
                </div>
              )}

              {currentVLProvider && currentVLProvider.alias === currentProvider?.alias && (
                <p className="text-sm text-muted-foreground py-2">
                  {t("llm_ocr.vlSameAsChat")}
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

      {/* Image Edit LLM Settings — provider selection only */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("llm_ocr.imgEditSettings")}
          </CardTitle>
          <CardDescription>{t("llm_ocr.imgEditSettingsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* Image Edit Provider Selection */}
              <div className="space-y-2">
                <Label htmlFor="img-edit-llm-provider">{t("llm_ocr.provider")}</Label>
                <Select
                  value={llmSettings.imgEditProvider}
                  onValueChange={handleImgEditProviderChange}
                >
                  <SelectTrigger id="img-edit-llm-provider">
                    <SelectValue placeholder={t("llm_providers.selectProvider")} />
                  </SelectTrigger>
                  <SelectContent>
                    {llmSettings.providers.map((p) => (
                      <SelectItem key={p.alias} value={p.alias}>
                        {p.alias} ({getProviderLabel(p.name)})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Image Edit Provider info */}
              {currentImgEditProvider && currentImgEditProvider.alias !== currentVLProvider?.alias && currentImgEditProvider.alias !== currentProvider?.alias && (
                <div className="rounded-lg border p-3 text-sm text-muted-foreground">
                  <span className="font-medium text-foreground">
                    {currentImgEditProvider.alias}
                  </span>{" "}
                  ({getProviderLabel(currentImgEditProvider.name)}) — {t("llm_ocr.model")}:{" "}
                  <span className="font-mono text-xs">{currentImgEditProvider.model}</span>
                </div>
              )}

              {currentImgEditProvider && currentImgEditProvider.alias === currentVLProvider?.alias && (
                <p className="text-sm text-muted-foreground py-2">
                  {t("llm_ocr.imgEditSameAsVL")}
                </p>
              )}

              {(!currentImgEditProvider || (currentImgEditProvider.alias !== currentVLProvider?.alias && currentImgEditProvider.alias === currentProvider?.alias)) && currentImgEditProvider?.alias === currentProvider?.alias && (
                <p className="text-sm text-muted-foreground py-2">
                  {t("llm_ocr.imgEditSameAsChat")}
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

      {/* Embeddings LLM Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("llm_ocr.embeddingSettings")}
          </CardTitle>
          <CardDescription>{t("llm_ocr.embeddingSettingsDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLlmLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <div className="space-y-4">
              {/* Embedding Provider Selection */}
              <div className="space-y-2">
                <Label htmlFor="embedding-provider">{t("llm_ocr.embeddingProvider")}</Label>
                <Select
                  value={embeddingProviderAlias}
                  onValueChange={handleEmbeddingProviderChange}
                >
                  <SelectTrigger id="embedding-provider">
                    <SelectValue placeholder={t("llm_providers.selectProvider")} />
                  </SelectTrigger>
                  <SelectContent>
                    {llmSettings.providers.map((p) => (
                      <SelectItem key={p.alias} value={p.alias}>
                        {p.alias} ({getProviderLabel(p.name)})
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Embedding Provider info (read-only, model from provider config) */}
              {currentEmbeddingProvider && (
                <div className="rounded-lg border p-3 text-sm text-muted-foreground">
                  <span className="font-medium text-foreground">
                    {currentEmbeddingProvider.alias}
                  </span>{" "}
                  ({getProviderLabel(currentEmbeddingProvider.name)}) — {t("llm_ocr.model")}:{" "}
                  <span className="font-mono text-xs">{currentEmbeddingProvider.model}</span>
                </div>
              )}

              {/* Embedding Dimension (auto-detected) */}
              <div className="space-y-2">
                <Label>{t("llm_ocr.embeddingDimension")}</Label>
                <div className="flex items-center gap-2">
                  {isEmbeddingProbing ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                      <span className="text-sm text-muted-foreground">{t("llm_ocr.embeddingDimensionProbing")}</span>
                    </>
                  ) : embeddingProbeError ? (
                    <span className="text-sm text-destructive">{embeddingProbeError}</span>
                  ) : embeddingDimension ? (
                    <span className="text-sm font-medium">{t("llm_ocr.embeddingDimensionDetected", { dimension: embeddingDimension })}</span>
                  ) : (
                    <span className="text-sm text-muted-foreground">{t("llm_ocr.embeddingDimensionUnknown")}</span>
                  )}
                </div>
                <p className="text-xs text-muted-foreground">{t("llm_ocr.embeddingDimensionDescription")}</p>
              </div>

              {/* Embedding Batch Size */}
              <div className="space-y-2">
                <Label htmlFor="embedding-batch-size">{t("llm_ocr.embeddingBatchSize")}</Label>
                <Input
                  id="embedding-batch-size"
                  type="number"
                  min={1}
                  max={500}
                  value={embeddingBatchSize}
                  onChange={(e) => {
                    const num = parseInt(e.target.value, 10)
                    if (!isNaN(num) && num >= 1) {
                      setEmbeddingBatchSize(num)
                    } else if (e.target.value === "") {
                      setEmbeddingBatchSize(0)
                    }
                  }}
                  className="w-32"
                />
                <p className="text-xs text-muted-foreground">{t("llm_ocr.embeddingBatchSizeDescription")}</p>
              </div>

              {/* No providers message */}
              {llmSettings.providers.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-4">
                  {t("llm_providers.noProviders")}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Tag Scan Settings */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Wand2 className="h-5 w-5" />
            {t("tagScan.title")}
          </CardTitle>
          <CardDescription>{t("tagScan.description")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Enable/Disable Checkbox */}
          <div className="flex items-center space-x-2 rounded-lg border p-3">
            <Checkbox
              id="tag-scan-enabled"
              checked={tagScanEnabled}
              onCheckedChange={(checked) => handleTagScanFieldChange("tagScanEnabled", checked === true)}
            />
            <div className="space-y-0.5">
              <Label htmlFor="tag-scan-enabled">{t("tagScan.enabled")}</Label>
              <p className="text-xs text-muted-foreground">
                {t("tagScan.description")}
              </p>
            </div>
          </div>

          {tagScanEnabled && (
            <>
              {/* Schedule */}
              <div className="space-y-2">
                <Label>{t("tagScan.schedule")}</Label>
                <div className="flex items-center gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="tag-scan-start-hour">{t("tagScan.startTime")}</Label>
                    <div className="flex gap-2">
                      <Select
                        value={String(tagScanStartHour)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanStartHour", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-start-hour" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                            <SelectItem key={h} value={String(h)}>
                              {String(h).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <span className="self-center text-muted-foreground">:</span>
                      <Select
                        value={String(tagScanStartMinute)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanStartMinute", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-start-minute" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 12 }, (_, i) => i * 5).map((m) => (
                            <SelectItem key={m} value={String(m)}>
                              {String(m).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>

                  <div className="space-y-2">
                    <Label htmlFor="tag-scan-end-hour">{t("tagScan.endTime")}</Label>
                    <div className="flex gap-2">
                      <Select
                        value={String(tagScanEndHour)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanEndHour", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-end-hour" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 24 }, (_, i) => i).map((h) => (
                            <SelectItem key={h} value={String(h)}>
                              {String(h).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <span className="self-center text-muted-foreground">:</span>
                      <Select
                        value={String(tagScanEndMinute)}
                        onValueChange={(val) => handleTagScanFieldChange("tagScanEndMinute", Number(val))}
                      >
                        <SelectTrigger id="tag-scan-end-minute" className="w-20">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {Array.from({ length: 12 }, (_, i) => i * 5).map((m) => (
                            <SelectItem key={m} value={String(m)}>
                              {String(m).padStart(2, "0")}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                </div>
              </div>

              {/* Status and Progress */}
              {tagScanStatus && (
                <div className="space-y-2">
                  <Label>{t("tagScan.status")}</Label>
                  <div className="flex items-center gap-4 rounded-lg border p-3">
                    <div className="flex items-center gap-2">
                      <span
                        className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${
                          tagScanStatus.running && !tagScanStatus.paused
                            ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
                            : tagScanStatus.paused
                            ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200"
                            : "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200"
                        }`}
                      >
                        {tagScanStatus.running && !tagScanStatus.paused
                          ? t("tagScan.running")
                          : tagScanStatus.paused
                          ? t("tagScan.paused")
                          : t("tagScan.stopped")}
                      </span>
                    </div>

                    <div className="flex-1 text-sm text-muted-foreground">
                      {tagScanStatus.scanned} {t("tagScan.of")} {tagScanStatus.total} {t("tagScan.images")}
                    </div>

                    <div className="flex gap-2">
                      {tagScanStatus.running && !tagScanStatus.paused ? (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handlePauseTagScan}
                          disabled={isTagScanPausing}
                        >
                          <Square className="mr-1.5 h-3.5 w-3.5" />
                          {isTagScanPausing ? t("common.saving") : t("tagScan.pause")}
                        </Button>
                      ) : (
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={handleResumeTagScan}
                          disabled={isTagScanPausing || !tagScanStatus.running}
                        >
                          <Play className="mr-1.5 h-3.5 w-3.5" />
                          {isTagScanPausing ? t("common.saving") : t("tagScan.resume")}
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* Progress Bar */}
                  {tagScanStatus.total > 0 && (
                    <div className="w-full h-2 bg-muted rounded-full overflow-hidden">
                      <div
                        className="h-full bg-primary transition-all duration-300"
                        style={{ width: `${(tagScanStatus.scanned / tagScanStatus.total) * 100}%` }}
                      />
                    </div>
                  )}

                  {/* Current Image */}
                  {tagScanStatus.currentImage && (
                    <p className="text-xs text-muted-foreground">
                      {t("tagScan.currentImage")}: {tagScanStatus.currentImage}
                    </p>
                  )}

                  {/* Last Error */}
                  {tagScanStatus.lastError && (
                    <p className="text-xs text-destructive">
                      {t("tagScan.lastError")}: {tagScanStatus.lastError}
                    </p>
                  )}
                </div>
              )}

              {/* Save Button */}
              <div className="flex justify-end pt-2">
                <Button
                  onClick={handleSaveTagScanSchedule}
                  disabled={isTagScanSaving || !tagScanFormDirty}
                >
                  {isTagScanSaving ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t("common.saving")}
                    </>
                  ) : (
                    t("tagScan.save")
                  )}
                </Button>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
