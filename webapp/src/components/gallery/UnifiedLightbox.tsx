import { useCallback, useEffect, useRef, useState } from "react"
import { Sparkles, Info, ScanText, Tags } from "lucide-react"
import type { LucideIcon } from "lucide-react"
import { useTranslation } from "@/i18n"
import { LightboxDialog } from "./lightbox/LightboxDialog"
import { buildImageUrl } from "@/utils/buildImageUrl"
import { OcrImagePanel } from "./lightbox/OcrImagePanel"
import { OcrResultPanel } from "./lightbox/OcrResultPanel"
import { TagsPanel } from "./lightbox/TagsPanel"
import { ChatPanel } from "./lightbox/ChatPanel"
import { ImageCompareSlider } from "./lightbox/ImageCompareSlider"
import { useOcrState } from "@/hooks/useOcrState"
import { useTagsState } from "@/hooks/useTagsState"
import { useImageDimensions } from "@/hooks/useImageDimensions"
import { useFileExport } from "@/hooks/useFileExport"
import { useChatAgent } from "@/hooks/useChatAgent"
import { useImageMetadata } from "@/hooks/useImageMetadata"
import { acceptEnhancement, rejectEnhancement } from "@/api/endpoints"
import { Skeleton } from "@/components/ui/skeleton"
import { Button } from "@/components/ui/button"
import { Camera, MapPin, MapPinPlus, Image as ImageIcon, Pencil, FileText } from "lucide-react"
import { GeoSearchForm } from "./GeoSearchForm"
import { UnderlineTabs } from "@/components/ui/underline-tabs"
import type { ImageMetadataDTO } from "@/types"

export type LightboxMode = "ai" | "exif" | "ocr" | "tags"

interface UnifiedLightboxProps {
  imagePath: string | null
  initialMode?: LightboxMode
  onClose: () => void
  showGeoForm?: boolean
  onShowGeoFormChange?: (show: boolean) => void
}

const TAB_CONFIG: { id: LightboxMode; labelKey: "lightbox.tab.ai" | "lightbox.tab.exif" | "lightbox.tab.ocr" | "lightbox.tab.tags"; icon: LucideIcon }[] = [
  { id: "ai", labelKey: "lightbox.tab.ai", icon: Sparkles },
  { id: "exif", labelKey: "lightbox.tab.exif", icon: Info },
  { id: "ocr", labelKey: "lightbox.tab.ocr", icon: ScanText },
  { id: "tags", labelKey: "lightbox.tab.tags", icon: Tags },
]

