import { useMemo } from 'react'
import { fmtDur, plural } from '../lib/format.js'
import { fmtBytes, fmtMs } from '../lib/profile.js'
import { splitKey } from '../lib/run.js'
import { ProfileTimeline } from './Profile.jsx'

const firstLine = (s) => String(s ?? '').split('\n')[0]

// The step a scenario failed on: its pickle index from the run state, and
// its text from the parsed feature, where a Background's steps come first.
function failedStep(run, features, name) {
  const hit = Object.entries(run.stepStatus).find(([k, v]) => v === 'failed' && splitKey(k)[0] === name)
  if (!hit) return null
  const index = splitKey(hit[0])[1]
  const feature = features.find((f) => f.scenarios?.some((s) => s.name === name))
  const bg = feature?.background ?? []
  const step = index < bg.length ? bg[index] : feature?.scenarios.find((s) => s.name === name)?.steps?.[index - bg.length]
  return { index, text: step ? `${String(step.keyword).trim()} ${step.text}` : `step ${index + 1}`, error: run.stepError[hit[0]] }
}

function Tile({ label, value, note, status }) {
  return (
    <div className="rp-tile">
      <span className="rp-tile-label">{label}</span>
      <span className="rp-tile-value" data-status={status}>{value}</span>
      {note && <span className="rp-tile-note">{note}</span>}
    </div>
  )
}

// Horizontal bars, one hue, anchored on the left: magnitude only. A failed
// row is told by its pip and its label as well as its colour.
function Bars({ rows, max, unit = fmtDur }) {
  const top = max ?? Math.max(1, ...rows.map((r) => r.value))
  return (
    <div className="rp-bars">
      {rows.map((r) => (
        <div className="rp-bar-row" key={r.key} role="group" aria-label={`${r.label}: ${unit(r.value)}`}>
          <span className="rp-bar-name" title={r.label}>
            {r.status && <i className="pip" data-status={r.status} />} {r.label}
          </span>
          <span className="rp-bar-track"><span className="rp-bar" data-status={r.status === 'failed' ? 'failed' : undefined} style={{ width: `${Math.max(1, (r.value / top) * 100)}%` }} /></span>
          <span className="rp-bar-value">{unit(r.value)}</span>
        </div>
      ))}
    </div>
  )
}

