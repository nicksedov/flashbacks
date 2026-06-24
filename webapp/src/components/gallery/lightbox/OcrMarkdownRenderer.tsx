import ReactMarkdown, { type Components } from "react-markdown"
import remarkGfm from "remark-gfm"

interface OcrMarkdownRendererProps {
  content: string
}

export function OcrMarkdownRenderer({ content }: OcrMarkdownRendererProps) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={
        {
          h1: (props) => <h1 className="text-xl font-bold mt-4 mb-2" {...props} />,
          h2: (props) => <h2 className="text-lg font-bold mt-3 mb-2" {...props} />,
          h3: (props) => <h3 className="text-base font-bold mt-2 mb-1" {...props} />,
          p: (props) => <p className="mb-2 leading-relaxed" {...props} />,
          ul: (props) => <ul className="list-disc list-inside mb-2 space-y-1" {...props} />,
          ol: (props) => <ol className="list-decimal list-inside mb-2 space-y-1" {...props} />,
          li: (props) => <li className="text-sm" {...props} />,
          code: ({ className, ...props }) => {
            const isInline = !className
            return isInline
              ? <code className="bg-muted-foreground/20 px-1.5 py-0.5 rounded text-sm font-mono" {...props} />
              : <code className="block bg-muted-foreground/20 p-2 rounded text-sm font-mono overflow-x-auto" {...props} />
          },
          blockquote: (props) => <blockquote className="border-l-4 border-primary pl-3 italic my-2" {...props} />,
          table: (props) => <table className="min-w-full border-collapse border border-border my-2" {...props} />,
          th: (props) => <th className="border border-border px-3 py-1.5 font-semibold text-left" {...props} />,
          td: (props) => <td className="border border-border px-3 py-1.5" {...props} />,
          a: (props) => <a className="text-primary underline hover:opacity-80" {...props} />,
          strong: (props) => <strong className="font-semibold" {...props} />,
          em: (props) => <em className="italic" {...props} />,
          hr: (props) => <hr className="border-border my-3" {...props} />,
          img: (props) => <img className="max-w-full h-auto rounded my-2" {...props} />,
        } as Components
      }
    >
      {content}
    </ReactMarkdown>
  )
}
