export function fmtDur(ms) {
  if (ms == null) return ''
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.floor(ms / 60000)}m${Math.round((ms % 60000) / 1000)}s`
}

export function fmtClock(ms) {
  const t = Math.max(0, ms) / 1000
  const m = Math.floor(t / 60)
  return `${String(m).padStart(2, '0')}:${(t - m * 60).toFixed(1).padStart(4, '0')}`
}

export function fmtBytes(n) {
  if (n == null) return ''
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  return `${(n / 1048576).toFixed(1)} MB`
}

export const plural = (n, word) => `${n} ${word}${n === 1 ? '' : 's'}`

export const clip = (s, n) => (String(s).length > n ? `${String(s).slice(0, n - 1)}…` : String(s))

export function when(ts) {
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return String(ts ?? '')
  const now = new Date()
  const today = d.toDateString() === now.toDateString()
  const yesterday = new Date(now.getTime() - 86400000).toDateString() === d.toDateString()
  const hhmmss = d.toTimeString().slice(0, 8)
  if (today) return `today ${hhmmss}`
  if (yesterday) return `yesterday ${hhmmss}`
  return `${d.toISOString().slice(0, 10)} ${hhmmss}`
}
