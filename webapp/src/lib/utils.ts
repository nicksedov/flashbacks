import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatSize(size: number): string {
  if (size < 1024) return `${size} B`
  const units = ["KB", "MB", "GB", "TB", "PB", "EB"]
  const exp = Math.min(Math.floor(Math.log(size) / Math.log(1024)) - 1, units.length - 1)
  const value = size / Math.pow(1024, exp + 1)
  return `${value.toFixed(1)} ${units[exp]}`
}