export function UnifiedLightbox({
  imagePath,
  initialMode = "ai",
  onClose,
  showGeoForm,
  onShowGeoFormChange,
}: UnifiedLightboxProps) {
  const { t, language } = useTranslation()
  const [activeMode, setActiveMode] = useState<LightboxMode>(initialMode)
  const [internalShowGeoForm, setInternalShowGeoForm] = useState(false)
  const [standardImageLoaded, setStandardImageLoaded] = useState(false)
  const isGeoFormVisible = showGeoForm ?? internalShowGeoForm

  const handleShowGeoForm = useCallback((show: boolean) => {
    if (onShowGeoFormChange) {
      onShowGeoFormChange(show)
    } else {
      setInternalShowGeoForm(show)
    }
  }, [onShowGeoFormChange])

  // Image version counter for cache-busting after enhancement
  const [standardImageVersion, setStandardImageVersion] = useState(0)
  const processedEnhanceRef = useRef<Set<string>>(new Set())
  const [enhancedPath, setEnhancedPath] = useState<string | null>(null)
  const [isAccepting, setIsAccepting] = useState(false)
  const [isRejecting, setIsRejecting] = useState(false)

  // Reset per-image state when imagePath changes (during render, not in effect)
  const [prevImagePath, setPrevImagePath] = useState(imagePath)
  const [prevInitialMode, setPrevInitialMode] = useState(initialMode)
  if (imagePath !== prevImagePath || initialMode !== prevInitialMode) {
    setPrevImagePath(imagePath)
    setPrevInitialMode(initialMode)
    setActiveMode(initialMode)
    setInternalShowGeoForm(false)
    setStandardImageLoaded(false)
    setEnhancedPath(null)
    processedEnhanceRef.current = new Set()
  }

  // OCR state
  const { ocrData, llmData, loading: ocrLoading, recognizing, resetState: resetOcr, handleRecognize } = useOcrState(
    activeMode === "ocr" ? imagePath : null
  )

  // Tags state
  const { tagsData, loading: tagsLoading, generating: tagsGenerating, error: tagsError, resetState: resetTags, handleGenerate: handleGenerateTags } = useTagsState(
    activeMode === "tags" ? imagePath : null
  )
  const isTextDocument = ocrData?.isTextDocument ?? false
  const ocrImageUrl = imagePath
    ? isTextDocument && ocrData?.angle !== undefined
      ? buildImageUrl(imagePath, "/api/ocr-image", { angle: ocrData.angle })
      : buildImageUrl(imagePath, "/api/image")
    : ""
  const { imageRef, displayDimensions, imageLoaded, handleImageLoad } = useImageDimensions(ocrImageUrl)
  const { handleSaveMd, handleSaveHtml } = useFileExport(llmData?.markdownContent, imagePath)

  // AI Chat state
  const {
    conversation,
    conversations,
    messages,
    isStreaming,
    error: chatError,
    tokenCount,
    maxTokens,
    isTokenLimitReached,
    currentImagePath,
    createNewConversation,
    loadConversation,
    loadConversations,
    removeConversation,
    sendMessage,
    abortStream,
    resetForImage,
    promptTokens,
    completionTokens,
    promptCacheHitTokens,
    promptCacheMissTokens,
    reasoningTokens,
  } = useChatAgent(language)

  // Reset conversation state and load/create when image changes
  useEffect(() => {
    if (!imagePath || activeMode !== "ai") return

    // Reset state for the image (clears conversation and messages)
    resetForImage(imagePath)

    // Load existing conversations for this image
    loadConversations(imagePath)
  }, [imagePath, activeMode, resetForImage, loadConversations])

  // Create new conversation if none exists for current image
  useEffect(() => {
    if (!imagePath || activeMode !== "ai") return
    if (currentImagePath !== imagePath) return // Wait for reset to complete
    if (conversation) return // Already have a conversation
    if (conversations.length > 0) return // User can select from history

    // No conversations for this image - create a new one
    createNewConversation(imagePath)
  }, [imagePath, activeMode, currentImagePath, conversation, conversations.length, createNewConversation])

  const handleNewConversation = useCallback(() => {
    createNewConversation(imagePath || undefined)
  }, [imagePath, createNewConversation])

  const handleDeleteConversation = useCallback(() => {
    if (conversation) {
      removeConversation(conversation.id)
    }
  }, [conversation, removeConversation])

  const handleLoadConversation = useCallback((id: number) => {
    loadConversation(id)
  }, [loadConversation])

  // EXIF metadata state (always loaded - needed for geo form date resolution)
  const { metadata, isLoading: metadataLoading, reload: reloadMetadata } = useImageMetadata(imagePath)

  const handleGpsSaved = useCallback(() => {
    reloadMetadata()
    handleShowGeoForm(false)
  }, [reloadMetadata, handleShowGeoForm])

  // Watch for completed enhance_image_quality tool calls
  useEffect(() => {
    for (const msg of messages) {
      if (msg.role !== "assistant" || !msg.toolCalls) continue
      for (let i = 0; i < msg.toolCalls.length; i++) {
        const tc = msg.toolCalls[i]
        if (tc.name === "enhance_image_quality" && tc.result) {
          const key = `${msg.id}-${i}`
          if (!processedEnhanceRef.current.has(key)) {
            processedEnhanceRef.current.add(key)
            setStandardImageVersion(v => v + 1)
            // Parse the JSON result to extract enhanced path
            try {
              const parsed = JSON.parse(tc.result)
              if (parsed.enhancedPath && parsed.status === "pending_approval") {
                setEnhancedPath(parsed.enhancedPath)
              }
            } catch {
              // Result is not JSON (legacy format or no_change status)
            }
          }
        }
      }
    }
  }, [messages])

  // Enhanced image URL for comparison
  const enhancedImageUrl = enhancedPath && imagePath
    ? buildImageUrl(imagePath, "/api/image-enhanced")
    : ""

  // URLs
  const cacheParam = standardImageVersion > 0 ? { _t: standardImageVersion } : undefined
  const standardImageUrl = imagePath ? buildImageUrl(imagePath, "/api/image", cacheParam) : ""

  const handleAcceptEnhancement = useCallback(async () => {
    if (!imagePath) return
    setIsAccepting(true)
    try {
      await acceptEnhancement({ imagePath })
      setEnhancedPath(null)
      setStandardImageVersion(v => v + 1)
    } catch {
      // Error is handled silently - image stays in comparison mode
    } finally {
      setIsAccepting(false)
    }
  }, [imagePath])

  const handleRejectEnhancement = useCallback(async () => {
    if (!imagePath) return
    setIsRejecting(true)
    try {
      await rejectEnhancement({ imagePath })
      setEnhancedPath(null)
    } catch {
      // Error is handled silently
    } finally {
      setIsRejecting(false)
    }
  }, [imagePath])

  const handleClose = useCallback(() => {
    abortStream()
    resetOcr()
    resetTags()
    resetForImage("")
    setInternalShowGeoForm(false)
    onClose()
  }, [abortStream, resetOcr, resetTags, resetForImage, onClose])

  const formatProcessingTime = (ms?: number) => {
    if (!ms) return ""
    if (ms < 1000) return t("llm_ocr.milliseconds", { ms })
    return t("llm_ocr.seconds", { seconds: (ms / 1000).toFixed(1) })
  }

  if (!imagePath) return null

  return (
    <LightboxDialog
      open={!!imagePath}
      onOpenChange={() => handleClose()}
      titleKey="lightbox.title"
      descriptionKey="lightbox.description"
    >
      <div className="flex h-full">
        {/* Left: Image panel */}
        {activeMode === "ocr" ? (
          <OcrImagePanel
            imageUrl={ocrImageUrl}
            ocrData={ocrData}
            isTextDocument={isTextDocument}
            loading={ocrLoading}
            imageRef={imageRef}
            displayDimensions={displayDimensions}
            imageLoaded={imageLoaded}
            handleImageLoad={handleImageLoad}
            className="flex-1 flex items-center justify-center p-8 relative h-full"
          />
        ) : enhancedPath ? (
          <div className="flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0 h-full relative p-4">
            <ImageCompareSlider
              originalUrl={standardImageUrl}
              enhancedUrl={enhancedImageUrl}
              onAccept={handleAcceptEnhancement}
              onReject={handleRejectEnhancement}
              isAccepting={isAccepting}
              isRejecting={isRejecting}
            />
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-black min-h-[300px] min-w-0 h-full relative">
            {!standardImageLoaded && (
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
              </div>
            )}
            <img
              src={standardImageUrl}
              alt={t("lightbox.alt")}
              className={`max-w-full max-h-full object-contain ${standardImageLoaded ? "" : "invisible"}`}
              onLoad={() => setStandardImageLoaded(true)}
            />
          </div>
        )}

        {/* Right: Panel with mode tabs */}
        <div className="w-full md:w-[400px] lg:w-[450px] md:min-w-[320px] border-t md:border-t-0 md:border-l bg-card h-full shrink-0 flex flex-col">
          <UnderlineTabs tabs={TAB_CONFIG} value={activeMode} onValueChange={setActiveMode} />

          {/* Panel content */}
          <div className="flex-1 min-h-0 overflow-hidden flex flex-col">
            {activeMode === "ai" && (
              <ChatPanelContent
                messages={messages}
                isStreaming={isStreaming}
                error={chatError}
                hasConversation={conversation !== null}
                imagePath={imagePath}
                tokenCount={tokenCount}
                maxTokens={maxTokens}
                isTokenLimitReached={isTokenLimitReached}
                conversations={conversations}
                activeConversationId={conversation?.id}
                onSendMessage={sendMessage}
                onAbortStream={abortStream}
                onNewConversation={handleNewConversation}
                onDeleteConversation={handleDeleteConversation}
                onLoadConversation={handleLoadConversation}
                promptTokens={promptTokens}
                completionTokens={completionTokens}
                promptCacheHitTokens={promptCacheHitTokens}
                promptCacheMissTokens={promptCacheMissTokens}
                reasoningTokens={reasoningTokens}
              />
            )}
            {activeMode === "exif" && (
              <ExifPanelContent
                metadata={metadata}
                isLoading={metadataLoading}
                imagePath={imagePath}
                showGeoForm={isGeoFormVisible}
                onShowGeoForm={() => handleShowGeoForm(true)}
                onGpsSaved={handleGpsSaved}
              />
            )}
            {activeMode === "ocr" && (
              <OcrResultPanel
                ocrData={ocrData}
                llmData={llmData}
                recognizing={recognizing}
                onRecognize={handleRecognize}
                onSaveMd={handleSaveMd}
                onSaveHtml={handleSaveHtml}
                formatProcessingTime={formatProcessingTime}
                className="w-full bg-card p-4 h-full flex flex-col"
              />
            )}
            {activeMode === "tags" && (
              <TagsPanel
                tagsData={tagsData}
                loading={tagsLoading}
                generating={tagsGenerating}
                error={tagsError}
                onGenerate={handleGenerateTags}
                formatProcessingTime={formatProcessingTime}
                className="w-full bg-card p-4 h-full flex flex-col"
              />
            )}
          </div>
        </div>
      </div>
    </LightboxDialog>
  )
}

