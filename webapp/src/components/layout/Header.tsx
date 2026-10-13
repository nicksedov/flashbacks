import { useTranslation } from "@/i18n"
import { useAuth } from "@/providers/useAuth"
import { useGallerySelection } from "@/providers/useGallerySelection"
import { getAvatarUrl } from "@/api/endpoints"
import { LogOut, Settings, User, Menu, X, Move, Trash2 } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { IconButton } from "@/components/ui/icon-button"
import { Button } from "@/components/ui/button"

interface HeaderProps {
  onTabChange: (tab: string) => void
  onMobileMenuToggle: () => void
}

export function Header({ onTabChange, onMobileMenuToggle }: HeaderProps) {
  const { t } = useTranslation()
  const { user, logout } = useAuth()
  const selection = useGallerySelection()

  return (
    <header
      className="flex-shrink-0 z-10 px-4 sm:px-6 py-3"
      style={{ backgroundColor: 'var(--color-header)' }}
    >
      <div className="flex items-center justify-between gap-3">
        {/* Selection bar: shown instead of mobile menu button when selection is active */}
        {selection.isActive ? (
          <div className="flex items-center gap-2">
            <Badge variant="secondary" className="text-xs">
              {selection.selectedCount === 1
                ? t("gallery.selection.selectedCountOne", { count: selection.selectedCount })
                : t("gallery.selection.selectedCount", { count: selection.selectedCount })}
            </Badge>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 px-2 text-xs"
              onClick={selection.clearSelection}
              aria-label={t("gallery.selection.clearSelection")}
              title={t("gallery.selection.clearSelection")}
            >
              <X className="h-3.5 w-3.5 md:mr-1" />
              <span className="hidden md:inline">{t("gallery.selection.clearSelection")}</span>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 px-2 text-xs"
              onClick={selection.moveSelected}
              aria-label={t("header.moveSelected")}
              title={t("header.moveSelected")}
            >
              <Move className="h-3.5 w-3.5 md:mr-1" />
              <span className="hidden md:inline">{t("header.moveSelected")}</span>
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 px-2 text-xs border-border"
              onClick={selection.deleteSelected}
              aria-label={t("common.delete")}
              title={t("common.delete")}
            >
              <Trash2 className="h-3.5 w-3.5 md:mr-1" />
              <span className="hidden md:inline">{t("common.delete")}</span>
            </Button>
          </div>
        ) : (
          <>
            {/* Mobile menu button */}
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="md:hidden"
              onClick={onMobileMenuToggle}
              aria-label={t("header.menu")}
              title={t("header.menu")}
            >
              <Menu className="h-5 w-5" />
            </Button>

            {/* Spacer for desktop alignment */}
            <div className="hidden md:block" />
          </>
        )}

        {user && (
          <div className="flex min-w-0 items-center gap-3">
            <Button
              type="button"
              variant="ghost"
              className="flex min-w-0 items-center gap-2 px-2 py-1"
              onClick={() => onTabChange("profile")}
              aria-label={user.displayName}
              title={user.displayName}
            >
              {user.hasAvatar ? (
                <img src={getAvatarUrl(user.id)} className="h-6 w-6 rounded-full object-cover" alt="" />
              ) : (
                <User className="h-4 w-4 text-muted-foreground" />
              )}
              <span className="hidden items-center gap-2 md:flex">
                <span className="text-sm font-medium">{user.displayName}</span>
                <Badge variant="outline" className="text-xs">
                  {user.role === "admin" ? t("adminPanel.roleAdmin") : t("adminPanel.roleUser")}
                </Badge>
              </span>
            </Button>
            <IconButton
              variant="ghost"
              size="sm"
              icon={Settings}
              onClick={() => onTabChange("settings")}
              title={t("tabs.preferences")}
            />
            <IconButton
              variant="ghost"
              size="sm"
              icon={LogOut}
              onClick={logout}
              aria-label={t("adminPanel.logout")}
              title={t("adminPanel.logout")}
            >
              <span className="hidden md:inline">{t("adminPanel.logout")}</span>
            </IconButton>
          </div>
        )}
      </div>
    </header>
  )
}
