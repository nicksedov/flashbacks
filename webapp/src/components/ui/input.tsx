import * as React from "react"
import { cn } from "@/lib/utils"
import type { LucideIcon } from "lucide-react"

const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<"input">>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        type={type}
        className={cn(
          "flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
          className
        )}
        ref={ref}
        {...props}
      />
    )
  }
)
Input.displayName = "Input"

interface InputWithIconProps extends React.ComponentProps<"input"> {
  /** Icon rendered in the leading (left) slot. */
  leadingIcon?: LucideIcon
  /** Icon rendered in the trailing (right) slot. */
  trailingIcon?: LucideIcon
  /** Additional classes for the slot icons. */
  iconClassName?: string
  /** Additional classes for the wrapper element (e.g. width constraints). */
  wrapperClassName?: string
}

/**
 * Input with leading/trailing icon slots. Icons are non-interactive decorations;
 * use a Button inside a relative wrapper if an interactive action is needed.
 */
const InputWithIcon = React.forwardRef<HTMLInputElement, InputWithIconProps>(
  ({ className, leadingIcon: LeadingIcon, trailingIcon: TrailingIcon, iconClassName, wrapperClassName, ...props }, ref) => (
    <div className={cn("relative", wrapperClassName)}>
      {LeadingIcon && (
        <span className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
          <LeadingIcon className={cn("h-4 w-4 text-muted-foreground", iconClassName)} />
        </span>
      )}
      <Input
        ref={ref}
        className={cn(
          LeadingIcon && "pl-8",
          TrailingIcon && "pr-8",
          className
        )}
        {...props}
      />
      {TrailingIcon && (
        <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
          <TrailingIcon className={cn("h-4 w-4 text-muted-foreground", iconClassName)} />
        </span>
      )}
    </div>
  )
)
InputWithIcon.displayName = "InputWithIcon"

export { Input, InputWithIcon }