// How the run ended, and what it cost: the count and the time, the features
// and their scenarios with what failed, the slowest scenarios, and what the
// app and its dependencies did while they ran (the profile, when the run
// had one).
export function Report({ scene, run, features }) {
  const { runId, containers, app, error, finished, summary } = scene
  const names = run.scOrder ?? []
  const st = run.scStatus
  const count = (status) => names.filter((n) => st[n] === status).length
  const passed = count('passed'); const failed = count('failed'); const undef = count('undefined')
  const total = summary?.total || names.length
  const steps = Object.values(run.stepStatus).reduce((acc, s) => { acc[s] = (acc[s] ?? 0) + 1; return acc }, {})
  const wall = finished?.at ?? scene.at
  const depsMs = containers.reduce((a, c) => a + (c.ms ?? 0), 0)
  const appMs = app?.ms ?? 0
  const scenariosMs = names.reduce((a, n) => a + (run.scDur[n] ?? 0), 0)
  const interrupted = !error && finished?.status === 'interrupted'
  const status = error || failed > 0 || finished?.status === 'failed' ? 'failed' : interrupted ? 'interrupted' : 'passed'

  const verdict = error
    ? `The run failed while ${{ containers: 'starting the dependencies', resources: 'initialising the resources', app: 'starting the app', config: 'reading tomato.yml' }[error.phase] ?? 'starting up'}`
    : interrupted ? `The run was interrupted after ${plural(names.length, 'scenario')}`
    : failed > 0 ? `${failed} of ${plural(total, 'scenario')} failed`
      : undef > 0 ? `${passed} passed, ${undef} undefined`
        : total ? `All ${plural(total, 'scenario')} passed` : 'No scenario ran'

  const slowest = useMemo(() => names
    .map((n) => ({ key: n, label: n, value: run.scDur[n] ?? 0, status: st[n] }))
    .filter((r) => r.value > 0).sort((a, b) => b.value - a.value).slice(0, 8), [names, run.scDur, st])

  const profiles = Object.values(run.scProfile ?? {})
  const perf = useMemo(() => {
    if (!profiles.length) return null
    const apps = profiles.map((p) => p.app).filter(Boolean)
    const cpuMs = apps.reduce((a, p) => a + (p.cpuMs ?? 0), 0)
    const peak = apps.reduce((a, p) => Math.max(a, p.peakRss ?? 0), 0)
    const ordered = Object.entries(run.scProfile).filter(([, p]) => p?.app).sort((a, b) => new Date(a[1].start) - new Date(b[1].start))
    const growth = ordered.length ? (ordered[ordered.length - 1][1].app.rssEnd ?? 0) - (ordered[0][1].app.rssStart ?? 0) : 0
    const waits = {}
    for (const p of profiles) {
      for (const t of Object.values(p.network ?? {})) {
        if (!t?.container) continue
        waits[t.container] = (waits[t.container] ?? 0) + (t.waitMs ?? 0)
      }
    }
    const logs = profiles.reduce((a, p) => ({ errors: a.errors + (p.logs?.errors ?? 0), warnings: a.warnings + (p.logs?.warnings ?? 0) }), { errors: 0, warnings: 0 })
    return { cpuMs, peak, growth, waits: Object.entries(waits).filter(([, ms]) => ms > 0).sort((a, b) => b[1] - a[1]), logs }
  }, [profiles, run.scProfile])

  const parts = [
    { key: 'deps', label: 'dependencies', ms: depsMs },
    { key: 'app', label: 'app start', ms: appMs },
    { key: 'scenarios', label: 'scenarios', ms: scenariosMs },
  ].filter((p) => p.ms > 0)
  const partsTotal = parts.reduce((a, p) => a + p.ms, 0)

  return (
    <div className="rp">
      <div className="rp-hero">
        <span className="rp-verdict" data-status={status}>{verdict}</span>
        <span className="rp-sub">
          {runId ? `run ${runId}` : 'this run'} · {fmtDur(wall)} from start to finish
          {containers.length ? ` · ${plural(containers.length, 'container')}` : ''}
          {app?.name ? ` · ${app.name}${app.url ? ` at ${app.url}` : ''}` : ''}
        </span>
        {error && (
          <div className="rp-error">
            {error.message}
            {error.logs?.length > 0 && (
              <ul className="rp-logs">{error.logs.slice(-6).map((l, i) => <li key={i}>{l}</li>)}</ul>
            )}
          </div>
        )}
      </div>

      <div className="rp-tiles">
        <Tile
          label="scenarios" status={failed > 0 ? 'failed' : undefined}
          value={<><i className="pip" data-status={failed > 0 ? 'failed' : 'passed'} />{failed > 0 ? `${failed} failed` : `${passed} passed`}</>}
          note={failed > 0 ? `${passed} passed${undef ? `, ${undef} undefined` : ''} of ${total}` : `of ${total}${undef ? `, ${undef} undefined` : ''}`}
        />
        <Tile
          label="steps" status={steps.failed ? 'failed' : undefined}
          value={steps.failed ? `${steps.failed} failed` : `${steps.passed ?? 0} passed`}
          note={[steps.failed ? `${steps.passed ?? 0} passed` : '', steps.skipped ? `${steps.skipped} skipped` : '', steps.undefined ? `${steps.undefined} undefined` : ''].filter(Boolean).join(' · ') || undefined}
        />
        <Tile label="scenario time" value={fmtDur(scenariosMs)} note={names.length ? `${fmtDur(scenariosMs / names.length)} on average` : undefined} />
        <Tile label="startup" value={fmtDur(depsMs + appMs)} note={[depsMs ? `dependencies ${fmtDur(depsMs)}` : '', appMs ? `app ${fmtDur(appMs)}` : ''].filter(Boolean).join(' · ') || undefined} />
        {perf && <Tile label="app CPU time" value={fmtMs(perf.cpuMs)} note="across the scenarios, all cores" />}
        {perf && <Tile label="app peak memory" value={fmtBytes(perf.peak)} note={Math.abs(perf.growth) >= 1 << 20 ? `${perf.growth > 0 ? '+' : '−'}${fmtBytes(Math.abs(perf.growth))} over the run` : 'steady over the run'} status={perf.growth >= 32 << 20 ? 'failed' : undefined} />}
        {perf && perf.logs.errors > 0 && <Tile label="app logged" value={`${perf.logs.errors} errors`} note={perf.logs.warnings ? `${perf.logs.warnings} warnings` : undefined} status="failed" />}
      </div>

      {parts.length > 0 && (
        <div className="rp-section">
          <h3>Where the time went <small>{fmtDur(partsTotal)}</small></h3>
          <div className="rp-anatomy" role="img" aria-label={parts.map((p) => `${p.label} ${fmtDur(p.ms)}`).join(', ')}>
            {parts.map((p) => <span key={p.key} data-part={p.key} style={{ flex: `${p.ms} 0 0` }} title={`${p.label} · ${fmtDur(p.ms)}`} />)}
          </div>
          <div className="rp-anatomy-keys">
            {parts.map((p) => <span className="rp-key" data-part={p.key} key={p.key}>{p.label} · {fmtDur(p.ms)}</span>)}
          </div>
        </div>
      )}

      {scene.features.length > 0 && (
        <div className="rp-section">
          <h3>Features <small>{plural(scene.features.length, 'feature')} · {plural(names.length, 'scenario')}</small></h3>
          {scene.features.map((f) => {
            const scs = f.scenarios
            const ok = scs.filter((n) => st[n] === 'passed').length
            const ms = scs.reduce((a, n) => a + (run.scDur[n] ?? 0), 0)
            return (
              <div className="rp-feature" key={`${f.file}:${f.name}`}>
                <div className="rp-feature-head">
                  <span className="rp-feature-name" title={f.file}>{f.name}</span>
                  <div className="runbar">{scs.map((n) => <i className="runbar-seg" key={n} data-status={st[n] ?? ''} />)}</div>
                  <span className="rp-feature-meta">{ok}/{scs.length} passed · {fmtDur(ms)}</span>
                </div>
                {scs.map((n) => {
                  const fail = st[n] === 'failed' ? failedStep(run, features, n) : null
                  return (
                    <div className="rp-sc" key={n} data-status={st[n] ?? ''}>
                      <i className="pip" data-status={st[n] ?? ''} />
                      <span className="rp-sc-name" title={n}>{n}</span>
                      <span className="rp-sc-time">{fmtDur(run.scDur[n])}</span>
                      {fail && <span className="rp-sc-fail"><b>{fail.text}</b>{fail.error ? ` — ${firstLine(fail.error)}` : ''}</span>}
                    </div>
                  )
                })}
              </div>
            )
          })}
        </div>
      )}

      {slowest.length > 1 && (
        <div className="rp-section">
          <h3>Slowest scenarios</h3>
          <Bars rows={slowest} />
        </div>
      )}

      {(perf || run.samples?.length > 0) && (
        <div className="rp-section rp-perf">
          <h3>Performance <small>measured from outside the app, while the scenarios ran</small></h3>
          {perf?.waits.length > 0 && (
            <>
              <h4>Time the app spent waiting on its dependencies</h4>
              <Bars rows={perf.waits.map(([name, ms]) => ({ key: name, label: name, value: ms }))} unit={fmtMs} />
            </>
          )}
          {run.samples?.length > 0 && <ProfileTimeline runState={run} />}
        </div>
      )}
    </div>
  )
}
