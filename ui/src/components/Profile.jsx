import { useMemo, useState } from 'react'
import {
  costliest, fmtBytes, fmtMs, laneMax, linePath, profileFacts, samplesIn, scenarioBands, timelineLanes,
} from '../lib/profile.js'

// A scenario's profile on its card: what it cost the app, the dependency it
// waited on most, what the app logged, and the app's CPU and memory over the
// scenario as a small chart.
export function ProfileStrip({ profile, samples }) {
  const facts = profileFacts(profile)
  const inside = useMemo(() => samplesIn(samples ?? [], profile), [samples, profile])
  if (!facts.length) return null
  return (
    <div className="profile-strip" aria-label="What the scenario cost the app">
      <Spark profile={profile} samples={inside} />
      {facts.map((f) => (
        <span className="pf" key={f.key} data-warn={f.warn ? 'true' : undefined} title={f.title}>
          <span className="pf-label">{f.label}</span>
          <span className="pf-value">{f.value}</span>
        </span>
      ))}
    </div>
  )
}

function Spark({ profile, samples }) {
  const app = samples.filter((s) => s.app)
  if (app.length < 2) return null
  const from = new Date(profile.start).getTime()
  const to = new Date(profile.end).getTime()
  const W = 96, H = 22
  const cpu = app.map((s) => [new Date(s.at).getTime(), s.app.cpu])
  const mem = app.map((s) => [new Date(s.at).getTime(), s.app.rss])
  const memMax = Math.max(...mem.map(([, v]) => v)) * 1.1
  return (
    <svg className="spark" width={W} height={H} viewBox={`0 0 ${W} ${H}`} role="img" aria-label="App CPU and memory during the scenario">
      <path className="spark-mem" d={linePath(mem, { from, to, max: memMax, width: W, height: H })} />
      <path className="spark-cpu" d={linePath(cpu, { from, to, max: laneMax({ unit: '%', points: cpu }), width: W, height: H })} />
    </svg>
  )
}

// The whole run as a timeline: a lane each for the app's CPU and memory and
// each container's CPU, with the scenarios as bands across them; hovering a
// band names it. Under it, the scenarios that cost the app the most.
export function ProfileTimeline({ runState, onSelectScenario }) {
  const samples = runState.samples ?? []
  const lanes = useMemo(() => timelineLanes(samples), [samples])
  const bands = useMemo(() => scenarioBands(runState.scProfile ?? {}), [runState.scProfile])
  const [hover, setHover] = useState(null)

  if (!samples.length) {
    return (
      <div className="empty">
        <h3>No profile yet</h3>
        <p>Run the suite: tomato measures the app and its containers while the scenarios run, from outside the app.</p>
      </div>
    )
  }
  const from = new Date(samples[0].at).getTime()
  const to = new Date(samples[samples.length - 1].at).getTime()
  const W = 600, H = 28
  const span = Math.max(1, to - from)
  const x = (t) => ((t - from) / span) * W

  return (
    <div className="profile-timeline">
      <div className="pt-meta">{fmtMs(to - from)} · {bands.length} scenarios · {samples.length} samples</div>
      {lanes.map((lane) => {
        const max = laneMax(lane)
        const peak = lane.points.reduce((m, [, v]) => Math.max(m, v ?? 0), 0)
        return (
          <div className="pt-lane" key={lane.key}>
            <div className="pt-label">
              <span>{lane.label}</span>
              <span className="pt-peak">{lane.unit === '%' ? `${Math.round(peak)}%` : fmtBytes(peak)}</span>
            </div>
            <svg className="pt-chart" viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" role="img" aria-label={lane.label}>
              {bands.map((b, i) => (
                <rect
                  key={b.name} className="pt-band" data-odd={i % 2 ? 'true' : undefined} data-hover={hover === b.name ? 'true' : undefined}
                  x={x(b.from)} y={0} width={Math.max(1, x(b.to) - x(b.from))} height={H}
                  onMouseEnter={() => setHover(b.name)} onMouseLeave={() => setHover(null)}
                  onClick={() => onSelectScenario?.(b.name)}
                >
                  <title>{b.name}</title>
                </rect>
              ))}
              <path className="pt-line" data-unit={lane.unit} d={linePath(lane.points, { from, to, max, width: W, height: H })} vectorEffect="non-scaling-stroke" />
            </svg>
          </div>
        )
      })}
      <div className="pt-hover">{hover ? hover : 'Hover a band to see its scenario.'}</div>

      <table className="pt-table">
        <thead>
          <tr><th>scenario</th><th>time</th><th>app CPU</th><th>peak</th><th>waiting on</th></tr>
        </thead>
        <tbody>
          {costliest(runState.scProfile ?? {}).slice(0, 15).map(({ name, profile }) => {
            const dep = Object.values(profile?.network ?? {}).sort((a, b) => b.waitMs - a.waitMs)[0]
            return (
              <tr key={name} onMouseEnter={() => setHover(name)} onMouseLeave={() => setHover(null)} onClick={() => onSelectScenario?.(name)}>
                <td className="pt-name" title={name}>{name}</td>
                <td>{fmtMs(profile?.durationMs)}</td>
                <td>{fmtMs(profile?.app?.cpuMs)}</td>
                <td>{fmtBytes(profile?.app?.peakRss)}</td>
                <td>{dep?.waitMs > 0 ? `${dep.container} ${fmtMs(dep.waitMs)}` : ''}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
