import { useState } from "react"
import { Settings, Brain, Server } from "lucide-react"
import { useTranslation } from "@/i18n"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
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
        <Tabs variant="underline" value={activeTab} onValueChange={(v) => setActiveTab(v as AdminTab)}>
          <TabsList>
            {TABS.map(({ id, labelKey, icon: Icon }) => (
              <TabsTrigger key={id} value={id}>
                <Icon className="h-3.5 w-3.5" />
                {t(labelKey)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <div className="mt-6">
          {activeTab === "general" && <AdminGeneralTab />}
          {activeTab === "analysis" && <AdminAnalysisTab />}
          {activeTab === "llmProviders" && <AdminLlmProvidersTab />}
        </div>
      </div>
    </div>
  )
}
