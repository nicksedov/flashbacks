import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { FolderPlus } from "lucide-react"
import { useTranslation } from "@/i18n"

interface AddFolderFormProps {
  onAdd: (path: string) => Promise<void>
  disabled?: boolean
}

export function AddFolderForm({ onAdd, disabled }: AddFolderFormProps) {
  const [path, setPath] = useState("")
  const [isSubmitting, setIsSubmitting] = useState(false)
  const { t } = useTranslation()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = path.trim()
    if (!trimmed) return

    setIsSubmitting(true)
    try {
      await onAdd(trimmed)
      setPath("")
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex gap-2">
      <Input
        value={path}
        onChange={(e) => setPath(e.target.value)}
        placeholder={t("addFolder.placeholder")}
        disabled={disabled || isSubmitting}
        className="flex-1 font-mono text-sm"
      />
      <Button
        type="submit"
        disabled={disabled || isSubmitting || !path.trim()}
        size="sm"
      >
        <FolderPlus className="mr-1.5 h-3.5 w-3.5" />
        {t("addFolder.button")}
      </Button>
    </form>
  )
}
