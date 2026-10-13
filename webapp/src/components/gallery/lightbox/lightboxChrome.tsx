import type { ReactNode } from "react"
import type { LucideIcon } from "lucide-react"
import { Sparkles, Info, ScanText, Tags } from "lucide-react"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Button } from "@/components/ui/button"
import type { ButtonProps } from "@/components/ui/button"
import { InfoPopover } from "@/components/ui/info-popover"
import { useTranslation } from "@/i18n"
import type { TranslationKey } from "@/i18n"
import { cn } from "@/lib/utils"

/** Lightbox modes shared by every lightbox surface. */
export type LightboxMode = "ai" | "exif" | "ocr" | "tags"

/** Mode tab definitions used by the lightbox tab bar. */
const LIGHTBOX_TAB_CONFIG: { id: LightboxMode; labelKey: TranslationKey; icon: LucideIcon }[] = [
  { id: "ai", labelKey: "lightbox.tab.ai", icon: Sparkles },
  { id: "exif", labelKey: "lightbox.tab.exif", icon: Info },
  { id: "ocr", labelKey: "lightbox.tab.ocr", icon: ScanText },
  { id: "tags", labelKey: "lightbox.tab.tags", icon: Tags },
]

/**
 * Lightbox mode tab bar. Below `md` the list wraps (`flex-wrap`) so the tabs
 * stay within the lightbox width instead of causing horizontal overflow.
 */
export function LightboxTabBar({
  value,
  onValueChange,
}: {
  value: LightboxMode
  onValueChange: (mode: LightboxMode) => void
}) {
  const { t } = useTranslation()
  return (
    <Tabs variant="underline" value={value} onValueChange={(v) => onValueChange(v as LightboxMode)}>
      <TabsList className="max-md:flex-wrap max-md:gap-y-1" data-testid="lightbox-tab-bar">
        {LIGHTBOX_TAB_CONFIG.map(({ id, labelKey, icon: Icon }) => (
          <TabsTrigger key={id} value={id}>
            <Icon className="h-3.5 w-3.5" />
            {t(labelKey)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  )
}

export interface LightboxActionProps extends Omit<ButtonProps, "children" | "aria-label"> {
  /** Leading icon, always rendered. */
  icon: ReactNode
  /** Visible label; hidden below `md` so the control is icon-first on mobile. */
  label: string
  /** Stable accessible name; defaults to the visible `label`. */
  ariaLabel?: string
}

/**
 * Lightbox action control that is icon-first below `md`: the text label is
 * hidden while the icon and accessible name remain, and the control presents a
 * 44×44 touch target on mobile (reset to the compact height at `md`).
 */
export function LightboxAction({
  icon,
  label,
  ariaLabel,
  className,
  variant = "outline",
  size = "sm",
  ...props
}: LightboxActionProps) {
  const accessibleName = ariaLabel ?? label
  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      aria-label={accessibleName}
      title={accessibleName}
      className={cn("h-11 min-w-11 gap-1.5 md:h-8", className)}
      {...props}
    >
      {icon}
      <span className="hidden md:inline">{label}</span>
    </Button>
  )
}

/**
 * A long informational lightbox readout. At `md` and above the full text is
 * shown inline; below `md` it collapses to the shared `InfoPopover` icon-only
 * control whose popup reveals the full text.
 */
export function LightboxInfoReadout({ text, className }: { text: string; className?: string }) {
  return (
    <div
      className={cn("flex items-center justify-end gap-1.5", className)}
      data-testid="lightbox-info-readout"
    >
      <span className="hidden md:inline">{text}</span>
      <div className="md:hidden">
        {/* The readout sits at the panel's left edge (the chat header), so the
            popup is left-anchored and width-clamped to open inside the lightbox
            instead of growing past its left edge and being clipped. */}
        <InfoPopover
          text={text}
          popoverClassName="left-0 right-auto max-w-[min(16rem,calc(100vw_-_7rem))]"
        />
      </div>
    </div>
  )
}
