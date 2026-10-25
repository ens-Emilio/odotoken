export type ToastTone = 'ok' | 'warn' | 'error'

export interface ToastItem {
  id: number
  tone: ToastTone
  text: string
}

let pushFn: ((tone: ToastTone, text: string) => void) | null = null
let seq = 0

export function toast(tone: ToastTone, text: string) {
  pushFn?.(tone, text)
}

export function registerToastPush(push: ((tone: ToastTone, text: string) => void) | null) {
  pushFn = push
}

export function nextToastID() {
  return ++seq
}
