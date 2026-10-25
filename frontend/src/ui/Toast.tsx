import { useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { AlertTriangle, CircleAlert, CircleCheck } from 'lucide-react'
import { nextToastID, registerToastPush, type ToastItem } from './toast-store'

export function ToastHost() {
  const [items, setItems] = useState<ToastItem[]>([])

  useEffect(() => {
    registerToastPush((tone, text) => {
      const id = nextToastID()
      setItems((prev) => [...prev, { id, tone, text }])
      setTimeout(() => setItems((prev) => prev.filter((t) => t.id !== id)), 6000)
    })
    return () => registerToastPush(null)
  }, [])

  const icons = {
    ok: <CircleCheck className="h-4 w-4 text-[var(--brand-ink)]" />,
    warn: <AlertTriangle className="h-4 w-4 text-[var(--warn-ink)]" />,
    error: <CircleAlert className="h-4 w-4 text-[var(--danger-ink)]" />,
  }

  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-[60] flex flex-col gap-2">
      <AnimatePresence>
        {items.map((t) => (
          <motion.div
            key={t.id}
            className="glass pointer-events-auto flex items-center gap-2.5 rounded-lg px-4 py-2.5 text-[12px] text-[var(--text-strong)] shadow-[var(--shadow-pop)]"
            initial={{ opacity: 0, x: 24 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 24 }}
            transition={{ type: 'spring', duration: 0.3, bounce: 0.2 }}
          >
            {icons[t.tone]}
            {t.text}
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  )
}