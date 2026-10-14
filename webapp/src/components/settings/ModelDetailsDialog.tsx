import type { ReactNode } from "react"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { useTranslation } from "@/i18n"
import type { LlmModelDTO } from "@/types"

/** Formats a context window length in tokens as a compact human-readable string. */
function formatContextLength(len?: number): string {
  if (!len) return "—"
  if (len >= 1_000_000) return `${(len / 1_000_000).toFixed(1)}M`
  if (len >= 1_000) return `${Math.round(len / 1_000)}K`
  return String(len)
}

/** Formats a per-1M-token price in USD, using an em dash when unknown. */
function formatPricePerMTok(price?: number): string {
  if (!price) return "—"
  return `$${price.toFixed(2)}`
}

interface ModelDetailsDialogProps {
  model: LlmModelDTO | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right font-medium">{children}</dd>
    </div>
  )
}

/**
 * Presentational dialog that shows the collected metadata of an LLM model:
 * context length, input/output modalities, capabilities, and token pricing.
 */
export function ModelDetailsDialog({ model, open, onOpenChange }: ModelDetailsDialogProps) {
  const { t } = useTranslation()

  if (!model) return null

  const inputModalities =
    model.inputModalities && model.inputModalities.length > 0 ? model.inputModalities.join(", ") : "—"
  const outputModalities =
    model.outputModalities && model.outputModalities.length > 0 ? model.outputModalities.join(", ") : "—"
  const capabilities = model.capabilities ?? []

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{model.name}</DialogTitle>
          <DialogDescription className="font-mono text-xs">{model.id}</DialogDescription>
        </DialogHeader>
        <dl className="space-y-2 text-sm">
          <Row label={t("llm_model_details.contextLength")}>
            {formatContextLength(model.contextLength)}
          </Row>
          <Row label={t("llm_model_details.inputModalities")}>{inputModalities}</Row>
          <Row label={t("llm_model_details.outputModalities")}>{outputModalities}</Row>
          <Row label={t("llm_model_details.capabilities")}>
            {capabilities.length > 0 ? (
              <span className="flex flex-wrap justify-end gap-1">
                {capabilities.map((cap) => (
                  <Badge key={cap} variant="outline" className="h-4 px-1 py-0 text-[10px]">
                    {cap}
                  </Badge>
                ))}
              </span>
            ) : (
              "—"
            )}
          </Row>
          <Row label={t("llm_model_details.inputPrice")}>
            {formatPricePerMTok(model.inputPricePerMTok)}
          </Row>
          <Row label={t("llm_model_details.outputPrice")}>
            {formatPricePerMTok(model.outputPricePerMTok)}
          </Row>
        </dl>
      </DialogContent>
    </Dialog>
  )
}
