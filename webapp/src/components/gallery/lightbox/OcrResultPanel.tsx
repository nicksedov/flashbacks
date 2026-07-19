import { Loader2, Wand2, Download, ScanText, Sparkles, AlertTriangle, WifiOff, Settings } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { OcrDataResponse, LlmOcrDataResponse } from "@/types"
import { OcrMarkdownRenderer } from "./OcrMarkdownRenderer"
import { Button } from "@/components/ui/button"

interface OcrResultPanelProps {
  ocrData: OcrDataResponse | null
  llmData: LlmOcrDataResponse | null
  recognizing: boolean
  onRecognize: () => void
  onSaveMd: () => void
  onSaveHtml: () => void
  formatProcessingTime: (ms?: number) => string
  className?: string
}

function detectLanguageFromOcr(ocrData: OcrDataResponse): string {
  let ruCount = 0
  let enCount = 0
  for (const box of ocrData.boxes) {
    for (const ch of box.word.toLowerCase()) {
      if (ch.charCodeAt(0) >= 0x0400 && ch.charCodeAt(0) <= 0x04FF) ruCount++
      if (ch.charCodeAt(0) >= 0x0061 && ch.charCodeAt(0) <= 0x007A) enCount++
    }
  }
  return ruCount > enCount ? "Русский" : "English"
}

/**
 * Categorizes an error message to provide contextual advice to the user.
 * Returns an object with an icon and a hint key for additional guidance.
 */
function categorizeError(error: string): { icon: React.ReactNode; hintKey: string } {
  const lower = error.toLowerCase()
  if (
    lower.includes("unreachable") ||
    lower.includes("failed to fetch") ||
    lower.includes("network") ||
    lower.includes("connection") ||
    lower.includes("timeout") ||
    lower.includes("abort")
  ) {
    return {
      icon: <WifiOff className="h-4 w-4 shrink-0" />,
      hintKey: "llm_ocr.errorHintNetwork",
    }
  }
  if (
    lower.includes("instrument not configured") ||
    lower.includes("not configured") ||
    lower.includes("settings not found") ||
    lower.includes("provider not found")
  ) {
    return {
      icon: <Settings className="h-4 w-4 shrink-0" />,
      hintKey: "llm_ocr.errorHintConfig",
    }
  }
  if (
    lower.includes("api key") ||
    lower.includes("unauthorized") ||
    lower.includes("authentication") ||
    lower.includes("invalid")
  ) {
    return {
      icon: <AlertTriangle className="h-4 w-4 shrink-0" />,
      hintKey: "llm_ocr.errorHintAuth",
    }
  }
  return {
    icon: <AlertTriangle className="h-4 w-4 shrink-0" />,
    hintKey: "llm_ocr.errorHintGeneral",
  }
}

function ErrorDisplay({ error }: { error: string }) {
  const { t } = useTranslation()
  const { icon, hintKey } = categorizeError(error)

  return (
    <div className="space-y-3">
      {/* Error message */}
      <div className="flex items-start gap-2 p-3 rounded-md bg-destructive/10 border border-destructive/20">
        {icon}
        <div className="min-w-0">
          <p className="text-xs font-medium text-destructive break-words">{error}</p>
        </div>
      </div>

      {/* Actionable hint */}
      <p className="text-xs text-muted-foreground leading-relaxed">
        {t(hintKey as import("@/i18n").TranslationKey)}
      </p>
    </div>
  )
}

