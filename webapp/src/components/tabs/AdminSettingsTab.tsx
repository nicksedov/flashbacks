import { useState } from "react"
import { Settings, Brain, Server } from "lucide-react"
import { useTranslation } from "@/i18n"
import { UnderlineTabs } from "@/components/ui/underline-tabs"
import { AdminGeneralTab } from "./AdminGeneralTab"
import { AdminAnalysisTab } from "./AdminAnalysisTab"
import { AdminLlmProvidersTab } from "./AdminLlmProvidersTab"

type AdminTab = "general" | "analysis" | "llmProviders"

const TABS = [
  { id: "general" as const, labelKey: "adminSettings.tabs.general" as const, icon: Settings },
  { id: "analysis" as const, labelKey: "adminSettings.tabs.analysis" as const, icon: Brain },
  { id: "llmProviders" as const, labelKey: "adminSettings.tabs.llmProviders" as const, icon: Server },
]

export function AdminSettingsTab() {
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<AdminTab>("general")

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">{t("adminPanel.adminSettings")}</h2>
        <p className="text-sm text-muted-foreground">
          {t("adminPanel.adminSettingsDescription")}
        </p>
      </div>

      <div>
        <UnderlineTabs tabs={TABS} value={activeTab} onValueChange={setActiveTab} />

        <div className="mt-6">
          {activeTab === "general" && <AdminGeneralTab />}
          {activeTab === "analysis" && <AdminAnalysisTab />}
          {activeTab === "llmProviders" && <AdminLlmProvidersTab />}
        </div>
      </div>
    </div>
  )
}
