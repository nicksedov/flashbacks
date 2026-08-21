import { useCallback } from "react"
import { renderToStaticMarkup } from "react-dom/server"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { useTranslation } from "@/i18n"
import { downloadBlob } from "@/lib/download"
import { markdownComponents } from "@/components/gallery/lightbox/markdownComponents"

interface UseFileExportReturn {
  getFileName: () => string
  handleSaveMd: () => void
  handleSaveHtml: () => void
}

/** Render markdown to static HTML using the same component overrides as the on-screen renderer. */
function markdownToHtml(markdown: string): string {
  return renderToStaticMarkup(
    <ReactMarkdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
      {markdown}
    </ReactMarkdown>
  )
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
    downloadBlob(new Blob([markdownContent], { type: "text/markdown" }), `${getFileName()}.md`)
  }, [markdownContent, getFileName])

  const handleSaveHtml = useCallback(() => {
    if (!markdownContent) return

    const html = markdownToHtml(markdownContent)

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

    downloadBlob(new Blob([fullHtml], { type: "text/html" }), `${getFileName()}.html`)
  }, [markdownContent, getFileName, language])

  return { getFileName, handleSaveMd, handleSaveHtml }
}
