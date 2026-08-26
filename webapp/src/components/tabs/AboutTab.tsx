import { useState } from "react"
import type { ElementType } from "react"
import { Mail, Send, Globe, ExternalLink, Info, Scale, Package } from "lucide-react"
import { useTranslation } from "@/i18n"
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"

interface ContactLink {
  label: string
  href: string
  icon: ElementType
}

interface ThirdPartyComponent {
  name: string
  license: string
  url: string
}

/**
 * Brand icons are not shipped by lucide-react (brand glyphs were removed), so they
 * are provided as inline SVGs (paths from Simple Icons). They inherit the current
 * text color via `currentColor` and therefore follow the active theme token.
 */
function BrandIcon({ path, className }: { path: string; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="currentColor"
      className={className}
      aria-hidden="true"
    >
      <path d={path} />
    </svg>
  )
}

const GithubIcon = ({ className }: { className?: string }) => (
  <BrandIcon
    className={className}
    path="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"
  />
)

const LinkedinIcon = ({ className }: { className?: string }) => (
  <BrandIcon
    className={className}
    path="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433c-1.144 0-2.063-.926-2.063-2.065 0-1.138.92-2.063 2.063-2.063 1.14 0 2.064.925 2.064 2.063 0 1.139-.925 2.065-2.064 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"
  />
)

const XTwitterIcon = ({ className }: { className?: string }) => (
  <BrandIcon
    className={className}
    path="M18.901 1.153h3.68l-8.04 9.19L24 22.846h-7.406l-5.8-7.584-6.638 7.584H.474l8.6-9.83L0 1.154h7.594l5.243 6.932ZM17.61 20.644h2.039L6.486 3.24H4.298Z"
  />
)

const FacebookIcon = ({ className }: { className?: string }) => (
  <BrandIcon
    className={className}
    path="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z"
  />
)

const contacts: ContactLink[] = [
  { label: "Email", href: "mailto:nasedov@gmail.com", icon: Mail },
  { label: "Website", href: "https://flashbacks.life", icon: Globe },
  { label: "Telegram", href: "https://t.me/nicksedov", icon: Send },
  { label: "GitHub", href: "https://github.com/nicksedov", icon: GithubIcon },
  { label: "LinkedIn", href: "https://www.linkedin.com/in/nikolay-sedov-16706366", icon: LinkedinIcon },
  { label: "Twitter / X", href: "https://twitter.com/n_a_sedov", icon: XTwitterIcon },
  { label: "Facebook", href: "https://www.facebook.com/nikolay.sedov.98", icon: FacebookIcon },
]

const thirdPartyComponents: ThirdPartyComponent[] = [
  { name: "Radix UI Primitives", license: "MIT", url: "https://www.radix-ui.com/" },
  { name: "React", license: "MIT", url: "https://react.dev/" },
  { name: "react-leaflet", license: "Hippocratic License 2.1", url: "https://react-leaflet.js.org/" },
  { name: "Leaflet", license: "BSD-2-Clause", url: "https://leafletjs.com/" },
  { name: "lucide-react", license: "ISC", url: "https://lucide.dev/" },
  { name: "react-markdown", license: "MIT", url: "https://github.com/remarkjs/react-markdown" },
  { name: "remark-gfm", license: "MIT", url: "https://github.com/remarkjs/remark-gfm" },
  { name: "sonner", license: "MIT", url: "https://sonner.emilkowal.ski/" },
  { name: "class-variance-authority", license: "Apache-2.0", url: "https://cva.style/" },
  { name: "clsx", license: "MIT", url: "https://github.com/lukeed/clsx" },
  { name: "tailwind-merge", license: "MIT", url: "https://github.com/dcastil/tailwind-merge" },
]

const features = [
  "about.feature.deduplication",
  "about.feature.metadata",
  "about.feature.ocr",
  "about.feature.gallery",
  "about.feature.smartSearch",
  "about.feature.enhancement",
] as const

export function AboutTab() {
  const { t } = useTranslation()
  const [section, setSection] = useState("about")

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-lg font-semibold mb-1">{t("about.title")}</h2>
        <p className="text-sm text-muted-foreground">{t("about.description")}</p>
      </div>

      <Tabs value={section} onValueChange={setSection} variant="underline">
        <TabsList>
          <TabsTrigger value="about">
            <Info className="h-4 w-4" />
            {t("about.section.about")}
          </TabsTrigger>
          <TabsTrigger value="license">
            <Scale className="h-4 w-4" />
            {t("about.section.license")}
          </TabsTrigger>
          <TabsTrigger value="thirdParty">
            <Package className="h-4 w-4" />
            {t("about.section.thirdParty")}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="about" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>{t("about.aboutTitle")}</CardTitle>
              <CardDescription>{t("about.aboutText")}</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="space-y-2 text-sm">
                {features.map((featureKey) => (
                  <li key={featureKey} className="flex items-start gap-2">
                    <span className="mt-1.5 h-1.5 w-1.5 flex-shrink-0 rounded-full bg-primary" />
                    <span>{t(featureKey)}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>{t("about.author")}</CardTitle>
              <CardDescription className="flex flex-col items-start gap-3 sm:flex-row sm:items-center sm:gap-4">
                <span className="text-base font-semibold text-foreground">Nikolay Sedov</span>
                <div className="flex flex-wrap items-center gap-2">
                  {contacts.map((contact) => (
                    <Button
                      key={contact.href}
                      asChild
                      variant="outline"
                      size="icon"
                      className="rounded-full"
                      aria-label={contact.label}
                      title={contact.label}
                    >
                      <a href={contact.href} target="_blank" rel="noopener noreferrer">
                        <contact.icon className="h-4 w-4" />
                      </a>
                    </Button>
                  ))}
                </div>
              </CardDescription>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">{t("about.copyright")}</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="license">
          <Card>
            <CardHeader>
              <CardTitle>MIT License</CardTitle>
              <CardDescription>{t("about.license.description")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <p className="text-sm">{t("about.license.permission")}</p>
              <p className="text-sm text-muted-foreground">{t("about.license.copyright")}</p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="thirdParty">
          <Card>
            <CardHeader>
              <CardTitle>{t("about.section.thirdParty")}</CardTitle>
              <CardDescription>{t("about.thirdParty.description")}</CardDescription>
            </CardHeader>
            <CardContent>
              <ul className="divide-y">
                {thirdPartyComponents.map((component) => (
                  <li
                    key={component.name}
                    className="flex items-center justify-between gap-4 py-3 first:pt-0 last:pb-0"
                  >
                    <a
                      href={component.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 text-sm font-medium text-foreground underline-offset-4 hover:underline"
                    >
                      {component.name}
                      <ExternalLink className="h-3 w-3 text-muted-foreground" />
                    </a>
                    <span className="text-sm text-muted-foreground">{component.license}</span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
