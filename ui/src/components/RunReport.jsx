import { useEffect, useMemo, useState } from 'react'
import { useLocalEnum } from '../hooks/useLocalState'
import { fmtBytes, when } from '../lib/format'
import { RUN_EVENTS, emptyRun, reduceRun } from '../lib/run.js'
import { runIdOf, shotsOfRun } from '../lib/runs.js'
import { Screenshots } from './Screenshots.jsx'
import { ProfileTimeline } from './Profile.jsx'

const VIEWS = ['logs', 'shots', 'profile']
const VIEW_LABEL = { logs: 'Logs', shots: 'Screenshots', profile: 'Profile' }
const FORMATS = [
  { key: 'ctrf', label: 'CTRF JSON', title: 'Common Test Report Format, for the ctrf.io reporters and GitHub Actions' },
  { key: 'junit', label: 'JUnit XML', title: 'JUnit XML, for Jenkins, GitLab, CircleCI and Azure DevOps' },
]

// The inspector's Report tab: one run at a time — the one happening or the
// last, until another is picked — with its logs, its screenshots and its
// profile, and the run exported as a standard test report. A past run's
// profile is rebuilt from the events it recorded, the way the film replays it.
export function RunReport({ runs, liveRunId, runState, shots, onOpenShot, onOpenLog, onWatch, onRefreshShots }) {
  const [picked, setPicked] = useState(null)
  const [view, setView] = useLocalEnum('reportView', VIEWS, 'logs')
  const run = runs.find((r) => r.name === picked)
    ?? runs.find((r) => liveRunId && runIdOf(r.name) === liveRunId)
    ?? runs[0] ?? null
  const live = run && liveRunId && runIdOf(run.name) === liveRunId

  // A run that is not the one in the page is read back from its events.
  const [past, setPast] = useState({ name: null, state: null })
  useEffect(() => {
    if (!run || live || view !== 'profile' || past.name === run.name) return undefined
    let gone = false
    fetch(`/api/runs/${encodeURIComponent(run.name)}/events`).then((r) => (r.ok ? r.json() : [])).then((msgs) => {
      if (gone) return
      const state = (Array.isArray(msgs) ? msgs : []).reduce((s, m) => (RUN_EVENTS.has(m.type) ? reduceRun(s, m) : s), emptyRun())
      setPast({ name: run.name, state })
    }).catch(() => { if (!gone) setPast({ name: run.name, state: emptyRun() }) })
    return () => { gone = true }
  }, [run, live, view, past.name])

  useEffect(() => { if (view === 'shots') onRefreshShots?.() }, [view, run?.name, onRefreshShots])

  const state = live ? runState : past.name === run?.name ? past.state : null
  const runShots = useMemo(() => (run ? shotsOfRun(shots, run.name, live ? runState : null) : []), [shots, run, live, runState])

  if (!run) {
    return (
      <div className="empty">
        <h3>No runs yet</h3>
        <p>Runs are written to <code>.tomato/runs</code>. Press <span className="kbd">R</span> to start one.</p>
      </div>
    )
  }

  const base = `/api/runs/${encodeURIComponent(run.name)}/report`
  return (
    <div className="rr">
      <div className="rr-head">
        <label className="rr-pick">
          <span className="label">run</span>
          <select className="input" value={run.name} onChange={(e) => setPicked(e.target.value)}>
            {runs.map((r) => (
              <option key={r.name} value={r.name}>
                {runIdOf(r.name)} · {when(r.timestamp)}{liveRunId && runIdOf(r.name) === liveRunId ? ' · this session' : ''}
              </option>
            ))}
          </select>
        </label>
        <div className="rr-actions">
          <button className="btn btn--sm" type="button" title="Watch this run again as a film" onClick={() => onWatch(run.name)}>
            <i className="ico-play" />Watch
          </button>
          <span className="rr-export">
            <span className="label">export</span>
            {FORMATS.map((f) => (
              <a className="btn btn--sm btn--ghost" key={f.key} href={`${base}?format=${f.key}`} download title={f.title}>{f.label}</a>
            ))}
          </span>
        </div>
        <div className="seg" role="tablist">
          {VIEWS.map((v) => (
            <button className="seg-opt" type="button" role="tab" key={v} aria-pressed={view === v ? 'true' : undefined} onClick={() => setView(v)}>
              {VIEW_LABEL[v]}
              {v === 'logs' && <span className="count">{run.logs?.length ?? 0}</span>}
              {v === 'shots' && <span className="count">{runShots.length}</span>}
            </button>
          ))}
        </div>
      </div>

      {view === 'logs' ? (
        <div className="rr-logs">
          {(run.logs ?? []).map((l) => (
            <button className="rr-log" type="button" key={l.name} title={l.absPath} onClick={() => onOpenLog(run.name, l.name, l.absPath)}>
              <span className="rr-log-name">{l.name}</span>
              <span className="rr-log-size">{fmtBytes(l.size)}</span>
            </button>
          ))}
          {!run.logs?.length && <p className="muted rr-note">This run wrote no logs.</p>}
          <p className="muted rr-note">A log opens in the output pane below.</p>
        </div>
      ) : view === 'shots' ? (
        <Screenshots shots={runShots} runState={live ? runState : emptyRun()} onOpen={onOpenShot} />
      ) : state ? (
        <ProfileTimeline runState={state} />
      ) : (
        <p className="muted rr-note">Reading the run’s profile…</p>
      )}
    </div>
  )
}
