import { Button, type ButtonProps } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { Loader2 } from "lucide-react"
import type { ElementType } from "react"

interface IconButtonProps extends ButtonProps {
  /** Icon component to display */
  icon: ElementType
  /** Whether to show loading spinner instead of icon */
  isLoading?: boolean
  /** Text to display when loading */
  loadingText?: string
  /** Text to display normally */
  children?: React.ReactNode
  /** Additional classes for the icon */
  iconClassName?: string
}

/**
 * Reusable button component with icon and loading state support.
 */
export function IconButton({
  icon: Icon,
  isLoading,
  loadingText,
  children,
  className,
  iconClassName,
  disabled,
  ...props
}: IconButtonProps) {
  const isDisabled = disabled || isLoading

  return (
    <Button className={cn("gap-1.5", className)} disabled={isDisabled} {...props}>
      {isLoading ? (
        <Loader2 className={cn("h-3.5 w-3.5 animate-spin", iconClassName)} />
      ) : (
        <Icon className={cn("h-3.5 w-3.5", iconClassName)} />
      )}
      {isLoading && loadingText ? loadingText : children}
    </Button>
  )
}
