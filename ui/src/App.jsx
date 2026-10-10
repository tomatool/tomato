import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useSocket } from './hooks/useSocket'
import { useLocalNumber } from './hooks/useLocalState'
import { flowModel } from './lib/flow'
import { stepKey, BACKGROUND_KEY } from './lib/steps.jsx'
import { TopBar } from './components/TopBar.jsx'
import { Tree } from './components/Tree.jsx'
import { ScenarioCard, BackgroundCard } from './components/ScenarioCard.jsx'
import { ConfigView } from './components/ConfigView.jsx'
import { Flow } from './components/Flow.jsx'
import { Topology } from './components/Topology.jsx'
import { Runs } from './components/Runs.jsx'
import { Transport } from './components/Transport.jsx'
import { Console } from './components/Console.jsx'
import { Peek } from './components/Peek.jsx'

const SIDE_MIN = 240
const SIDE_DEFAULT = 340
const CONSOLE_MIN = 80

const emptyRun = () => ({ scStatus: {}, scDur: {}, stepStatus: {}, stepDur: {}, stepError: {} })

export function App() {
  // server data
  const [features, setFeatures] = useState([])
  const [runs, setRuns] = useState([])
  const [topo, setTopo] = useState(null)
  const [config, setConfig] = useState(null)
  const [changed, setChanged] = useState(() => new Set())

  // run state
  const [runState, setRunState] = useState(emptyRun)
  const [running, setRunning] = useState(false)
  const [runId, setRunId] = useState(null)
  const [lines, setLines] = useState([])
  const runStartedAt = useRef(0)

  // view state
  const [view, setView] = useState('feature')
  const [selectedFile, setSelectedFile] = useState(null)
  const [selectedScenario, setSelectedScenario] = useState(0)
  const [filter, setFilter] = useState('')
  const [cfgMode, setCfgMode] = useState('structured')
  const [sideTab, setSideTab] = useState('flow')
  const [openDirs, setOpenDirs] = useState({})
  const [collapsed, setCollapsed] = useState({})
  const [treeOpen, setTreeOpen] = useState(true)
  const [sideOpen, setSideOpen] = useState(true)
  const [openRun, setOpenRun] = useState(null)
  const [logTabs, setLogTabs] = useState([])
  const [activeLog, setActiveLog] = useState(null)
  const [hot, setHot] = useState(null)
  const [pinned, setPinned] = useState(null)
  const [cursor, setCursor] = useState(-1)
  const [playing, setPlaying] = useState(false)
  const playTimer = useRef(null)

  const [sideWidth, setSideWidth] = useLocalNumber('sideWidth', SIDE_DEFAULT)
  const [consoleHeight, setConsoleHeight] = useLocalNumber('consoleHeight', 220)

  // ——— server messages ———
  const onMessage = useCallback((m) => {
    switch (m.type) {
      case 'init':
      case 'update': {
        setFeatures(m.features ?? [])
        if (m.runs) setRuns(m.runs)
        if (m.topology) setTopo(m.topology)
        if (m.changedFiles?.length) {
          setChanged((prev) => new Set([...prev, ...m.changedFiles]))
          for (const f of m.changedFiles) {
            setOpenDirs((prev) => {
              const next = { ...prev }
              const parts = f.split('/')
              for (let i = 0; i < parts.length - 1; i += 1) next[parts.slice(0, i + 1).join('/')] = true
              return next
            })
            setTimeout(() => setChanged((prev) => {
              const next = new Set(prev); next.delete(f); return next
            }), 3000)
          }
        }
        break
      }
      case 'run_started':
        runStartedAt.current = Date.now()
        setRunning(true)
        setRunState(emptyRun())
        setLines([])
        break
      case 'scenario_running':
        setRunState((p) => ({ ...p, scStatus: { ...p.scStatus, [m.scenario]: 'running' } }))
        break
      case 'scenario_passed':
      case 'scenario_failed': {
        const status = m.type === 'scenario_passed' ? 'passed' : 'failed'
        setRunState((p) => ({
          ...p,
          scStatus: { ...p.scStatus, [m.scenario]: status },
          scDur: m.durationMs == null ? p.scDur : { ...p.scDur, [m.scenario]: m.durationMs },
        }))
        break
      }
      case 'step_started':
        if (m.scenario != null && m.stepIndex != null) {
          setRunState((p) => ({
            ...p, stepStatus: { ...p.stepStatus, [stepKey(m.scenario, m.stepIndex)]: 'running' },
          }))
        }
        break
      case 'step_finished':
        if (m.scenario != null && m.stepIndex != null) {
          const k = stepKey(m.scenario, m.stepIndex)
          setRunState((p) => ({
            ...p,
            stepStatus: { ...p.stepStatus, [k]: m.status ?? '' },
            stepDur: m.durationMs == null ? p.stepDur : { ...p.stepDur, [k]: m.durationMs },
            stepError: m.output ? { ...p.stepError, [k]: m.output } : p.stepError,
          }))
        }
        break
      case 'run_output':
        setLines((p) => {
          const next = [...p, { text: m.output ?? '', status: m.status, at: Date.now() - runStartedAt.current }]
          return next.length > 4000 ? next.slice(-4000) : next
        })
        break
      case 'run_finished':
      case 'run_error':
        setRunning(false)
        // Anything still queued or running never reported; clear it rather
        // than leave a spinner behind.
        setRunState((p) => {
          const scStatus = { ...p.scStatus }
          for (const [k, v] of Object.entries(scStatus)) {
            if (v === 'queued' || v === 'running') delete scStatus[k]
          }
          return { ...p, scStatus }
        })
        break
      case 'runs_update':
        setRuns(m.runs ?? [])
        break
      case 'run_created':
        setRunId(m.runId)
        break
      default:
        break
    }
  }, [])

  const socket = useSocket(onMessage)

  useEffect(() => {
    fetch('/api/config').then((r) => r.json()).then(setConfig)
      .catch((e) => setConfig({ error: String(e) }))
  }, [])

  // ——— derived ———
  const visibleScenarios = useCallback((f) => {
    const list = f.scenarios ?? []
    if (!filter) return list
    const q = filter.toLowerCase()
    if (q.startsWith('@')) {
      return list.filter((s) => (s.tags ?? []).some((t) => t.toLowerCase().startsWith(q)))
    }
    return list.filter((s) => s.name.toLowerCase().includes(q)
      || (s.steps ?? []).some((st) => st.text.toLowerCase().includes(q)))
  }, [filter])

  const matchingFeatures = useMemo(() => features.filter((f) => {
    if (!filter) return true
    const q = filter.toLowerCase()
    return f.name.toLowerCase().includes(q)
      || (f.tags ?? []).some((t) => t.toLowerCase().includes(q))
      || visibleScenarios(f).length > 0
  }), [features, filter, visibleScenarios])

  const allScenarios = useMemo(
    () => features.flatMap((f) => f.scenarios ?? []), [features],
  )

  const featureStatus = useCallback((f) => {
    let any = false; let all = true; let failed = false; let undef = false; let live = false
    for (const s of f.scenarios ?? []) {
      const st = runState.scStatus[s.name]
      if (!st) { all = false; continue }
      any = true
      if (st === 'failed') failed = true
      else if (st === 'undefined') undef = true
      else if (st === 'running' || st === 'queued') live = true
    }
    if (live) return 'running'
    if (failed) return 'failed'
    if (undef) return 'undefined'
    return any && all ? 'passed' : ''
  }, [runState.scStatus])

  const feature = features.find((f) => f.filePath === selectedFile) ?? null
  const shown = feature ? visibleScenarios(feature) : []
  const scenario = shown[selectedScenario] ?? shown[0] ?? null
  const model = useMemo(() => flowModel(scenario, runState), [scenario, runState])

  // The pointer is not over anything in a view you just switched away from,
  // so a hover left highlighted there would be a lie.
  useEffect(() => { setHot(null) }, [sideTab, scenario?.name])

  // Pick the first feature once something arrives.
  useEffect(() => {
    if (!selectedFile && features.length) setSelectedFile(features[0].filePath)
  }, [features, selectedFile])

  // ——— actions ———
  const post = (url) => fetch(url, { method: 'POST' }).catch(() => {})
  const runAll = useCallback(() => { if (!running) post('/api/run') }, [running])
  const runScenario = useCallback((name) => {
    if (!running) post(`/api/run?scenario=${encodeURIComponent(name)}`)
  }, [running])
  const stopRun = useCallback(() => { if (running) post('/api/stop') }, [running])
  const runFailed = useCallback(() => {
    const names = Object.entries(runState.scStatus)
      .filter(([, v]) => v === 'failed').map(([k]) => k)
    if (names.length === 1) runScenario(names[0])
    else if (names.length) runAll()
  }, [runState.scStatus, runAll, runScenario])

  const toggleCard = useCallback((key, isCollapsed) => {
    setCollapsed((p) => ({ ...p, [key]: !isCollapsed }))
  }, [])

  const togglePin = useCallback((i) => {
    setPinned((p) => (p === i ? null : i))
  }, [])

  const seek = useCallback((i) => {
    clearInterval(playTimer.current); playTimer.current = null; setPlaying(false)
    const last = (model?.msgs.length ?? 0) - 1
    setCursor(Math.max(-1, Math.min(last, i)))
  }, [model])

  const play = useCallback((from) => {
    clearInterval(playTimer.current)
    const msgs = model?.msgs ?? []
    if (!msgs.length) return
    const last = msgs.length - 1
    let i = from ?? (cursor >= last ? -1 : cursor)
    setPlaying(true)
    const tick = () => {
      i += 1
      setCursor(Math.min(last, i))
      if (i >= last) { clearInterval(playTimer.current); playTimer.current = null; setPlaying(false) }
    }
    tick()
    if (i < last) playTimer.current = setInterval(tick, 1400)
  }, [model, cursor])

  const togglePlay = useCallback(() => {
    if (playTimer.current) {
      clearInterval(playTimer.current); playTimer.current = null; setPlaying(false)
    } else play()
  }, [play])

  useEffect(() => () => clearInterval(playTimer.current), [])

  const openLog = useCallback((run, name) => {
    setLogTabs((p) => (p.some((t) => t.run === run && t.name === name)
      ? p : [...p, { run, name }].slice(-6)))
    setActiveLog({ run, name })
    fetch(`/api/runs/${encodeURIComponent(run)}/logs/${encodeURIComponent(name)}`)
      .then((r) => r.text())
      .then((txt) => setLines(txt.split('\n').map((text) => ({ text, at: null }))))
      .catch((e) => setLines([{ text: String(e), status: 'failed', at: null }]))
  }, [])

  const moveScenario = useCallback((d) => {
    if (!shown.length) return
    setSelectedScenario((i) => Math.max(0, Math.min(shown.length - 1, i + d)))
    setCursor(-1)
    setPinned(null)
  }, [shown.length])

  // ——— keyboard ———
  useEffect(() => {
    const onKey = (e) => {
      const t = e.target
      const typing = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)

      if (e.key === 'Escape') {
        if (typing) t.blur()
        if (pinned != null) setPinned(null)
        else if (filter) setFilter('')
        else if (running) stopRun()
        return
      }
      if (typing || e.metaKey || e.ctrlKey || e.altKey) return

      switch (e.key) {
        case '/': e.preventDefault(); document.getElementById('filter')?.focus(); break
        case 'j': moveScenario(1); break
        case 'k': moveScenario(-1); break
        case 'r': if (scenario) runScenario(scenario.name); break
        case 'R': runAll(); break
        case '[': setTreeOpen((v) => !v); break
        case ']': setSideOpen((v) => !v); break
        case 'ArrowRight': if (sideTab !== 'runs') { e.preventDefault(); seek(cursor + 1) } break
        case 'ArrowLeft': if (sideTab !== 'runs') { e.preventDefault(); seek(cursor - 1) } break
        case ' ': if (sideTab !== 'runs') { e.preventDefault(); togglePlay() } break
        default: break
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [filter, running, stopRun, moveScenario, scenario, runScenario, runAll, sideTab, cursor, seek, togglePlay, pinned])

  // ——— pane dragging ———
  const [resizing, setResizing] = useState(null)
  useEffect(() => {
    if (!resizing) return undefined
    const onMove = (e) => {
      if (resizing.kind === 'side') {
        // The handle is on the pane's leading edge, so moving left widens it.
        const max = Math.max(SIDE_MIN, window.innerWidth - 420)
        setSideWidth(Math.round(Math.min(Math.max(resizing.start - (e.clientX - resizing.at), SIDE_MIN), max)))
      } else {
        const max = window.innerHeight * 0.7
        setConsoleHeight(Math.round(Math.min(Math.max(resizing.start + (resizing.at - e.clientY), CONSOLE_MIN), max)))
      }
    }
    const onUp = () => setResizing(null)
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    return () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
    }
  }, [resizing, setSideWidth, setConsoleHeight])

  const crumb = view === 'config'
    ? <b>{config?.path ?? 'tomato.yml'}</b>
    : feature
      ? (() => {
        const parts = feature.filePath.split('/')
        const base = parts.pop()
        return <>{parts.join('/')}{parts.length ? '/' : ''}<b>{base}</b></>
      })()
      : ''

  return (
    <div
      className="app"
      data-tree={treeOpen ? 'open' : 'closed'}
      data-side={sideOpen ? 'open' : 'closed'}
      data-resizing={resizing ? 'true' : undefined}
      style={{ '--side-open-w': `${sideWidth}px` }}
    >
      <TopBar
        crumb={crumb}
        live={socket === 'online' ? '' : socket}
        liveText={socket === 'online'
          ? `watching ${config?.watching ?? './features'}`
          : socket === 'error' ? 'connection error' : 'disconnected'}
        scenarios={allScenarios}
        scStatus={runState.scStatus}
        scDur={runState.scDur}
        running={running}
        onRunAll={runAll}
        onRunFailed={runFailed}
        onStop={stopRun}
      />

      <aside className="pane pane--tree">
        <div className="rail">
          <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Show features  [" onClick={() => setTreeOpen(true)}>
            <i className="ico-chev" />
          </button>
          <span className="rail-label">Features</span>
        </div>
        <div className="pane-head">
          <span className="label">Features</span>
          <span className="count">{features.length}</span>
          <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Hide features  [" onClick={() => setTreeOpen(false)}>
            <i className="ico-chev" data-dir="left" />
          </button>
        </div>
        <div style={{ padding: '8px 12px 4px' }}>
          <label className="search">
            <input
              className="input" id="filter" placeholder="Filter or @tag"
              autoComplete="off" spellCheck="false"
              value={filter} onChange={(e) => setFilter(e.target.value)}
            />
            <span className="kbd">/</span>
          </label>
        </div>
        <div className="pane-body">
          <Tree
            features={matchingFeatures}
            config={config}
            view={view}
            selectedFile={selectedFile}
            selectedScenario={selectedScenario}
            scStatus={runState.scStatus}
            changed={changed}
            openDirs={openDirs}
            featureStatus={featureStatus}
            visibleScenarios={visibleScenarios}
            onToggleDir={(d) => setOpenDirs((p) => ({ ...p, [d]: p[d] === false }))}
            onSelectConfig={() => { setView('config'); setActiveLog(null) }}
            onSelectFile={(p) => { setView('feature'); setSelectedFile(p); setSelectedScenario(0); setCursor(-1); setPinned(null) }}
            onSelectScenario={(p, i) => { setView('feature'); setSelectedFile(p); setSelectedScenario(i); setCursor(-1); setPinned(null) }}
          />
        </div>
      </aside>

      <main className="pane pane--doc">
        {view === 'config' ? (
          <ConfigView
            config={config} mode={cfgMode} onMode={setCfgMode}
            featureCount={features.length} scenarioCount={allScenarios.length}
          />
        ) : !feature ? (
          <div className="empty">
            <h3>No feature selected</h3>
            <p>
              Pick a <code>.feature</code> file on the left, or press <span className="kbd">/</span> to
              filter. <span className="kbd">R</span> runs everything.
            </p>
          </div>
        ) : (
          <div className="doc">
            <div className="feature-head">
              <span className="feature-kw">FEATURE</span>
              <h1>{feature.name}</h1>
              <span className="feature-path">{feature.filePath}</span>
              {feature.description && <p className="feature-desc">{feature.description}</p>}
              {feature.tags?.length > 0 && (
                <div className="row">
                  {feature.tags.map((t) => {
                    const name = t.replace(/^@/, '')
                    return (
                      <button
                        className="tag" type="button" key={t}
                        aria-pressed={filter.toLowerCase() === `@${name.toLowerCase()}` ? 'true' : undefined}
                        onClick={() => setFilter((f) => (f.toLowerCase() === `@${name.toLowerCase()}` ? '' : `@${name}`))}
                      >
                        {name}
                      </button>
                    )
                  })}
                </div>
              )}
            </div>

            {feature.background?.length > 0 && (() => {
              const isCollapsed = collapsed[BACKGROUND_KEY] ?? true
              return (
                <BackgroundCard
                  steps={feature.background}
                  collapsed={isCollapsed}
                  onToggle={() => toggleCard(BACKGROUND_KEY, isCollapsed)}
                  runState={runState}
                />
              )
            })()}

            {shown.length === 0 && (
              <div className="empty">
                <h3>Nothing matches “{filter}”</h3>
                <p>Press <span className="kbd">esc</span> to clear the filter.</p>
              </div>
            )}

            {shown.map((s, i) => {
              const isCollapsed = collapsed[s.name] ?? runState.scStatus[s.name] === 'passed'
              return (
                <ScenarioCard
                  key={s.name}
                  scenario={s}
                  index={i}
                  current={selectedScenario === i}
                  collapsed={isCollapsed}
                  onToggle={() => toggleCard(s.name, isCollapsed)}
                  onRun={runScenario}
                  onTag={(name) => setFilter((f) => (f.toLowerCase() === `@${name.toLowerCase()}` ? '' : `@${name}`))}
                  filter={filter}
                  runState={runState}
                />
              )
            })}
          </div>
        )}
      </main>

      <aside className="pane pane--side">
        <div
          className="resize-x"
          title="Drag to resize · double-click to reset"
          onMouseDown={(e) => {
            if (!sideOpen) return
            e.preventDefault()
            setResizing({ kind: 'side', at: e.clientX, start: sideWidth })
          }}
          onDoubleClick={() => setSideWidth(SIDE_DEFAULT)}
        />
        <div className="rail">
          <button className="btn btn--ghost btn--sm btn--icon" type="button" title="Show inspector  ]" onClick={() => setSideOpen(true)}>
            <i className="ico-chev" data-dir="left" />
          </button>
          {['flow', 'topology', 'runs'].map((t) => (
            <button
              className="rail-tab" type="button" key={t}
              aria-selected={sideTab === t ? 'true' : undefined}
              onClick={() => { setSideTab(t); setSideOpen(true) }}
            >
              {t[0].toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>

        <div className="tabs" role="tablist">
          <button className="tab" type="button" role="tab" aria-selected={sideTab === 'flow' ? 'true' : undefined} onClick={() => setSideTab('flow')}>
            Flow <span className="count">{model?.msgs.length ?? 0}</span>
          </button>
          <button className="tab" type="button" role="tab" aria-selected={sideTab === 'topology' ? 'true' : undefined} onClick={() => setSideTab('topology')}>
            Topology
          </button>
          <button className="tab" type="button" role="tab" aria-selected={sideTab === 'runs' ? 'true' : undefined} onClick={() => setSideTab('runs')}>
            Runs <span className="count">{runs.length}</span>
          </button>
          <span style={{ flex: 1 }} />
          <button className="btn btn--ghost btn--sm btn--icon" type="button" style={{ alignSelf: 'center' }} title="Hide inspector  ]" onClick={() => setSideOpen(false)}>
            <i className="ico-chev" />
          </button>
        </div>

        {sideTab !== 'runs' && model && (
          <Transport
            msgs={model.msgs} cursor={cursor} playing={playing}
            onSeek={seek} onPlay={togglePlay} onReplay={() => play(-1)}
          />
        )}

        <div className="pane-body">
          {sideTab === 'runs' ? (
            <Runs runs={runs} openRun={openRun} onOpenRun={setOpenRun} onOpenLog={openLog} />
          ) : !model ? (
            <div className="empty">
              <h3>No scenario selected</h3>
              <p>Pick a scenario to see what it talks to.</p>
            </div>
          ) : sideTab === 'topology' ? (
            <Topology
              model={model} topo={topo} cursor={cursor}
              hot={hot} pinned={pinned} onHot={setHot} onPin={togglePin}
            />
          ) : (
            <Flow
              model={model} cursor={cursor}
              hot={hot} pinned={pinned} onHot={setHot} onPin={togglePin}
            />
          )}
        </div>

        {sideTab !== 'runs' && model && (() => {
          // A pin wins over the pointer — that is what pinning is for. With
          // neither, the dock follows the transport.
          const shownMsg = pinned != null ? model.msgs[pinned]
            : hot != null ? model.msgs[hot]
              : cursor >= 0 ? model.msgs[cursor] : null
          return (
            <div className="peek-dock" data-pinned={pinned != null ? 'true' : undefined}>
              <div className="peek-dock-head">
                <span className="label">{pinned != null ? 'pinned' : 'detail'}</span>
                {pinned != null && (
                  <button
                    className="btn btn--ghost btn--sm"
                    type="button"
                    onClick={() => setPinned(null)}
                  >
                    Unpin <span className="kbd">esc</span>
                  </button>
                )}
              </div>
              {shownMsg ? (
                <div className="peek-dock-body"><Peek msg={shownMsg} /></div>
              ) : (
                <div className="peek-dock-hint">
                  Hover a line for the step behind it. Click to keep it here.
                </div>
              )}
            </div>
          )
        })()}
      </aside>

      <Console
        lines={lines}
        logTabs={logTabs}
        activeLog={activeLog}
        runId={runId}
        running={running}
        height={consoleHeight}
        onTab={(t) => { if (!t) { setActiveLog(null); setLines([]) } else openLog(t.run, t.name) }}
        onResizeStart={(e) => {
          e.preventDefault()
          setResizing({ kind: 'console', at: e.clientY, start: consoleHeight })
        }}
      />
    </div>
  )
}
