import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Loader2, RefreshCw, Trash2, Pencil, X } from "lucide-react"
import { useTranslation } from "@/i18n"
import type { LlmProviderDTO, LlmModelDTO, LlmProviderType } from "@/types"

// Provider type display labels
const PROVIDER_LABELS: Record<LlmProviderType, string> = {
  ollama: "Ollama",
  ollama_cloud: "Ollama Cloud",
  openai: "OpenAI API compatible",
  deepseek: "DeepSeek",
  alibaba: "Alibaba Cloud",
}

interface ProviderConfigFormProps {
  provider: LlmProviderDTO
  providers: LlmProviderDTO[]
  availableModels: LlmModelDTO[]
  isModelsLoading: boolean
  showModelInput: boolean
  onFieldChange: (alias: string, field: keyof LlmProviderDTO, value: string | boolean) => void
  onAliasUpdate: (oldAlias: string, newAlias: string) => Promise<void>
  onDelete: (alias: string) => Promise<void>
  onLoadModels: () => void
  onToggleModelInput: (show: boolean) => void
  isSaving: boolean
  namePrefix: string
}

export function ProviderConfigForm({
  provider,
  providers,
  availableModels,
  isModelsLoading,
  showModelInput,
  onFieldChange,
  onAliasUpdate,
  onDelete,
  onLoadModels,
  onToggleModelInput,
  isSaving,
  namePrefix,
}: ProviderConfigFormProps) {
  const { t } = useTranslation()
  const [editingAlias, setEditingAlias] = useState(provider.alias)
  const [isEditingAlias, setIsEditingAlias] = useState(false)

  // Sync editing alias when provider changes (e.g., dropdown switch)
  if (!isEditingAlias && editingAlias !== provider.alias) {
    setEditingAlias(provider.alias)
  }

  const getProviderLabel = (name: LlmProviderType): string => PROVIDER_LABELS[name] ?? name

  return (
    <div className="space-y-4 rounded-lg border p-4">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">
          {t("llm_providers.providerLabel", { alias: provider.alias })}
          <span className="ml-2 text-xs text-muted-foreground">
            ({getProviderLabel(provider.name)})
          </span>
        </h4>
        <div className="flex gap-2">
          {providers.length > 1 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => onDelete(provider.alias)}
              className="h-8 w-8 p-0 text-destructive"
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          )}
        </div>
      </div>

      {/* Alias field */}
      <div className="space-y-2">
        <Label htmlFor={`${namePrefix}-alias`}>{t("llm_providers.alias")}</Label>
        {isEditingAlias ? (
          <div className="flex gap-2">
            <Input
              id={`${namePrefix}-alias`}
              value={editingAlias}
              onChange={(e) => setEditingAlias(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && editingAlias !== provider.alias && editingAlias.trim()) {
                  onAliasUpdate(provider.alias, editingAlias).then(() => setIsEditingAlias(false))
                } else if (e.key === "Escape") {
                  setEditingAlias(provider.alias)
                  setIsEditingAlias(false)
                }
              }}
              disabled={isSaving}
              autoFocus
            />
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                if (editingAlias !== provider.alias) {
                  onAliasUpdate(provider.alias, editingAlias).then(() => setIsEditingAlias(false))
                }
              }}
              disabled={isSaving || editingAlias === provider.alias || !editingAlias.trim()}
            >
              {isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : t("common.save")}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="h-8 w-8 p-0"
              onClick={() => {
                setEditingAlias(provider.alias)
                setIsEditingAlias(false)
              }}
              disabled={isSaving}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        ) : (
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{provider.alias}</span>
            <Button
              variant="ghost"
              size="sm"
              className="h-8 w-8 p-0"
              onClick={() => setIsEditingAlias(true)}
              disabled={isSaving}
            >
              <Pencil className="h-4 w-4" />
            </Button>
          </div>
        )}
      </div>

      {/* API URL (hidden for ollama_cloud — predefined) */}
      {provider.name !== "ollama_cloud" && (
        <div className="space-y-2">
          <Label htmlFor={`${namePrefix}-apiurl`}>API URL</Label>
          <Input
            id={`${namePrefix}-apiurl`}
            placeholder={
              provider.name === "ollama"
                ? "http://localhost:11434"
                : provider.name === "alibaba"
                  ? "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
                  : "https://api.openai.com"
            }
            value={provider.apiUrl}
            onChange={(e) => onFieldChange(provider.alias, "apiUrl", e.target.value)}
          />
        </div>
      )}

      {/* API Key (only for providers that require key auth) */}
      {(provider.name === "openai" || provider.name === "ollama_cloud" || provider.name === "alibaba") && (
        <div className="space-y-2">
          <Label htmlFor={`${namePrefix}-apikey`}>API Key</Label>
          <Input
            id={`${namePrefix}-apikey`}
            type="password"
            autoComplete="new-password"
            placeholder="sk-..."
            value={provider.apiKey}
            onChange={(e) => onFieldChange(provider.alias, "apiKey", e.target.value)}
          />
        </div>
      )}

      {/* Model */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label htmlFor={`${namePrefix}-model`}>{t("llm_ocr.model")}</Label>
          <Button
            variant="ghost"
            size="sm"
            onClick={onLoadModels}
            disabled={isModelsLoading}
            className="h-6 px-2 text-xs"
          >
            {isModelsLoading ? (
              <Loader2 className="mr-1 h-3 w-3 animate-spin" />
            ) : (
              <RefreshCw className="mr-1 h-3 w-3" />
            )}
            {t("llm_providers.loadModels")}
          </Button>
        </div>

        {/* Model dropdown or input */}
        {availableModels.length > 0 && !showModelInput ? (
          <div className="space-y-2">
            <Select
              value={provider.model}
              onValueChange={(value) => onFieldChange(provider.alias, "model", value)}
            >
              <SelectTrigger id={`${namePrefix}-model`}>
                <SelectValue placeholder={t("llm_providers.selectModel")} />
              </SelectTrigger>
              <SelectContent>
                {availableModels.map((model) => (
                  <SelectItem key={model.id} value={model.id}>
                    {model.name}
                    {model.size ? ` (${(model.size / 1073741824).toFixed(1)} GB)` : ""}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              variant="link"
              size="sm"
              className="px-0 h-auto text-xs"
              onClick={() => onToggleModelInput(true)}
            >
              {t("llm_providers.enterModelManually")}
            </Button>
          </div>
        ) : (
          <div className="space-y-2">
            <Input
              id={`${namePrefix}-model`}
              placeholder={
                provider.name === "ollama" || provider.name === "ollama_cloud"
                  ? "minicpm-v"
                  : provider.name === "alibaba"
                    ? "qwen-image-edit-plus"
                    : "gpt-4-vision-preview"
              }
              value={provider.model}
              onChange={(e) => onFieldChange(provider.alias, "model", e.target.value)}
            />
            {availableModels.length > 0 && showModelInput && (
              <Button
                variant="link"
                size="sm"
                className="px-0 h-auto text-xs"
                onClick={() => onToggleModelInput(false)}
              >
                {t("llm_providers.selectFromModels")}
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
