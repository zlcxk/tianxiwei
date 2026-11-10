import { reactive } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export interface ToastItem {
  id: number
  kind: ToastKind
  message: string
}

let seq = 0
export const toasts = reactive<ToastItem[]>([])

function push(kind: ToastKind, message: string, duration = 3200) {
  const id = ++seq
  toasts.push({ id, kind, message })
  window.setTimeout(() => {
    const index = toasts.findIndex((t) => t.id === id)
    if (index >= 0) toasts.splice(index, 1)
  }, duration)
}

export const toast = {
  success: (message: string) => push('success', message),
  error: (message: string) => push('error', message, 4200),
  info: (message: string) => push('info', message),
}

export function dismissToast(id: number) {
  const index = toasts.findIndex((t) => t.id === id)
  if (index >= 0) toasts.splice(index, 1)
}
