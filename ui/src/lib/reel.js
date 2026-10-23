// A run as a film. Every /ws message that changes what the stage shows is
// kept in order, stamped with the moment it arrived, so the run can be
// watched as it happens, scrubbed back and forth once it is over, and a past
// run played again from its log. A scene is the state of the run after a
// prefix of those events: the containers coming up, the app, which feature
// and scenario and step the run is on, and how it ended; its `run` is the
// same per-step state the cards use (lib/run.js), folded from the same
// events, so a scrub shows what was known at that moment and nothing later.
import { RUN_EVENTS, emptyRun, reduceRun } from './run.js'

export const REEL_EVENTS = new Set([
  'run_started', 'run_created', 'run_phase', 'container_status', 'app_status',
  'feature_started', 'feature_finished', 'scenario_running', 'scenario_passed', 'scenario_failed',
  'step_started', 'step_finished', 'run_error', 'run_summary', 'run_finished',
])

export const emptyReel = () => ({ events: [], startedAt: 0, source: 'live' })

// record keeps one live message, at the time since the run started. A run
// joined midway — the page opened while it ran — starts the reel from the
// first message seen.
export function record(reel, m, now = Date.now()) {
  if (!REEL_EVENTS.has(m.type)) return reel
  if (m.type === 'run_started' || !reel.startedAt) {
    return { events: [{ ...m, at: 0 }], startedAt: now, source: 'live' }
  }
  return { ...reel, events: reel.events.concat({ ...m, at: now - reel.startedAt }) }
}

// fromLog is the reel of a past run, from the messages its log replays
// (GET /api/runs/{run}/events). The log has no clock, so the moments are
// rebuilt from what it does know: how long each container, the app, and
// each step took.
export function fromLog(messages, now = Date.now()) {
  // Each container, the app and each scenario is placed from when it
  // started plus how long the log says it took; the steps run on inside
  // their scenario.
  let at = 0
  const started = {}
  const events = []
  for (const m of messages) {
    if (!REEL_EVENTS.has(m.type)) continue
    const took = m.durationMs ?? 0
    switch (m.type) {
      case 'container_status':
        if (m.status === 'starting') started[`c:${m.name}`] = at
        else at = (started[`c:${m.name}`] ?? at) + took
        break
      case 'app_status':
        if (m.status === 'starting') started.app = at
        else at = (started.app ?? at) + took
        break
      case 'scenario_running':
        started.scenario = at
        break
      case 'step_finished':
        at += took
        break
      case 'scenario_passed':
      case 'scenario_failed':
        at = Math.max(at, (started.scenario ?? at) + took)
        break
      default:
        break
    }
    events.push({ ...m, at })
  }
  // startedAt is when the replay was asked for: each one is a new film.
  return { events, startedAt: now, source: 'log' }
}

// reelRunId is the run's id once the runner has said it.
export const reelRunId = (reel) => reel.events.find((e) => e.type === 'run_created')?.runId ?? null

// The acts of the film, in order. A run reaches the ones its events show.
export const ACTS = [
  { key: 'deps', label: 'Dependencies' },
  { key: 'app', label: 'App' },
  { key: 'scenarios', label: 'Scenarios' },
  { key: 'report', label: 'Report' },
]

// nextAct is the act the run is in after one more event.
export function nextAct(act, e) {
  switch (e.type) {
    case 'run_phase':
      if (e.phase === 'containers') return 'deps'
      if (e.phase === 'app') return 'app'
      if (e.phase === 'tests') return 'scenarios'
      return act
    case 'feature_started':
    case 'scenario_running':
      return 'scenarios'
    case 'run_finished':
      return 'report'
    default:
      return act
  }
}

export const emptyScene = () => ({
  runId: null,
  phase: null,
  act: 'start',
  // The containers in the order they started, each {name, status, ms}.
  containers: [],
  // The app under test: {name, status, ms, url}; null until it starts.
  app: null,
  // The features in the order they ran, each with the scenarios seen so far.
  features: [],
  // Where the run is: the names of the feature and scenario, and the step
  // {index, status, ms, error} that is running or just finished.
  feature: null,
  scenario: null,
  step: null,
  // How it ended: a failure outside the scenarios, the runner's count, and
  // the final status.
  error: null,
  summary: null,
  finished: null,
  // The latest event's type and time, for what the stage says about it.
  last: null,
  at: 0,
})

