import { useCallback, useEffect, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Badge } from "@/components/ui/badge"
import {
  fetchLlmSettings,
  createLlmProvider,
  updateLlmProvider,
  deleteLlmProvider,
  fetchLlmModels,
} from "@/api/endpoints"
import {
  Loader2,
  Plus,
  RefreshCw,
  Trash2,
  Pencil,
  Server,
  Eye,
  MessageSquare,
  Brain,
  Wand2,
} from "lucide-react"
import { useTranslation } from "@/i18n"
import type { LlmSettingsResponse, LlmProviderDTO, LlmModelDTO, LlmProviderType, LlmInstrumentType } from "@/types"

// Provider type display labels
const PROVIDER_LABELS: Record<LlmProviderType, string> = {
  ollama: "Ollama",
  ollama_cloud: "Ollama Cloud",
  openai: "OpenAI API compatible",
  deepseek: "DeepSeek",
  alibaba: "Alibaba Cloud",
}

const ALLOWED_PROVIDER_TYPES: LlmProviderType[] = ["ollama", "ollama_cloud", "openai", "deepseek", "alibaba"]

// Capability labels
type ModelCapability = "chat" | "tool_calling" | "vision" | "embedding"

const CAPABILITY_LABELS: Record<ModelCapability, string> = {
  chat: "Chat",
  tool_calling: "Tools",
  vision: "Vision",
  embedding: "Embedding",
}

// Instrument labels for usage badges
const INSTRUMENT_ICONS: Record<LlmInstrumentType, { icon: typeof MessageSquare; label: string }> = {
  chat: { icon: MessageSquare, label: "llm_providers.usageShortChat" },
  vl: { icon: Eye, label: "llm_providers.usageShortVL" },
  embedding: { icon: Brain, label: "llm_providers.usageShortEmb" },
  image_edit: { icon: Wand2, label: "llm_providers.usageShortImgEdit" },
}

// Infer capabilities based on provider type and model name heuristics.
// When apiCapabilities are provided (from Ollama /api/show), they take precedence.
function inferCapabilities(
  providerType: LlmProviderType,
  modelId: string,
  apiCapabilities?: string[],
): ModelCapability[] {
  // If the backend provided actual capabilities, use them directly
  if (apiCapabilities && apiCapabilities.length > 0) {
    const caps: ModelCapability[] = []
    for (const c of apiCapabilities) {
      if (c === "chat" || c === "tool_calling" || c === "vision" || c === "embedding") {
        caps.push(c as ModelCapability)
      }
    }
    if (caps.length > 0) return caps
  }

  // Fallback heuristics for providers that don't report capabilities
  const caps: ModelCapability[] = []

  // All providers support chat completion
  caps.push("chat")

  // Tool calling support
  if (providerType === "deepseek" || providerType === "openai") {
    caps.push("tool_calling")
  } else if (providerType === "ollama" || providerType === "ollama_cloud") {
    caps.push("tool_calling")
  } else if (providerType === "alibaba") {
    caps.push("tool_calling")
  }

  // Vision support heuristics
  const modelLower = modelId.toLowerCase()
  const visionModels = [
    "minicpm", "llava", "bakllava", "cogvlm", "vision", "gpt-4o", "gpt-4-vision",
    "gemini", "claude", "pixtral", "llama3.2-vision", "qwen2-vl", "qwen-vl",
    "qwen-vl-plus", "qwen-vl-max", "qwen-image-edit",
    "deepseek-vl", "internvl", "glm-4v",
  ]
  if (visionModels.some((vm) => modelLower.includes(vm))) {
    caps.push("vision")
  } else if (providerType === "deepseek") {
    if (modelLower.includes("v4-pro")) {
      caps.push("vision")
    }
  } else if (providerType === "openai") {
    if (modelLower.includes("gpt-4") || modelLower.includes("gpt-4o") || modelLower.includes("vision")) {
      caps.push("vision")
    }
  } else if (providerType === "alibaba") {
    if (modelLower.includes("qwen-vl") || modelLower.includes("qwen-image-edit")) {
      caps.push("vision")
    }
  }

  return caps
}

