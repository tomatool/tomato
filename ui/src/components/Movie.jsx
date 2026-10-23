import { useEffect, useMemo, useRef, useState } from 'react'
import { actsOf, beatsOf, dwellOf, foldScene } from '../lib/reel.js'
import { flowModel } from '../lib/flow.js'
import { pickleIndex, stepRun } from '../lib/run.js'
import { tokenizeStep } from '../lib/steps.jsx'
import { fmtClock, fmtDur, plural } from '../lib/format.js'
import { Topology } from './Topology.jsx'
import { Report } from './Report.jsx'

const EMPTY_MODEL = { lanes: [], msgs: [] }

// A film watched as it happens runs a little behind the run, each beat held
// for its moment; this far behind, it catches up instead.
const BACKLOG = 6
const CATCH_UP = 120

const firstLine = (s) => String(s ?? '').split('\n')[0]

// The feature and scenario on stage, from what the server parsed: by file
// when the runner named one, by name otherwise.
function onStage(scene, features) {
  const seen = scene.features.find((f) => f.name === scene.feature)
  const feature = features.find((f) => (seen?.file ? f.filePath === seen.file : false))
    ?? features.find((f) => f.name === scene.feature) ?? null
  const scenario = feature?.scenarios?.find((s) => s.name === scene.scenario) ?? null
  return { feature, scenario }
}

// What the nodes of the topology are doing, by key: the containers while
// they come up, and the app while it does.
function stageStatus(scene, topo) {
  if (scene.act !== 'start' && scene.act !== 'deps' && scene.act !== 'app') return null
  const status = {}
  for (const c of topo?.containers ?? []) status[`c:${c.name}`] = { state: 'waiting' }
  for (const c of scene.containers) status[`c:${c.name}`] = { state: c.status, ms: c.ms }
  if (scene.app) status.app = { state: scene.app.status, ms: scene.app.ms }
  else if (scene.act !== 'start') status.app = { state: 'waiting' }
  return status
}

// The numbered rows under the stage: the containers coming up, the app's
// steps, or the scenario's steps as they run.
function seqRows({ scene, run, feature, scenario, topo }) {
  if (scene.act === 'start' || scene.act === 'deps') {
    const names = [...new Set([...scene.containers.map((c) => c.name), ...(topo?.containers ?? []).map((c) => c.name)])]
    return {
      title: names.length ? 'dependencies' : 'no dependencies to start',
      rows: names.map((name, i) => {
        const c = scene.containers.find((x) => x.name === name)
        const status = !c ? 'queued' : c.status === 'starting' ? 'running' : c.status === 'ready' ? 'passed' : 'failed'
        const note = !c ? 'waiting' : c.status === 'starting' ? 'pulling and starting…' : c.status === 'ready' ? `ready in ${fmtDur(c.ms)}` : 'failed to start'
        return { key: name, n: i + 1, status, label: name, note, current: c?.status === 'starting' }
      }),
    }
  }
  if (scene.act === 'app') {
    const app = scene.app
    const name = app?.name ?? 'app'
    const up = app?.status === 'listening' || app?.status === 'ready'
    const failed = app?.status === 'failed' || scene.error?.phase === 'app'
    const start = {
      key: 'start', n: 1, label: `start ${name}`, current: app?.status === 'starting',
      status: failed && !up ? 'failed' : up ? 'passed' : app ? 'running' : 'queued',
      note: up ? `listening at ${app.url ?? ''}` : failed ? 'did not start' : app ? 'waiting for it to listen…' : 'waiting',
    }
    const health = {
      key: 'health', n: 2, label: 'health check', current: app?.status === 'listening',
      status: app?.status === 'ready' ? 'passed' : failed && up ? 'failed' : up ? 'running' : 'queued',
      note: app?.status === 'ready' ? `ready in ${fmtDur(app.ms)}` : failed && up ? 'failed' : up ? 'checking…' : '',
    }
    return { title: name, rows: [start, health] }
  }
  if (scene.act === 'scenarios' && scenario) {
    const bg = feature?.background ?? []
    return {
      title: scenario.name,
      rows: (scenario.steps ?? []).map((step, i) => {
        const index = pickleIndex(bg, i)
        const r = stepRun(run, scenario.name, index, step)
        return {
          key: index, n: i + 1, status: r.status || 'queued', text: step, current: scene.step?.index === index,
          note: r.status === 'running' ? '…' : r.dur != null ? fmtDur(r.dur) : '',
          error: r.status === 'failed' ? firstLine(r.error) : null,
        }
      }),
    }
  }
  return { title: scene.scenario ?? scene.feature ?? '', rows: [] }
}

