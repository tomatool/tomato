// Per-step run state, keyed the way the runner reports it.
//
// The runner numbers a step by its position in the godog pickle: a
// Background's steps come first, and the scenario's own steps start after
// them (internal/formatter). Every status, duration and error from the /ws
// feed is stored under that one index, so a row that wants its state has to
// say where it sits in the pickle — the scenario's own row i is pickle step
// background.length + i, and Background row i is pickle step i of whichever
// scenario ran it. Every view goes through here; a second numbering shifted
// every status by the Background's length and put the failed step on no row
// at all (#212).

export const stepKey = (scenario, i) => `${scenario}\u0000${i}`
// splitKey reads a stepKey back: [scenario, index].
export function splitKey(k) {
  const i = String(k).lastIndexOf('\u0000')
  return [String(k).slice(0, i), Number(String(k).slice(i + 1))]
}

// scOrder is the scenarios of this run in the order they started, so the
// feature's Background card can show the run that is happening or just did.
// stepShots are the screenshots a step took, [{resource, path}] under its key:
// what the runner recorded on the step's end, so a picture is matched to its
// step by path and never guessed from the file's name (lib/screenshots.js).
export const emptyRun = () => ({
  scStatus: {}, scDur: {}, stepStatus: {}, stepDur: {}, stepError: {}, stepShots: {}, stepMismatch: {}, scOrder: [],
})

// The /ws message types that change the run state. Everything else — the
// feature list, output lines, run ids — is the App's business.
export const RUN_EVENTS = new Set([
  'run_started', 'scenario_running', 'scenario_passed', 'scenario_failed',
  'step_started', 'step_finished', 'run_finished', 'run_error',
])

// reduceRun applies one /ws message to the run state and returns the next
// state; a message that changes nothing returns the same object.
export function reduceRun(state, m) {
  switch (m.type) {
    case 'run_started':
      return emptyRun()
    case 'scenario_running': {
      if (m.scenario == null) return state
      const scOrder = [...(state.scOrder ?? []).filter((n) => n !== m.scenario), m.scenario]
      return { ...state, scStatus: { ...state.scStatus, [m.scenario]: 'running' }, scOrder }
    }
    case 'scenario_passed':
    case 'scenario_failed': {
      if (m.scenario == null) return state
      const status = m.type === 'scenario_passed' ? 'passed' : 'failed'
      return {
        ...state,
        scStatus: { ...state.scStatus, [m.scenario]: status },
        scDur: m.durationMs == null ? state.scDur : { ...state.scDur, [m.scenario]: m.durationMs },
      }
    }
    case 'step_started': {
      if (m.scenario == null || m.stepIndex == null) return state
      return { ...state, stepStatus: { ...state.stepStatus, [stepKey(m.scenario, m.stepIndex)]: 'running' } }
    }
    case 'step_finished': {
      if (m.scenario == null || m.stepIndex == null) return state
      const k = stepKey(m.scenario, m.stepIndex)
      return {
        ...state,
        stepStatus: { ...state.stepStatus, [k]: m.status ?? '' },
        stepDur: m.durationMs == null ? state.stepDur : { ...state.stepDur, [k]: m.durationMs },
        stepError: m.output ? { ...state.stepError, [k]: m.output } : state.stepError,
        stepShots: m.stepScreenshots?.length ? { ...state.stepShots, [k]: m.stepScreenshots } : state.stepShots,
        // Both sides of a failed comparison, when the runner sent them
        // (internal/formatter); components/Mismatch.jsx draws the diff.
        stepMismatch: m.mismatch ? { ...state.stepMismatch, [k]: m.mismatch } : state.stepMismatch,
      }
    }
    case 'run_finished':
    case 'run_error': {
      // Anything still queued or running never reported; clear it rather
      // than leave a spinner behind.
      const scStatus = { ...state.scStatus }
      let changed = false
      for (const [k, v] of Object.entries(scStatus)) {
        if (v === 'queued' || v === 'running') { delete scStatus[k]; changed = true }
      }
      return changed ? { ...state, scStatus } : state
    }
    default:
      return state
  }
}

// pickleIndex is where the scenario's own step i sits in the pickle: after
// the Background, when the feature has one.
export const pickleIndex = (background, i) => (background?.length ?? 0) + i

// stepRun is what the run reported for pickle step `index` of `scenario`.
// `unmatched` is known from parsing alone, so a step with no definition
// reads as undefined before the suite has ever run.
export function stepRun(runState, scenario, index, step) {
  const fallback = step?.unmatched ? 'undefined' : ''
  if (scenario == null || index == null) return { status: fallback, dur: undefined, error: undefined, screenshots: undefined, mismatch: undefined }
  const k = stepKey(scenario, index)
  return {
    status: runState.stepStatus[k] ?? fallback,
    dur: runState.stepDur[k],
    error: runState.stepError[k],
    screenshots: runState.stepShots?.[k],
    mismatch: runState.stepMismatch?.[k],
  }
}

// backgroundScenario is whose run the feature's Background card shows: the
// scenario of this feature that is running, else the last one that started.
// null before any of them has run.
export function backgroundScenario(feature, runState) {
  const names = new Set((feature?.scenarios ?? []).map((s) => s.name))
  const order = (runState.scOrder ?? []).filter((n) => names.has(n))
  if (!order.length) return null
  const running = order.filter((n) => runState.scStatus[n] === 'running')
  return running.length ? running[running.length - 1] : order[order.length - 1]
}

// backgroundRows is the Background, row by row, as it ran for one scenario.
export function backgroundRows(background, scenario, runState) {
  return (background ?? []).map((step, i) => ({ step, index: i, ...stepRun(runState, scenario, i, step) }))
}

// A step with one of these statuses stopped the scenario; the rest of the
// pickle is skipped after it.
export const STOPPING = new Set(['failed', 'undefined', 'ambiguous'])

// failedBackgroundSteps are the Background steps that stopped this scenario
// before its own steps ran — what its card has to show, since the Background
// card can only show one scenario's run at a time.
export function failedBackgroundSteps(background, scenario, runState) {
  return backgroundRows(background, scenario, runState).filter((r) => STOPPING.has(r.status))
}

// backgroundStatus sums up the Background's run for one scenario, for the
// card's pip and border: failed, running, passed, or '' when it has not
// reported.
export function backgroundStatus(background, scenario, runState) {
  const rows = backgroundRows(background, scenario, runState)
  if (!rows.length || scenario == null) return ''
  if (rows.some((r) => STOPPING.has(r.status))) return 'failed'
  if (rows.some((r) => r.status === 'running')) return 'running'
  if (rows.every((r) => r.status === 'passed')) return 'passed'
  return ''
}