// Wrapper to adapt ChatPanel to the unified layout (removes its own border-l and width)
function ChatPanelContent(props: {
  messages: import("@/types").ChatMessage[]
  isStreaming: boolean
  error: string | null
  hasConversation: boolean
  imagePath: string | null
  tokenCount: number
  maxTokens: number
  isTokenLimitReached: boolean
  conversations: import("@/types").Conversation[]
  activeConversationId?: number
  onSendMessage: (content: string) => void
  onAbortStream: () => void
  onNewConversation: () => void
  onDeleteConversation: () => void
  onLoadConversation: (id: number) => void
  promptTokens?: number
  completionTokens?: number
  promptCacheHitTokens?: number
  promptCacheMissTokens?: number
  reasoningTokens?: number
}) {
  return (
    <ChatPanel
      {...props}
      className="w-full h-full flex flex-col bg-card"
    />
  )
}

// EXIF panel content (adapted from ImageLightbox's MetadataContent)
function ExifPanelContent({
  metadata,
  isLoading,
  imagePath,
  showGeoForm,
  onShowGeoForm,
  onGpsSaved,
}: {
  metadata: ImageMetadataDTO | null
  isLoading: boolean
  imagePath: string
  showGeoForm: boolean
  onShowGeoForm: () => void
  onGpsSaved: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className="h-full overflow-y-auto p-4">
      <h3 className="text-sm font-semibold mb-3">{t("metadata.title")}</h3>
      {isLoading ? (
        <MetadataSkeleton />
      ) : metadata ? (
        <MetadataContent
          metadata={metadata}
          imagePath={imagePath}
          showGeoForm={showGeoForm}
          onShowGeoForm={onShowGeoForm}
          onGpsSaved={onGpsSaved}
        />
      ) : (
        <div className="space-y-4">
          <p className="text-xs text-muted-foreground">{t("metadata.noData")}</p>
          <div>
            <div className="flex items-center gap-1.5 mb-2">
              <span className="text-muted-foreground"><FileText className="h-3.5 w-3.5" /></span>
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("metadata.sectionFile")}</span>
            </div>
            <div className="flex justify-between items-baseline gap-2 text-xs">
              <span className="text-muted-foreground shrink-0">{t("metadata.file")}</span>
              <span className="font-medium text-right break-all" title={imagePath}>{imagePath}</span>
            </div>
          </div>
          <div>
            <div className="flex items-center gap-1.5 mb-2">
              <MapPin className="h-3.5 w-3.5 text-muted-foreground" />
              <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("metadata.sectionLocation")}</span>
            </div>
            {showGeoForm ? (
              <GeoSearchForm imagePath={imagePath} onGpsSaved={onGpsSaved} />
            ) : (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="w-full text-xs"
                onClick={onShowGeoForm}
              >
                <MapPinPlus className="h-3.5 w-3.5 mr-1.5" />
                {t("geo.addLocation")}
              </Button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function MetadataSkeleton() {
  return (
    <div className="space-y-3">
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex justify-between">
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-3 w-24" />
        </div>
      ))}
    </div>
  )
}

interface MetadataContentProps {
  metadata: ImageMetadataDTO
  imagePath: string
  showGeoForm: boolean
  onShowGeoForm: () => void
  onGpsSaved: () => void
}

function MetadataContent({ metadata, imagePath, showGeoForm, onShowGeoForm, onGpsSaved }: MetadataContentProps) {
  const { t } = useTranslation()

  const imageFields = buildFields([
    [t("metadata.dimensions"), metadata.dimensions],
  ])

  const cameraFields = buildFields([
    [t("metadata.camera"), metadata.cameraModel],
    [t("metadata.lens"), metadata.lensModel],
    [t("metadata.iso"), metadata.iso ? String(metadata.iso) : ""],
    [t("metadata.aperture"), metadata.aperture],
    [t("metadata.shutterSpeed"), metadata.shutterSpeed],
    [t("metadata.focalLength"), metadata.focalLength],
    [t("metadata.dateTaken"), metadata.dateTaken],
  ])

  const technicalFields = buildFields([
    [t("metadata.colorSpace"), metadata.colorSpace],
    [t("metadata.software"), metadata.software],
    [t("metadata.orientation"), metadata.orientation ? String(metadata.orientation) : ""],
  ])

  const coordsLabel =
    metadata.hasGps && metadata.gpsLatitude != null && metadata.gpsLongitude != null
      ? `${metadata.gpsLatitude.toFixed(4)}\u00b0, ${metadata.gpsLongitude.toFixed(4)}\u00b0`
      : ""
  const locationFields = buildFields([
    [t("metadata.nameLocal"), metadata.nameLocal],
    [t("metadata.nameEng"), metadata.nameEng],
    [t("metadata.coordinates"), coordsLabel],
  ])

  const hasAnything =
    imageFields.length > 0 ||
    cameraFields.length > 0 ||
    technicalFields.length > 0 ||
    locationFields.length > 0 ||
    !metadata.hasGps

  if (!hasAnything) {
    return <p className="text-xs text-muted-foreground">{t("metadata.noData")}</p>
  }

  return (
    <div className="space-y-4">
      {/* File section */}
      <div>
        <div className="flex items-center gap-1.5 mb-2">
          <span className="text-muted-foreground"><FileText className="h-3.5 w-3.5" /></span>
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("metadata.sectionFile")}</span>
        </div>
        <div className="space-y-1.5">
          <div className="flex justify-between items-baseline gap-2 text-xs">
            <span className="text-muted-foreground shrink-0">{t("metadata.file")}</span>
            <span className="font-medium text-right break-all" title={imagePath}>
              {imagePath}
            </span>
          </div>
        </div>
      </div>

      {imageFields.length > 0 && (
        <MetadataSection icon={<ImageIcon className="h-3.5 w-3.5" />} title={t("metadata.sectionImage")} fields={imageFields} />
      )}
      {cameraFields.length > 0 && (
        <MetadataSection icon={<Camera className="h-3.5 w-3.5" />} title={t("metadata.sectionCamera")} fields={cameraFields} />
      )}
      {technicalFields.length > 0 && (
        <MetadataSection icon={<Info className="h-3.5 w-3.5" />} title={t("metadata.sectionTechnical")} fields={technicalFields} />
      )}

      {/* Location section */}
      <div>
        <div className="flex items-center gap-1.5 mb-2">
          <MapPin className="h-3.5 w-3.5 text-muted-foreground" />
          <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{t("metadata.sectionLocation")}</span>
        </div>

        {showGeoForm ? (
          <GeoSearchForm imagePath={imagePath} date={metadata.dateTaken.slice(0, 10)} onGpsSaved={onGpsSaved} />
        ) : metadata.hasGps ? (
          <>
            <div className="space-y-1.5">
              {locationFields.map(([label, value]) => (
                <div key={label} className="flex justify-between items-baseline gap-2 text-xs">
                  <span className="text-muted-foreground shrink-0">{label}</span>
                  <span className="font-medium text-right truncate" title={value}>
                    {value}
                  </span>
                </div>
              ))}
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="w-full text-xs mt-2"
              onClick={onShowGeoForm}
            >
              <Pencil className="h-3.5 w-3.5 mr-1.5" />
              {t("geo.editLocation")}
            </Button>
          </>
        ) : (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-full text-xs"
            onClick={onShowGeoForm}
          >
            <MapPinPlus className="h-3.5 w-3.5 mr-1.5" />
            {t("geo.addLocation")}
          </Button>
        )}
      </div>
    </div>
  )
}

function MetadataSection({
  icon,
  title,
  fields,
}: {
  icon: React.ReactNode
  title: string
  fields: [string, string][]
}) {
  return (
    <div>
      <div className="flex items-center gap-1.5 mb-2">
        <span className="text-muted-foreground">{icon}</span>
        <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">{title}</span>
      </div>
      <div className="space-y-1.5">
        {fields.map(([label, value]) => (
          <div key={label} className="flex justify-between items-baseline gap-2 text-xs">
            <span className="text-muted-foreground shrink-0">{label}</span>
            <span className="font-medium text-right truncate" title={value}>
              {value}
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

function buildFields(entries: [string, string][]): [string, string][] {
  return entries.filter(([, value]) => value !== "" && value !== "0")
}