// The narration under the stage.
function caption({ scene, feature, scenario, topo }) {
  const { act, containers, app, error } = scene
  if (error && (act === 'start' || act === 'deps' || act === 'app')) {
    const where = { containers: 'starting the dependencies', resources: 'initialising the resources', app: 'starting the app', config: 'reading tomato.yml' }[error.phase] ?? 'starting up'
    return { kicker: 'the run failed', title: `Failed while ${where}`, text: firstLine(error.message), status: 'failed' }
  }
  switch (act) {
    case 'start':
      return { kicker: 'tomato run', title: scene.runId ?? 'starting', text: 'Starting up: the dependencies first, then the app, then every scenario against it.' }
    case 'deps': {
      const ready = containers.filter((c) => c.status === 'ready').length
      const pending = containers.filter((c) => c.status === 'starting').map((c) => c.name)
      const total = Math.max(containers.length, topo?.containers?.length ?? 0)
      if (!total) return { kicker: 'act 1 · dependencies', title: 'Starting dependencies', text: 'Nothing to start: this suite has no containers.' }
      return {
        kicker: 'act 1 · dependencies',
        title: `Starting ${plural(total, 'container')}`,
        text: `${ready} of ${total} ready${pending.length ? ` · waiting for ${pending.join(', ')}` : ''}. Each one pulses until its wait_for check answers.`,
      }
    }
    case 'app': {
      const name = app?.name ?? 'the app'
      const texts = {
        starting: `tomato started ${name} with the dependencies' addresses in its environment and is waiting for it to listen.`,
        listening: `${name} is listening at ${app?.url ?? ''}; tomato gives it a moment to settle, then checks its health.`,
        ready: `${name} is ready at ${app?.url ?? ''}, ${fmtDur(app?.ms)} after it was started. The scenarios can begin.`,
        failed: `${name} did not come up.`,
      }
      return { kicker: 'act 2 · the app', title: app ? name : 'Starting the app', text: texts[app?.status] ?? 'Initialising the resources the app depends on.', status: app?.status === 'failed' ? 'failed' : undefined }
    }
    case 'scenarios': {
      if (scene.last === 'feature_started' || !scene.scenario) {
        const n = feature?.scenarios?.length
        return { kicker: 'act 3 · scenarios', title: scene.feature ?? 'Running scenarios', text: n ? `${plural(n, 'scenario')} in ${feature.filePath}.` : '' }
      }
      const kicker = `act 3 · ${scene.feature ?? 'scenarios'}`
      const st = scene.step
      if (!st) return { kicker, title: scene.scenario, text: 'Starting the scenario.' }
      const bg = feature?.background ?? []
      const step = st.index < bg.length ? bg[st.index] : scenario?.steps?.[st.index - bg.length]
      const who = step ? `${String(step.keyword).trim()} ${step.text}` : `step ${st.index + 1}`
      const text = st.status === 'running' ? `${who}`
        : st.status === 'passed' ? `${who} — passed in ${fmtDur(st.ms)}`
          : st.status === 'failed' ? `${who} — failed: ${firstLine(st.error)}`
            : `${who} — ${st.status}`
      return { kicker, title: scene.scenario, text, status: st.status === 'failed' ? 'failed' : undefined }
    }
    default:
      return { kicker: 'report', title: scene.runId ? `run ${scene.runId}` : 'the run', text: '' }
  }
}