function formatContextLength(len: number | undefined): string {
  if (!len) return "—"
  if (len >= 1_000_000) return `${(len / 1_000_000).toFixed(1)}M`
  if (len >= 1_000) return `${(len / 1_000).toFixed(0)}K`
  return String(len)
}

export function AdminLlmProvidersTab() {
  const { t } = useTranslation()

  const [llmSettings, setLlmSettings] = useState<LlmSettingsResponse | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [isSaving, setIsSaving] = useState(false)

  // Model cache
  const [modelCache, setModelCache] = useState<Record<string, LlmModelDTO[]>>({})
  const [expandedProviderAlias, setExpandedProviderAlias] = useState<string | null>(null)
  const [loadingModelsAlias, setLoadingModelsAlias] = useState<string | null>(null)

  // New provider form — model is now set per instrument, not on the provider
  const [showNewProvider, setShowNewProvider] = useState(false)
  const [newProviderType, setNewProviderType] = useState<LlmProviderType>("ollama")
  const [newProviderAlias, setNewProviderAlias] = useState("")
  const [newProviderApiUrl, setNewProviderApiUrl] = useState("")
  const [newProviderApiKey, setNewProviderApiKey] = useState("")

  // Editing state — model is now set per instrument, not on the provider
  const [editingProviderAlias, setEditingProviderAlias] = useState<string | null>(null)
  const [editingAliasValue, setEditingAliasValue] = useState("")
  const [editingApiUrl, setEditingApiUrl] = useState("")
  const [editingApiKey, setEditingApiKey] = useState("")

  const loadSettings = useCallback(async () => {
    setIsLoading(true)
    try {
      const settings = await fetchLlmSettings()
      setLlmSettings(settings)
      // Seed model cache from cachedModels in response
      setModelCache((prev) => {
        const next = { ...prev }
        for (const p of settings.providers) {
          if (p.cachedModels && p.cachedModels.length > 0) {
            next[p.alias] = p.cachedModels
          }
        }
        return next
      })
    } catch {
      setLlmSettings(null)
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    ;(async () => {
      try {
        const settings = await fetchLlmSettings()
        setLlmSettings(settings)
        setModelCache((prev) => {
          const next = { ...prev }
          for (const p of settings.providers) {
            if (p.cachedModels && p.cachedModels.length > 0) {
              next[p.alias] = p.cachedModels
            }
          }
          return next
        })
      } catch {
        setLlmSettings(null)
      }
    })()
  }, [])

  // Load models for a provider
  const loadModelsForProvider = useCallback(
    async (alias: string, forceRefresh = false) => {
      if (!alias) return
      if (!forceRefresh && modelCache[alias]) {
        return modelCache[alias]
      }
      setLoadingModelsAlias(alias)
      try {
        const response = await fetchLlmModels(alias, forceRefresh)
        if (response.success) {
          if (response.models.length > 0) {
            setModelCache((prev) => ({ ...prev, [alias]: response.models }))
            return response.models
          }
          toast.info(t("llm_providers.modelsLoaded", { count: 0 }))
          return null
        }
        toast.error(response.error || t("llm_providers.modelsLoadFailed"))
        return null
      } catch {
        toast.error(t("llm_providers.modelsLoadFailed"))
        return null
      } finally {
        setLoadingModelsAlias(null)
      }
    },
    [modelCache, t],
  )

  const handleToggleModels = useCallback(
    async (alias: string) => {
      if (expandedProviderAlias === alias) {
        setExpandedProviderAlias(null)
        return
      }
      setExpandedProviderAlias(alias)
      await loadModelsForProvider(alias)
    },
    [expandedProviderAlias, loadModelsForProvider],
  )

  // Add provider — model is no longer set here (moved to instrument settings)
  const handleAddProvider = useCallback(async () => {
    if (!newProviderAlias.trim()) {
      toast.error("Alias is required")
      return
    }
    const providers = llmSettings?.providers ?? []
    if (providers.some((p) => p.alias === newProviderAlias.trim())) {
      toast.error(t("llm_providers.aliasMustBeUnique"))
      return
    }

    const defaultApiUrl =
      newProviderType === "ollama"
        ? "http://localhost:11434"
        : newProviderType === "ollama_cloud"
          ? "https://ollama.com"
          : newProviderType === "deepseek"
            ? "https://api.deepseek.com"
            : newProviderType === "alibaba"
              ? "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
              : "https://api.openai.com"

    const apiUrl =
      newProviderType === "ollama_cloud" || newProviderType === "deepseek"
        ? defaultApiUrl
        : newProviderApiUrl.trim() || defaultApiUrl

    setIsSaving(true)
    try {
      await createLlmProvider({
        alias: newProviderAlias.trim(),
        name: newProviderType,
        apiUrl,
        apiKey:
          newProviderType === "ollama_cloud" ||
          newProviderType === "openai" ||
          newProviderType === "deepseek" ||
          newProviderType === "alibaba"
            ? newProviderApiKey
            : undefined,
      })
      toast.success(t("llm_ocr.settingsSaved"))
      setShowNewProvider(false)
      setNewProviderAlias("")
      setNewProviderApiUrl("")
      setNewProviderApiKey("")
      await loadSettings()
    } catch {
      toast.error(t("llm_ocr.settingsSaveFailed"))
    } finally {
      setIsSaving(false)
    }
  }, [
    newProviderAlias,
    newProviderType,
    newProviderApiUrl,
    newProviderApiKey,
    llmSettings?.providers,
    loadSettings,
    t,
  ])

  // Start editing a provider — no model to load
  const startEditing = useCallback(
    (provider: LlmProviderDTO) => {
      setEditingProviderAlias(provider.alias)
      setEditingAliasValue(provider.alias)
      setEditingApiUrl(provider.apiUrl)
      setEditingApiKey(provider.apiKey)
    },
    [],
  )

  const cancelEditing = useCallback(() => {
    setEditingProviderAlias(null)
  }, [])

  // Save provider edits — no model field
  const handleSaveEdit = useCallback(
    async (oldAlias: string) => {
      if (!editingAliasValue.trim()) return
      const providers = llmSettings?.providers ?? []
      if (editingAliasValue !== oldAlias && providers.some((p) => p.alias === editingAliasValue)) {
        toast.error(t("llm_providers.aliasMustBeUnique"))
        return
      }

      setIsSaving(true)
      try {
        const update: Record<string, string> = {}
        const provider = providers.find((p) => p.alias === oldAlias)
        if (!provider) return

        if (editingAliasValue !== oldAlias) {
          update.alias = editingAliasValue
        }
        if (editingApiUrl !== provider.apiUrl) {
          update.apiUrl = editingApiUrl
        }
        if (editingApiKey !== provider.apiKey) {
          update.apiKey = editingApiKey
        }

        if (Object.keys(update).length > 0) {
          await updateLlmProvider(oldAlias, update)
        }

        // Handle alias rename in cache
        if (update.alias && modelCache[oldAlias]) {
          setModelCache((prev) => {
            const next = { ...prev }
            next[update.alias!] = prev[oldAlias]
            delete next[oldAlias]
            return next
          })
        }

        toast.success(t("llm_ocr.settingsSaved"))
        setEditingProviderAlias(null)
        await loadSettings()
      } catch {
        toast.error(t("llm_ocr.settingsSaveFailed"))
      } finally {
        setIsSaving(false)
      }
    },
    [editingAliasValue, editingApiUrl, editingApiKey, llmSettings?.providers, loadSettings, t, modelCache],
  )

  // Delete provider
  const handleDeleteProvider = useCallback(
    async (alias: string) => {
      if (!confirm(t("llm_providers.deleteConfirm", { alias }))) return

      // Check if provider is in use by any instrument
      if (llmSettings) {
        const usages: string[] = []
        for (const instr of llmSettings.instruments) {
          if (instr.providerAlias === alias) {
            const iconInfo = INSTRUMENT_ICONS[instr.type]
            if (iconInfo) {
              usages.push(t(iconInfo.label as never))
            }
          }
        }
        if (usages.length > 0) {
          toast.error(t("llm_providers.cannotDeleteInUse", { usages: usages.join(", ") }))
          return
        }
      }

      setIsSaving(true)
      try {
        await deleteLlmProvider(alias)
        toast.success(t("llm_ocr.settingsSaved"))
        setModelCache((prev) => {
          const next = { ...prev }
          delete next[alias]
          return next
        })
        if (expandedProviderAlias === alias) setExpandedProviderAlias(null)
        await loadSettings()
      } catch {
        toast.error(t("llm_ocr.settingsSaveFailed"))
      } finally {
        setIsSaving(false)
      }
    },
    [llmSettings, expandedProviderAlias, loadSettings, t],
  )

  const getProviderLabel = (name: LlmProviderType): string => PROVIDER_LABELS[name] ?? name

  // Check if provider is in use by any instrument (new model via instruments[])
  const isProviderInUse = (alias: string): string[] => {
    if (!llmSettings) return []
    const usages: string[] = []
    for (const instr of llmSettings.instruments) {
      if (instr.providerAlias === alias) {
        const iconInfo = INSTRUMENT_ICONS[instr.type]
        if (iconInfo) {
          usages.push(t(iconInfo.label as never))
        }
      }
    }
    return usages
  }

  const providers = llmSettings?.providers ?? []

  return (
    <div className="space-y-6">
      {isLoading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <>
          {/* Provider List */}
          {providers.length === 0 ? (
            <Card>
              <CardContent className="py-10 text-center">
                <Server className="mx-auto h-10 w-10 text-muted-foreground mb-3" />
                <p className="text-muted-foreground">{t("llm_providers.noProviders")}</p>
              </CardContent>
            </Card>
          ) : (
            <div className="space-y-4">
              {providers.map((provider) => {
                const usages = isProviderInUse(provider.alias)
                const isExpanded = expandedProviderAlias === provider.alias
                const models = modelCache[provider.alias] ?? []
                const isLoadingModels = loadingModelsAlias === provider.alias
                const isEditingThis = editingProviderAlias === provider.alias

                return (
                  <Card key={provider.alias}>
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <CardTitle className="text-base">{provider.alias}</CardTitle>
                          <Badge variant="secondary" className="text-xs">
                            {getProviderLabel(provider.name)}
                          </Badge>
                          {usages.length > 0 && (
                            <div className="flex gap-1">
                              {llmSettings?.instruments
                                .filter((instr) => instr.providerAlias === provider.alias)
                                .map((instr) => {
                                  const iconInfo = INSTRUMENT_ICONS[instr.type]
                                  if (!iconInfo) return null
                                  const Icon = iconInfo.icon
                                  return (
                                    <Badge key={instr.type} variant="outline" className="text-xs gap-1">
                                      <Icon className="h-3 w-3" />
                                      {t(iconInfo.label as never)}
                                    </Badge>
                                  )
                                })}
                            </div>
                          )}
                        </div>
                        <div className="flex gap-1">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => startEditing(provider)}
                            disabled={isSaving || editingProviderAlias !== null}
                            className="h-8 w-8 p-0"
                            title={t("common.edit")}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleDeleteProvider(provider.alias)}
                            disabled={isSaving || usages.length > 0}
                            className="h-8 w-8 p-0 text-destructive"
                            title={usages.length > 0 ? t("llm_providers.cannotDeleteInUseTooltip") : t("common.delete")}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </div>
                      <CardDescription className="flex flex-col gap-1">
                        <span className="text-xs">
                          API URL: {provider.name === "ollama_cloud"
                            ? "https://ollama.com"
                            : provider.apiUrl}
                        </span>
                        {llmSettings?.instruments
                          .filter((instr) => instr.providerAlias === provider.alias)
                          .map((instr) => (
                            <span key={instr.type} className="text-xs text-muted-foreground">
                              {t("llm_ocr.model")} ({instr.type}):{" "}
                              <span className="font-mono">{instr.model}</span>
                            </span>
                          ))}
                      </CardDescription>
                    </CardHeader>

                    {/* Edit form inline — no model field (moved to instrument settings) */}
                    {isEditingThis && (
                      <CardContent className="border-t pt-4 space-y-3">
                        <div className="space-y-2">
                          <Label>{t("llm_providers.alias")}</Label>
                          <Input
                            value={editingAliasValue}
                            onChange={(e) => setEditingAliasValue(e.target.value)}
                            disabled={isSaving}
                            placeholder={t("llm_providers.aliasPlaceholder")}
                          />
                        </div>

                        {provider.name !== "ollama_cloud" && (
                          <div className="space-y-2">
                            <Label>API URL</Label>
                            <Input
                              value={editingApiUrl}
                              onChange={(e) => setEditingApiUrl(e.target.value)}
                              disabled={isSaving}
                              placeholder={
                                provider.name === "ollama"
                                  ? "http://localhost:11434"
                                  : provider.name === "deepseek"
                                    ? "https://api.deepseek.com"
                                    : provider.name === "alibaba"
                                      ? "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
                                      : "https://api.openai.com"
                              }
                            />
                          </div>
                        )}

                        {(provider.name === "openai" || provider.name === "ollama_cloud" || provider.name === "deepseek" || provider.name === "alibaba") && (
                          <div className="space-y-2">
                            <Label>API Key</Label>
                            <Input
                              type="password"
                              autoComplete="new-password"
                              value={editingApiKey}
                              onChange={(e) => setEditingApiKey(e.target.value)}
                              disabled={isSaving}
                              placeholder="sk-..."
                            />
                          </div>
                        )}

                        <p className="text-xs text-muted-foreground">
                          {t("llm_providers.manageInProvidersTab")}
                        </p>

                        <div className="flex gap-2 justify-end pt-1">
                          <Button variant="outline" size="sm" onClick={cancelEditing} disabled={isSaving}>
                            {t("common.cancel")}
                          </Button>
                          <Button
                            size="sm"
                            onClick={() => handleSaveEdit(provider.alias)}
                            disabled={isSaving || !editingAliasValue.trim()}
                          >
                            {isSaving ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : null}
                            {t("common.save")}
                          </Button>
                        </div>
                      </CardContent>
                    )}

                    {/* Models section — still shows available models for the provider */}
                    <CardContent className="pt-0">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="w-full justify-between text-xs"
                        onClick={() => handleToggleModels(provider.alias)}
                      >
                        <span>
                          {t("llm_providers.modelsCount", {
                            count: models.length || (provider.cachedModels?.length ?? 0),
                          })}
                        </span>
                        <div className="flex items-center gap-1">
                          {isLoadingModels && <Loader2 className="h-3 w-3 animate-spin" />}
                          <RefreshCw
                            className="h-3 w-3"
                            onClick={(e) => {
                              e.stopPropagation()
                              loadModelsForProvider(provider.alias, true).then(() => {
                                if (!isExpanded) setExpandedProviderAlias(provider.alias)
                              })
                            }}
                          />
                        </div>
                      </Button>

                      {isExpanded && (
                        <div className="mt-2 space-y-0.5 max-h-64 overflow-y-auto border rounded-md">
                          {models.length === 0 && !isLoadingModels ? (
                            <p className="text-xs text-muted-foreground text-center py-4">
                              {t("llm_providers.noModelsLoaded")}
                            </p>
                          ) : (
                            models.map((model) => {
                              const caps = inferCapabilities(provider.name, model.id, model.capabilities)
                              return (
                                <div
                                  key={model.id}
                                  className="flex items-center justify-between px-3 py-1.5 text-xs hover:bg-muted/50"
                                >
                                  <span className="font-medium truncate">{model.name}</span>
                                  <div className="flex items-center gap-2 flex-shrink-0 ml-2">
                                    <div className="flex gap-0.5">
                                      {caps.map((cap) => (
                                        <Badge key={cap} variant="outline" className="text-[10px] px-1 py-0 h-4">
                                          {CAPABILITY_LABELS[cap]}
                                        </Badge>
                                      ))}
                                    </div>
                                    {model.contextLength ? (
                                      <Badge variant="secondary" className="text-[10px] px-1 py-0 h-4 font-mono">
                                        {formatContextLength(model.contextLength)}
                                      </Badge>
                                    ) : null}
                                  </div>
                                </div>
                              )
                            })
                          )}
                        </div>
                      )}
                    </CardContent>
                  </Card>
                )
              })}
            </div>
          )}

          {/* Add New Provider — no model field */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">{t("llm_providers.addProvider")}</CardTitle>
            </CardHeader>
            <CardContent>
              {showNewProvider ? (
                <div className="space-y-3 rounded-lg border p-4">
                  <h4 className="text-sm font-medium">{t("llm_providers.newProvider")}</h4>

                  {/* Provider Type */}
                  <div className="space-y-2">
                    <Label>{t("llm_providers.type")}</Label>
                    <Select
                      value={newProviderType}
                      onValueChange={(v) => setNewProviderType(v as LlmProviderType)}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {ALLOWED_PROVIDER_TYPES.map((type) => (
                          <SelectItem key={type} value={type}>
                            {getProviderLabel(type)}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  {/* Alias */}
                  <div className="space-y-2">
                    <Label htmlFor="new-alias">{t("llm_providers.alias")}</Label>
                    <Input
                      id="new-alias"
                      placeholder={t("llm_providers.aliasPlaceholder")}
                      value={newProviderAlias}
                      onChange={(e) => setNewProviderAlias(e.target.value)}
                    />
                  </div>

                  {/* API URL */}
                  {newProviderType !== "ollama_cloud" && (
                    <div className="space-y-2">
                      <Label htmlFor="new-apiurl">API URL</Label>
                      <Input
                        id="new-apiurl"
                        placeholder={
                          newProviderType === "ollama"
                            ? "http://localhost:11434"
                            : newProviderType === "deepseek"
                              ? "https://api.deepseek.com"
                              : newProviderType === "alibaba"
                                ? "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
                                : "https://api.openai.com"
                        }
                        value={newProviderApiUrl}
                        onChange={(e) => setNewProviderApiUrl(e.target.value)}
                      />
                    </div>
                  )}

                  {/* API Key */}
                  {(newProviderType === "ollama_cloud" ||
                    newProviderType === "openai" ||
                    newProviderType === "deepseek" ||
                    newProviderType === "alibaba") && (
                    <div className="space-y-2">
                      <Label htmlFor="new-apikey">API Key</Label>
                      <Input
                        id="new-apikey"
                        type="password"
                        autoComplete="new-password"
                        placeholder="sk-..."
                        value={newProviderApiKey}
                        onChange={(e) => setNewProviderApiKey(e.target.value)}
                      />
                    </div>
                  )}

                  <p className="text-xs text-muted-foreground">
                    {t("llm_providers.manageInProvidersTab")}
                  </p>

                  <div className="flex gap-2 justify-end">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        setShowNewProvider(false)
                        setNewProviderAlias("")
                        setNewProviderApiUrl("")
                        setNewProviderApiKey("")
                      }}
                    >
                      {t("common.cancel")}
                    </Button>
                    <Button
                      size="sm"
                      onClick={handleAddProvider}
                      disabled={isSaving || !newProviderAlias.trim()}
                    >
                      {isSaving ? <Loader2 className="mr-1 h-4 w-4 animate-spin" /> : <Plus className="mr-1 h-4 w-4" />}
                      {t("llm_providers.add")}
                    </Button>
                  </div>
                </div>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowNewProvider(true)}
                  className="w-full"
                  disabled={editingProviderAlias !== null}
                >
                  <Plus className="mr-1.5 h-4 w-4" />
                  {t("llm_providers.addProvider")}
                </Button>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
