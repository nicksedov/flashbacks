import { useCallback, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import type { Theme } from "@/theme"
import { THEMES } from "@/theme/options"
import { updateUserSettings } from "@/api/endpoints"
import { useSettings } from "@/providers/useSettings"
import { Globe } from "lucide-react"
import { useTranslation } from "@/i18n"

export function SettingsTab() {
  const { theme, setTheme, language, setLanguage } = useSettings()
  const { t } = useTranslation()

  const [selectedTheme, setSelectedTheme] = useState<Theme>(theme as Theme)
  const [selectedLanguage, setSelectedLanguage] = useState<"en" | "ru">(language)
  const [isSaving, setIsSaving] = useState(false)

  const handleSavePreferences = useCallback(async () => {
    setIsSaving(true)
    try {
      await updateUserSettings({ theme: selectedTheme, language: selectedLanguage })
      setTheme(selectedTheme)
      setLanguage(selectedLanguage)
      toast.success(t("settings.preferencesSaved"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.saveFailed"))
    } finally {
      setIsSaving(false)
    }
  }, [selectedTheme, selectedLanguage, setTheme, setLanguage, t])

  return (
    <div className="space-y-6">
      {/* Preferences Header */}
      <div>
        <h2 className="text-lg font-semibold mb-1">{t("settings.preferences")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("settings.preferencesDescription")}
        </p>
      </div>

      {/* Theme and Language Settings card */}
      <Card>
        <CardHeader>
          <CardTitle>{t("settings.preferences")}</CardTitle>
          <CardDescription>{t("settings.preferencesDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Theme Setting */}
          <div className="space-y-2">
            <Label htmlFor="theme-select">{t("settings.theme")}</Label>
            <Select value={selectedTheme} onValueChange={(v) => setSelectedTheme(v as Theme)}>
              <SelectTrigger id="theme-select">
                <SelectValue placeholder={t("settings.selectTheme")} />
              </SelectTrigger>
              <SelectContent>
                {THEMES.map(({ id, labelKey, icon: Icon, iconClassName, swatch }) => (
                  <SelectItem key={id} value={id}>
                    <span className="flex items-center gap-2">
                      <div className={`h-3 w-3 rounded-full ${swatch}`} />
                      <Icon className={`h-4 w-4 ${iconClassName}`} />
                      {t(labelKey)}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Language Setting */}
          <div className="space-y-2">
            <Label htmlFor="language-select">{t("settings.language")}</Label>
            <Select value={selectedLanguage} onValueChange={(v) => setSelectedLanguage(v as "en" | "ru")}>
              <SelectTrigger id="language-select">
                <SelectValue>
                  <span className="flex items-center gap-2">
                    <Globe className="h-4 w-4" />
                    {selectedLanguage === "en" ? "English" : "Русский"}
                  </span>
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="en">
                  <span className="flex items-center gap-2">
                    <Globe className="h-4 w-4" />
                    English
                  </span>
                </SelectItem>
                <SelectItem value="ru">
                  <span className="flex items-center gap-2">
                    <Globe className="h-4 w-4" />
                    Русский
                  </span>
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Save Button - Left adjusted, not full width */}
          <Button
            onClick={handleSavePreferences}
            disabled={isSaving || (selectedTheme === theme && selectedLanguage === language)}
          >
            {isSaving ? t("common.saving") : t("settings.savePreferences")}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}