export function OcrResultPanel({
  ocrData,
  llmData,
  recognizing,
  onRecognize,
  onSaveMd,
  onSaveHtml,
  formatProcessingTime,
  className,
}: OcrResultPanelProps) {
  const { t } = useTranslation()
  const panelClass = className ?? "w-[50%] bg-card border-l p-4 h-full flex flex-col"

  if (recognizing) {
    return (
      <div className={panelClass}>
        <div className="flex flex-col items-center justify-center h-full">
          <Loader2 className="h-8 w-8 animate-spin text-primary mb-3" />
          <p className="text-sm font-medium">{t("llm_ocr.recognizing")}</p>
        </div>
      </div>
    )
  }

  if (llmData?.found && llmData.success && llmData.markdownContent) {
    return (
      <div className={panelClass}>
        <div className="flex flex-col h-full">
          <div className="shrink-0 space-y-4">
            <h3 className="text-sm font-semibold mb-3">{t("llm_ocr.title")}</h3>

            {/* Result section */}
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <Sparkles className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("llm_ocr.sectionResult")}</span>
              </div>
              <div className="space-y-1.5">
                <div className="flex justify-between items-baseline gap-2 text-xs">
                  <span className="text-muted-foreground shrink-0">{t("llm_ocr.language")}</span>
                  <span className="font-medium text-right truncate">{llmData.language === "ru" ? "Русский" : "English"}</span>
                </div>
                <div className="flex justify-between items-baseline gap-2 text-xs">
                  <span className="text-muted-foreground shrink-0">{t("llm_ocr.provider")}</span>
                  <span className="font-medium text-right truncate">{llmData.provider}</span>
                </div>
                <div className="flex justify-between items-baseline gap-2 text-xs">
                  <span className="text-muted-foreground shrink-0">{t("llm_ocr.model")}</span>
                  <span className="font-medium text-right truncate">{llmData.model}</span>
                </div>
                <div className="flex justify-between items-baseline gap-2 text-xs">
                  <span className="text-muted-foreground shrink-0">{t("llm_ocr.processingTime")}</span>
                  <span className="font-medium text-right truncate">{formatProcessingTime(llmData.processingTimeMs)}</span>
                </div>
              </div>
            </div>

            {/* Actions */}
            <div className="flex gap-2">
              <Button variant="outline" size="sm" className="flex-1 text-xs" onClick={onSaveMd}>
                <Download className="h-3.5 w-3.5 mr-1.5" />
                .md
              </Button>
              <Button variant="outline" size="sm" className="flex-1 text-xs" onClick={onSaveHtml}>
                <Download className="h-3.5 w-3.5 mr-1.5" />
                .html
              </Button>
              <Button variant="outline" size="sm" className="text-xs" onClick={onRecognize}>
                <Wand2 className="h-3.5 w-3.5 mr-1.5" />
                {t("llm_ocr.recognizeButton")}
              </Button>
            </div>
          </div>

          {/* Markdown content — scrollable */}
          <div className="mt-4 overflow-y-auto min-h-0">
            <div className="p-4 bg-muted rounded-lg markdown-body">
              <OcrMarkdownRenderer content={llmData.markdownContent} />
            </div>
          </div>
        </div>
      </div>
    )
  }

  if (llmData?.error) {
    return (
      <div className={panelClass}>
        <div className="h-full overflow-y-auto">
          <div className="space-y-4">
            <h3 className="text-sm font-semibold">{t("llm_ocr.title")}</h3>
            <ErrorDisplay error={llmData.error} />
            <Button variant="outline" size="sm" className="w-full text-xs" onClick={onRecognize}>
              <Wand2 className="h-3.5 w-3.5 mr-1.5" />
              {t("llm_ocr.recognizeButton")}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className={panelClass}>
      <div className="h-full overflow-y-auto">
        <div className="space-y-4">
          <h3 className="text-sm font-semibold mb-3">{t("llm_ocr.title")}</h3>

          {ocrData && (
            <div>
              <div className="flex items-center gap-1.5 mb-2">
                <ScanText className="h-3.5 w-3.5 text-muted-foreground" />
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("llm_ocr.sectionDetected")}</span>
              </div>
              <div className="space-y-1.5">
                <div className="flex justify-between items-baseline gap-2 text-xs">
                  <span className="text-muted-foreground shrink-0">{t("llm_ocr.language")}</span>
                  <span className="font-medium text-right truncate">{detectLanguageFromOcr(ocrData)}</span>
                </div>
              </div>
            </div>
          )}

          <Button variant="outline" size="sm" className="w-full text-xs" onClick={onRecognize}>
            <Wand2 className="h-3.5 w-3.5 mr-1.5" />
            {t("llm_ocr.recognizeButton")}
          </Button>

          <p className="text-xs text-muted-foreground text-center">
            {t("llm_ocr.description")}
          </p>
        </div>
      </div>
    </div>
  )
}
