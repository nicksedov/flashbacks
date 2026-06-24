import { toast } from "sonner"

/**
 * Show a success toast notification.
 */
export function showToastSuccess(message: string): void {
  toast.success(message)
}

/**
 * Show an error toast notification.
 */
export function showToastError(error: string | Error): void {
  const message = error instanceof Error ? error.message : error
  toast.error(message)
}

/**
 * Show an info toast notification.
 */
export function showToastInfo(message: string): void {
  toast.info(message)
}

/**
 * Show a warning toast notification.
 */
export function showToastWarning(message: string): void {
  toast.warning(message)
}

/**
 * Execute an async operation and show success/error toasts automatically.
 * @returns The result of the promise, or undefined if it failed
 */
export async function showToastResult<T>(
  promise: Promise<T>,
  successMsg: string,
  errorMsg: string = "Operation failed"
): Promise<T | undefined> {
  try {
    const result = await promise
    toast.success(successMsg)
    return result
  } catch (err) {
    const message = err instanceof Error ? err.message : errorMsg
    toast.error(message)
    return undefined
  }
}
