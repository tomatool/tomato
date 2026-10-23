// What the run's profile says, shaped for the scenario cards and the profile
// timeline. The runner sends a Summary per scenario (internal/profile) and
// samples of the app and its containers, about four a second.

export const fmtBytes = (b) => {
  if (b == null) return ''
  if (b >= 1 << 30) return `${(b / (1 << 30)).toFixed(1)} GB`
  if (b >= 1 << 20) return `${Math.round(b / (1 << 20))} MB`
  if (b >= 1 << 10) return `${Math.round(b / (1 << 10))} KB`
  return `${b} B`
}

export const fmtDelta = (d) => (d < 0 ? `−${fmtBytes(-d)}` : `+${fmtBytes(d)}`)

export const fmtMs = (ms) => {
  if (ms == null) return ''
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms / 1000)}s`
}

const time = (t) => new Date(t).getTime()

// The dependency the app waited on longest in a scenario, if any.
export function slowestDependency(profile) {
  let best = null
  for (const t of Object.values(profile?.network ?? {})) {
    if (t.waitMs > 0 && (!best || t.waitMs > best.waitMs)) best = t
  }
  return best
}

// The facts a scenario card shows, most telling first.
export function profileFacts(profile) {
  if (!profile) return []
  const facts = []
  const app = profile.app
  if (app) {
    facts.push({ key: 'cpu', label: 'CPU', value: fmtMs(app.cpuMs) })
    facts.push({ key: 'mem', label: 'peak', value: fmtBytes(app.peakRss) })
    const d = app.rssEnd - app.rssStart
    if (Math.abs(d) >= 1 << 20) facts.push({ key: 'delta', label: 'Δ mem', value: fmtDelta(d), warn: d >= 32 << 20 })
  }
  const dep = slowestDependency(profile)
  if (dep) facts.push({ key: 'dep', label: dep.container, value: `${fmtMs(dep.waitMs)} · ${dep.roundTrips} round trip${dep.roundTrips === 1 ? '' : 's'}` })
  const logs = profile.logs
  if (logs?.errors) facts.push({ key: 'err', label: 'logged', value: `${logs.errors} error${logs.errors === 1 ? '' : 's'}`, warn: true, title: logs.examples?.join('\n') })
  else if (logs?.warnings) facts.push({ key: 'warn', label: 'logged', value: `${logs.warnings} warning${logs.warnings === 1 ? '' : 's'}`, title: logs.examples?.join('\n') })
  return facts
}

// The samples inside a scenario's window.
export function samplesIn(samples, profile) {
  if (!profile?.start || !profile?.end) return []
  const from = time(profile.start), to = time(profile.end)
  return samples.filter((s) => { const t = time(s.at); return t >= from && t <= to })
}

// A series as an SVG path over width × height: x is time between from and
// to, y the value between 0 and max (the top). Points without a value are
// skipped.
export function linePath(points, { from, to, max, width, height }) {
  if (!points.length || to <= from || !(max > 0)) return ''
  let d = ''
  for (const [t, v] of points) {
    if (v == null) continue
    const x = ((t - from) / (to - from)) * width
    const y = height - Math.min(1, v / max) * height
    d += `${d ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`
  }
  return d
}

// The lanes of the run timeline: the app's CPU and memory, then each
// container's CPU, busiest first.
export function timelineLanes(samples) {
  const app = samples.filter((s) => s.app)
  const lanes = []
  if (app.length) {
    lanes.push({ key: 'app-cpu', label: 'app CPU', unit: '%', points: app.map((s) => [time(s.at), s.app.cpu]) })
    lanes.push({ key: 'app-mem', label: 'app memory', unit: 'bytes', points: app.map((s) => [time(s.at), s.app.rss]) })
  }
  const names = new Set()
  for (const s of samples) for (const n of Object.keys(s.containers ?? {})) names.add(n)
  const ctr = [...names].map((name) => ({
    key: `ctr-${name}`, label: `${name} CPU`, unit: '%',
    points: samples.filter((s) => s.containers?.[name]).map((s) => [time(s.at), s.containers[name].cpu]),
  }))
  const busy = (l) => l.points.reduce((n, [, v]) => n + (v ?? 0), 0)
  ctr.sort((a, b) => busy(b) - busy(a))
  return lanes.concat(ctr)
}

// The scale a lane is drawn against: the peak, rounded up, and never below
// a floor so a quiet lane does not look busy.
export function laneMax(lane) {
  const peak = lane.points.reduce((m, [, v]) => Math.max(m, v ?? 0), 0)
  if (lane.unit === '%') return Math.max(100, Math.ceil(peak / 50) * 50)
  return Math.max(64 << 20, peak * 1.1)
}

// The scenarios that ran, as bands over the timeline, in the order they ran.
export function scenarioBands(scProfile) {
  return Object.entries(scProfile)
    .filter(([, p]) => p?.start && p?.end)
    .map(([name, p]) => ({ name, from: time(p.start), to: time(p.end), profile: p }))
    .sort((a, b) => a.from - b.from)
}

// The scenarios, most expensive first, for the table under the timeline.
export function costliest(scProfile) {
  return Object.entries(scProfile)
    .map(([name, p]) => ({ name, profile: p, cpu: p?.app?.cpuMs ?? -1 }))
    .sort((a, b) => b.cpu - a.cpu)
}
