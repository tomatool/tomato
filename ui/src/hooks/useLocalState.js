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

// useLocalEnum remembers one of a fixed set of strings. Anything else in
// storage — an old value, or something hand-edited — falls back, so a stale
// key cannot put the UI in a state it has no rendering for.
export function useLocalEnum(key, allowed, fallback) {
  const [value, setValue] = useState(() => {
    const raw = read(key, '')
    return allowed.includes(raw) ? raw : fallback
  })

  const set = useCallback((next) => {
    if (!allowed.includes(next)) return
    setValue(next)
    try { localStorage.setItem(`tomato-ui.${key}`, next) } catch { /* not fatal */ }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  return [value, set]
}
