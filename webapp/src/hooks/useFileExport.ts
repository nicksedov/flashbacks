import { useCallback } from "react"
import { marked } from "marked"
import { useTranslation } from "@/i18n"

interface UseFileExportReturn {
  getFileName: () => string
  handleSaveMd: () => void
  handleSaveHtml: () => void
}

export function useFileExport(markdownContent: string | undefined, imagePath: string | null): UseFileExportReturn {
  const { language } = useTranslation()
  const getFileName = useCallback(() => {
    if (!imagePath) return "document"
    const base = imagePath.split(/[\\/]/).pop() || "document"
    return base.replace(/\.[^.]+$/, "")
  }, [imagePath])

  const handleSaveMd = useCallback(() => {
    if (!markdownContent) return
    const blob = new Blob([markdownContent], { type: "text/markdown" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `${getFileName()}.md`
    a.click()
    URL.revokeObjectURL(url)
  }, [markdownContent, getFileName])

  const handleSaveHtml = useCallback(() => {
    if (!markdownContent) return

    const html = marked(markdownContent, {
      gfm: true,
      breaks: true,
    })

    const fullHtml = `<!DOCTYPE html>
<html lang="${language}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>${getFileName()}</title>
<style>
body { font-family: system-ui, -apple-system, sans-serif; max-width: 800px; margin: 40px auto; padding: 0 20px; line-height: 1.6; color: #333; }
h1, h2, h3 { margin-top: 1.5em; margin-bottom: 0.5em; }
p { margin-bottom: 1em; }
table { border-collapse: collapse; width: 100%; margin: 1em 0; }
th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
th { background: #f5f5f5; font-weight: bold; }
code { background: #f4f4f4; padding: 2px 6px; border-radius: 3px; font-family: monospace; }
pre { background: #f4f4f4; padding: 1em; border-radius: 5px; overflow-x: auto; }
pre code { background: none; padding: 0; }
blockquote { border-left: 4px solid #ddd; margin: 1em 0; padding: 0.5em 1em; color: #666; }
a { color: #0066cc; }
ul, ol { margin-left: 1.5em; }
</style>
</head>
<body>
${html}
</body>
</html>`

    const blob = new Blob([fullHtml], { type: "text/html" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `${getFileName()}.html`
    a.click()
    URL.revokeObjectURL(url)
  }, [markdownContent, getFileName, language])

  return { getFileName, handleSaveMd, handleSaveHtml }
}