// reduceScene applies one event to the scene.
export function reduceScene(s, e) {
  const base = { ...s, last: e.type, at: e.at ?? s.at, act: nextAct(s.act, e) }
  switch (e.type) {
    case 'run_started':
      return { ...emptyScene(), last: e.type, at: e.at ?? 0 }
    case 'run_created':
      return { ...base, runId: e.runId ?? s.runId }
    case 'run_phase':
      return { ...base, phase: e.phase }
    case 'container_status': {
      const next = { name: e.name, status: e.status, ms: e.durationMs ?? null }
      const i = s.containers.findIndex((c) => c.name === e.name)
      const containers = i < 0 ? [...s.containers, next] : s.containers.map((c, j) => (j === i ? next : c))
      return { ...base, containers }
    }
    case 'app_status':
      return { ...base, app: { name: e.name || s.app?.name || 'app', status: e.status, ms: e.durationMs ?? s.app?.ms ?? null, url: e.url || s.app?.url || null } }
    case 'feature_started':
      return {
        ...base,
        features: [...s.features, { name: e.feature, file: e.file, scenarios: [] }],
        feature: e.feature, scenario: null, step: null,
      }
    case 'scenario_running': {
      const featureName = e.feature || s.feature
      let features = s.features
      if (featureName && !features.some((f) => f.name === featureName)) {
        features = [...features, { name: featureName, file: e.file, scenarios: [] }]
      }
      features = features.map((f) => (
        f.name === featureName && !f.scenarios.includes(e.scenario) ? { ...f, scenarios: [...f.scenarios, e.scenario] } : f
      ))
      return { ...base, features, feature: featureName, scenario: e.scenario, step: null }
    }
    case 'step_started':
      return { ...base, step: { index: e.stepIndex, status: 'running' } }
    case 'step_finished':
      return { ...base, step: { index: e.stepIndex, status: e.status ?? '', ms: e.durationMs ?? null, error: e.output || null } }
    case 'run_error':
      return { ...base, error: { phase: e.phase, message: e.output, logs: e.logs ?? [] } }
    case 'run_summary':
      return { ...base, summary: { total: e.total ?? 0, passed: e.passed ?? 0, failed: e.failed ?? 0, skipped: e.skipped ?? 0 } }
    case 'run_finished':
      return { ...base, finished: { status: e.status || (s.error ? 'failed' : 'passed'), at: e.at ?? s.at } }
    default:
      return base
  }
}

// foldScene is the scene and run state after the first `upto` events. With
// the previous fold it goes on from there when the events are the same ones
// with more on the end, which is every tick of a run being watched; only a
// scrub backwards folds from the start.
export function foldScene(events, upto, prev) {
  let i = 0
  let scene = emptyScene()
  let run = emptyRun()
  const end = Math.min(upto, events.length)
  if (prev && prev.upto <= end && prev.upto > 0 && events[prev.upto - 1] === prev.events[prev.upto - 1]) {
    i = prev.upto; scene = prev.scene; run = prev.run
  }
  for (; i < end; i += 1) {
    const e = events[i]
    scene = reduceScene(scene, e)
    if (RUN_EVENTS.has(e.type)) run = reduceRun(run, e)
  }
  return { events, upto: end, scene, run }
}

// The beats of the film: the events worth a moment of their own. The rest —
// the run id arriving, a feature closing, the count — ride along with them.
const NOT_A_BEAT = new Set(['run_created', 'feature_finished', 'run_summary'])
export const beatsOf = (events) => events.flatMap((e, i) => (NOT_A_BEAT.has(e.type) ? [] : [i]))

// How long a beat is held when the film plays itself: a title longer than a
// step, a failure longer than a pass.
const DWELL = {
  run_started: 1400, run_phase: 500, container_status: 600, app_status: 900,
  feature_started: 1800, scenario_running: 1100, step_started: 300, step_finished: 650,
  scenario_passed: 450, scenario_failed: 1300, run_error: 1500, run_finished: 0,
}
export const dwellOf = (e) => DWELL[e?.type] ?? 500

// actsOf is which acts the events reach and the beat each one opens on.
export function actsOf(events, beats) {
  const found = []
  let act = 'start'
  let b = 0
  events.forEach((e, i) => {
    const next = nextAct(act, e)
    while (b < beats.length && beats[b] < i) b += 1
    if (next !== act && !found.some((a) => a.key === next)) {
      // The act opens on this event's beat, or the next one when the event
      // itself is not a beat.
      found.push({ key: next, beat: Math.min(b, beats.length - 1) })
    }
    act = next
  })
  return ACTS.flatMap((a) => {
    const hit = found.find((f) => f.key === a.key)
    return hit ? [{ ...a, beat: hit.beat }] : []
  })
}
