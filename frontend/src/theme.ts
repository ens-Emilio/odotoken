import { useEffect, useState } from 'react'
import { getPalette, type PaletteId } from './themes'

export type ThemeMode = 'light' | 'dark' | 'system'

function applyTheme(mode: ThemeMode, palette: PaletteId) {
  const dark = mode === 'dark' || (mode === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', dark)
  
  const p = getPalette(palette)
  const colors = dark ? p.colors.dark : p.colors.light
  
  // Remove old custom properties
  const root = document.documentElement
  const existingVars = Array.from(root.style)
  for (const v of existingVars) {
    if (v.startsWith('--')) root.style.removeProperty(v)
  }
  
  // Apply new palette colors
  for (const [key, value] of Object.entries(colors)) {
    root.style.setProperty(key, value)
  }
}

export function useTheme() {
  const [mode, setMode] = useState<ThemeMode>(() => (localStorage.getItem('odt-theme') as ThemeMode) || 'system')
  const [palette, setPalette] = useState<PaletteId>(() => (localStorage.getItem('odt-palette') as PaletteId) || 'original')
  
  useEffect(() => {
    applyTheme(mode, palette)
    localStorage.setItem('odt-theme', mode)
    localStorage.setItem('odt-palette', palette)
    
    if (mode === 'system') {
      const mq = window.matchMedia('(prefers-color-scheme: dark)')
      const onChange = () => applyTheme('system', palette)
      mq.addEventListener('change', onChange)
      return () => mq.removeEventListener('change', onChange)
    }
  }, [mode, palette])
  
  return { mode, setMode, palette, setPalette }
}