function SeqRow({ row }) {
  return (
    <div className="reel-row" data-status={row.status} data-current={row.current ? 'true' : undefined}>
      <span className="reel-n">{row.n}</span>
      <i className="pip" data-status={row.status === 'queued' ? '' : row.status} />
      <span className="reel-label" title={row.text ? `${String(row.text.keyword).trim()} ${row.text.text}` : row.label}>
        {row.text ? <><span className="step-kw">{String(row.text.keyword).trim()}</span> {tokenizeStep(row.text.text)}</> : row.label}
      </span>
      <span className="reel-note">{row.note}</span>
      {row.error && <span className="reel-err">{row.error}</span>}
    </div>
  )
}

// The run as a film: the dependencies coming up on the topology, then the
// app, then each scenario's traffic over it step by step, and the report at
// the end. It follows the run as it happens, a beat behind so every moment
// is seen, and can be paused, stepped and played again once it is over.
// `startAt` is a beat to open on, paused, for a link to a moment of a run;
// `onStarted` is told once it has been used.
export function Movie({ reel, features, topo, running, onClose, startAt, onStarted }) {
  const { events, source } = reel
  const beats = useMemo(() => beatsOf(events), [events])
  const acts = useMemo(() => actsOf(events, beats), [events, beats])
  const last = beats.length - 1

  // shown is the beat on the stage; mode says who moves it: the run itself
  // (follow), the player (playing) or nobody (paused).
  const [shown, setShown] = useState(0)
  const [mode, setMode] = useState('follow')
  useEffect(() => {
    if (startAt != null && source === 'log') {
      setShown(startAt); setMode('paused'); onStarted?.()
      return
    }
    setShown(0); setMode(source === 'log' ? 'playing' : 'follow')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [reel.startedAt, source])

  useEffect(() => {
    if (mode === 'paused') return undefined
    if (shown >= last) {
      if (mode === 'playing' && last >= 0) setMode('paused')
      return undefined
    }
    const hold = mode === 'follow' && last - shown > BACKLOG ? CATCH_UP : dwellOf(events[beats[shown]])
    const t = setTimeout(() => setShown((n) => Math.min(last, n + 1)), hold)
    return () => clearTimeout(t)
  }, [mode, shown, last, events, beats])

  const at = Math.max(0, Math.min(shown, last))
  const upto = last < 0 ? 0 : beats[at] + 1
  const foldRef = useRef(null)
  const { scene, run } = useMemo(() => {
    const fold = foldScene(events, upto, foldRef.current)
    foldRef.current = fold
    return fold
  }, [events, upto])

  // A new act starts at the top of the pane, whatever was scrolled to.
  useEffect(() => { document.querySelector('.pane--doc')?.scrollTo({ top: 0 }) }, [scene.act])

  const seek = (i) => { setMode('paused'); setShown(Math.max(0, Math.min(last, i))) }
  const togglePlay = () => {
    if (mode === 'playing') setMode('paused')
    else { if (shown >= last) setShown(0); setMode('playing') }
  }
  const goLive = () => { setMode('follow'); setShown(last) }

  useEffect(() => {
    const onKey = (e) => {
      const t = e.target
      if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (e.key === 'ArrowRight') { e.preventDefault(); seek(at + 1) } else if (e.key === 'ArrowLeft') { e.preventDefault(); seek(at - 1) } else if (e.key === ' ') { e.preventDefault(); togglePlay() }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  const { feature, scenario } = onStage(scene, features)
  const model = useMemo(() => (scenario ? flowModel(scenario, run, feature?.background) : EMPTY_MODEL), [scenario, run, feature])
  const bgLen = feature?.background?.length ?? 0
  const stepCursor = scene.act === 'scenarios' && scene.step ? scene.step.index - bgLen : -1
  const status = stageStatus(scene, topo)
  const seq = seqRows({ scene, run, feature, scenario, topo })
  const cap = caption({ scene, feature, scenario, topo })
  const following = mode === 'follow' && source === 'live'
  const behind = following && running && shown < last

  if (!events.length) {
    return (
      <div className="empty">
        <h3>Nothing to watch yet</h3>
        <p>Press <span className="kbd">R</span> to run the suite: it plays here as it happens. Past runs can be watched from the Runs tab.</p>
      </div>
    )
  }

  const card = scene.act === 'start'
    ? { kicker: source === 'log' ? 'replaying run' : 'tomato run', title: scene.runId ?? '…', sub: 'the dependencies, the app, then every scenario' }
    : scene.last === 'feature_started' && feature
      ? { kicker: 'Feature', title: feature.name, sub: `${feature.filePath} · ${plural(feature.scenarios?.length ?? 0, 'scenario')}` }
      : scene.last === 'feature_started' ? { kicker: 'Feature', title: scene.feature, sub: '' } : null

  return (
    <div className="reel" data-act={scene.act}>
      <div className="reel-head">
        <span className="reel-live" data-on={following && running ? 'true' : undefined}>
          {following && running ? (behind ? 'live · catching up' : 'live') : running ? 'paused · run in progress' : source === 'log' ? 'replay' : 'finished'}
        </span>
        <span style={{ flex: 1 }} />
        <div className="reel-acts" role="tablist">
          {acts.map((a, i) => (
            <button
              className="reel-act" type="button" role="tab" key={a.key}
              aria-selected={scene.act === a.key ? 'true' : undefined}
              onClick={() => seek(a.beat)}
            >
              {i + 1} · {a.label}
            </button>
          ))}
        </div>
        {onClose && (
          <button className="btn btn--ghost btn--sm" type="button" title="Back to the feature  esc" onClick={onClose}>
            Close <span className="kbd">esc</span>
          </button>
        )}
      </div>

      {scene.act === 'report' ? (
        <Report scene={scene} run={run} features={features} />
      ) : (
        <>
          <div className="reel-stage">
            <Topology
              model={model} topo={topo} cursor={stepCursor}
              hot={null} pinned={null} onHot={() => {}} onPin={() => {}}
              status={status} everything packets fit
            />
            {card && (
              <div className="reel-card" key={`${scene.act}-${scene.feature}`}>
                <span className="reel-card-kicker">{card.kicker}</span>
                <h2>{card.title}</h2>
                {card.sub && <span className="reel-card-sub">{card.sub}</span>}
              </div>
            )}
          </div>

          <div className="reel-below">
            <div className="reel-seq">
              <div className="reel-seq-title">{seq.title}</div>
              {seq.rows.map((row) => <SeqRow row={row} key={row.key} />)}
              {!seq.rows.length && <span className="muted">—</span>}
            </div>
            <div className="reel-caption">
              <span className="reel-kicker">{cap.kicker}</span>
              <span className="reel-title">{cap.title}</span>
              <p className="reel-text" data-status={cap.status}>{cap.text}</p>
            </div>
          </div>
        </>
      )}

      <div className="reel-nav">
        <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Previous beat  ←" disabled={at <= 0} onClick={() => seek(at - 1)}>
          <i className="ico-step" data-dir="back" />
        </button>
        <button className="btn btn--sm btn--icon" type="button" title="Play / pause  space" aria-pressed={mode === 'playing' ? 'true' : undefined} onClick={togglePlay}>
          <i className={mode === 'playing' ? 'ico-pause' : 'ico-play'} />
        </button>
        <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Next beat  →" disabled={at >= last} onClick={() => seek(at + 1)}>
          <i className="ico-step" />
        </button>
        <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Play from the start" onClick={() => { setShown(0); setMode('playing') }}>
          <i className="ico-replay" />
        </button>
        <span className="reel-pos">{at + 1}/{last + 1}</span>
        <span className="reel-clock">{fmtClock(scene.at)}</span>
        <span style={{ flex: 1 }} />
        {source === 'live' && !following && (
          <button className="btn btn--sm" type="button" onClick={goLive}>{running ? 'Back to live' : 'Jump to the end'}</button>
        )}
      </div>
    </div>
  )
}
