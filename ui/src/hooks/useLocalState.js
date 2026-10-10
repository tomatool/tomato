import { useCallback, useState } from 'react'

// localStorage is a per-viewer convenience here: a private window or blocked
// site data makes every access throw, so each one is guarded and the caller
// still gets a working default.
function read(key, fallback) {
  try {
    const raw = localStorage.getItem(`tomato-ui.${key}`)
    return raw == null ? fallback : raw
  } catch {
    return fallback
  }
}

export function useLocalNumber(key, fallback) {
  const [value, setValue] = useState(() => {
    const n = parseInt(read(key, ''), 10)
    return Number.isFinite(n) && n > 0 ? n : fallback
  })

  const set = useCallback((next) => {
    setValue(next)
    try { localStorage.setItem(`tomato-ui.${key}`, String(next)) } catch { /* not fatal */ }
  }, [key])

  return [value, set]
}
