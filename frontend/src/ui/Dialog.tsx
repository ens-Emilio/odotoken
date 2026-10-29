import { useEffect, useRef, type ReactNode } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { X } from 'lucide-react'
import { useLocale } from '../i18n/useLocale'

export function Dialog({ open, onClose, title, children }: { open: boolean; onClose: () => void; title: string; children: ReactNode }) {
  const { t } = useLocale()
  const closeBtnRef = useRef<HTMLButtonElement>(null)
  useEffect(() => {
    if (!open) return
    const prev = document.activeElement as HTMLElement | null
    closeBtnRef.current?.focus()
    const root = document.getElementById('dialog-root')
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      if (e.key === 'Tab' && root) {
        const focusable = Array.from(root.querySelectorAll<HTMLElement>('button,[href],input,select,textarea,[tabindex]:not([tabindex="-1"])'))
        if (focusable.length === 0) return
        const first = focusable[0]
        const last = focusable[focusable.length - 1]
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus() }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus() }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
      prev?.focus()
    }
  }, [open, onClose])

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={onClose}
          role="presentation"
        >
          <motion.div
            id="dialog-root"
            role="dialog"
            aria-modal="true"
            aria-label={title}
            className="max-h-[80vh] w-full max-w-2xl overflow-auto rounded-[var(--radius)] border border-[var(--border)] bg-[var(--bg-elevated)] shadow-[var(--shadow-pop)]"
            initial={{ scale: 0.96, y: 8 }}
            animate={{ scale: 1, y: 0 }}
            exit={{ scale: 0.96, y: 8 }}
            transition={{ type: 'spring', duration: 0.25, bounce: 0.15 }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="sticky top-0 flex items-center justify-between border-b border-[var(--border-soft)] bg-[var(--bg-elevated)] px-5 py-3">
              <h3 id="dialog-title" className="text-[13px] font-semibold text-[var(--text-strong)]">{title}</h3>
              <button
                ref={closeBtnRef}
                aria-label={t('common.close')}
                className="rounded-md p-1 text-[var(--text-soft)] hover:bg-[var(--bg-fill-secondary)] hover:text-[var(--text-strong)] focus:outline-none focus:ring-2 focus:ring-[var(--brand)]"
                onClick={onClose}
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="p-5">{children}</div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}