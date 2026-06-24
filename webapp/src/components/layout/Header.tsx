import { useTranslation } from "@/i18n"
import { useAuth } from "@/providers/AuthProvider"
import { getAvatarUrl } from "@/api/endpoints"
import { LogOut, Settings, User, Menu } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { IconButton } from "@/components/ui/icon-button"

interface HeaderProps {
  onTabChange: (tab: string) => void
  onMobileMenuToggle: () => void
}

export function Header({ onTabChange, onMobileMenuToggle }: HeaderProps) {
  const { t } = useTranslation()
  const { user, logout } = useAuth()

  return (
    <header
      className="flex-shrink-0 z-10 px-4 sm:px-6 py-3"
      style={{ backgroundColor: 'var(--color-header)' }}
    >
      <div className="flex items-center justify-between gap-3">
        {/* Mobile menu button */}
        <button
          type="button"
          className="flex h-9 w-9 items-center justify-center rounded-md hover:bg-muted transition-colors md:hidden"
          onClick={onMobileMenuToggle}
          aria-label={t("header.menu")}
          title={t("header.menu")}
        >
          <Menu className="h-5 w-5" />
        </button>

        {/* Spacer for desktop alignment */}
        <div className="hidden md:block" />

        {user && (
          <div className="flex items-center gap-3">
            <button
              type="button"
              className="flex items-center gap-2 rounded-md px-2 py-1 hover:bg-muted transition-colors cursor-pointer"
              onClick={() => onTabChange("profile")}
            >
              {user.hasAvatar ? (
                <img src={getAvatarUrl(user.id)} className="h-6 w-6 rounded-full object-cover" alt="" />
              ) : (
                <User className="h-4 w-4 text-muted-foreground" />
              )}
              <span className="text-sm font-medium">{user.displayName}</span>
              <Badge variant="outline" className="text-xs">
                {user.role === "admin" ? t("adminPanel.roleAdmin") : t("adminPanel.roleUser")}
              </Badge>
            </button>
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
            >
              {t("adminPanel.logout")}
            </IconButton>
          </div>
        )}
      </div>
    </header>
  )
}
