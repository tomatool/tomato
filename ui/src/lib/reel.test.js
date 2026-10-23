import { describe, expect, it } from 'vitest'
import {
  actsOf, beatsOf, dwellOf, emptyReel, foldScene, fromLog, record, reelRunId,
} from './reel.js'

// A run of one feature with one scenario of two steps, as the /ws feed
// sends it, each message a second after the last.
const feed = [
  { type: 'run_started' },
  { type: 'run_created', runId: '216b61f6' },
  { type: 'run_phase', phase: 'containers' },
  { type: 'container_status', name: 'postgres', status: 'starting' },
  { type: 'container_status', name: 'postgres', status: 'ready', durationMs: 2300 },
  { type: 'run_phase', phase: 'resources' },
  { type: 'run_phase', phase: 'app' },
  { type: 'app_status', name: 'orderly', status: 'starting' },
  { type: 'app_status', name: 'orderly', status: 'listening', url: 'http://localhost:8099', durationMs: 900 },
  { type: 'app_status', name: 'orderly', status: 'ready', url: 'http://localhost:8099', durationMs: 6100 },
  { type: 'run_phase', phase: 'resources' },
  { type: 'run_phase', phase: 'tests' },
  { type: 'feature_started', feature: 'Orders', file: 'features/orders.feature' },
  { type: 'scenario_running', feature: 'Orders', scenario: 'An order is placed', status: 'running' },
  { type: 'step_started', scenario: 'An order is placed', stepIndex: 0, status: 'running' },
  { type: 'step_finished', scenario: 'An order is placed', stepIndex: 0, status: 'passed', durationMs: 12 },
  { type: 'step_started', scenario: 'An order is placed', stepIndex: 1, status: 'running' },
  { type: 'step_finished', scenario: 'An order is placed', stepIndex: 1, status: 'failed', durationMs: 3, output: 'expected 201, got 500' },
  { type: 'scenario_failed', feature: 'Orders', scenario: 'An order is placed', status: 'failed', durationMs: 15 },
  { type: 'feature_finished', feature: 'Orders' },
  { type: 'run_summary', total: 1, failed: 1 },
  { type: 'run_finished', status: 'failed' },
]

const recorded = () => feed.reduce((reel, m, i) => record(reel, m, 1000 + i * 1000), emptyReel())

describe('record', () => {
  it('stamps each message with the time since the run started', () => {
    const reel = recorded()
    expect(reel.events.map((e) => e.at)).toEqual(feed.map((_, i) => i * 1000))
    expect(reel.source).toBe('live')
  })

  it('knows the run id once the runner has said it', () => {
    expect(reelRunId(emptyReel())).toBeNull()
    expect(reelRunId(recorded())).toBe('216b61f6')
  })

  it('ignores what does not change the stage', () => {
    const reel = record(recorded(), { type: 'run_output', output: 'line' }, 99999)
    expect(reel.events).toHaveLength(feed.length)
  })

  it('starts a new reel on run_started', () => {
    const reel = record(recorded(), { type: 'run_started' }, 50000)
    expect(reel.events).toHaveLength(1)
    expect(reel.startedAt).toBe(50000)
  })

  it('starts from the first message when the run was joined midway', () => {
    const reel = record(emptyReel(), { type: 'scenario_running', scenario: 'S' }, 7000)
    expect(reel.events).toEqual([{ type: 'scenario_running', scenario: 'S', at: 0 }])
    expect(record(reel, { type: 'step_started', scenario: 'S', stepIndex: 0 }, 7500).events[1].at).toBe(500)
  })
})

describe('the scene', () => {
  const sceneAfter = (n) => foldScene(recorded().events, n).scene

  it('starts with the run coming up', () => {
    expect(sceneAfter(2)).toMatchObject({ runId: '216b61f6', act: 'start' })
    expect(sceneAfter(4)).toMatchObject({ act: 'deps', phase: 'containers', containers: [{ name: 'postgres', status: 'starting', ms: null }] })
    expect(sceneAfter(5).containers).toEqual([{ name: 'postgres', status: 'ready', ms: 2300 }])
  })

  it('then the app, which keeps its address and time as it comes up', () => {
    expect(sceneAfter(8)).toMatchObject({ act: 'app', app: { name: 'orderly', status: 'starting', url: null } })
    expect(sceneAfter(9).app).toEqual({ name: 'orderly', status: 'listening', ms: 900, url: 'http://localhost:8099' })
    expect(sceneAfter(10).app).toMatchObject({ status: 'ready', ms: 6100 })
    // Initialising resources after the app is still the app's act.
    expect(sceneAfter(11).act).toBe('app')
  })

  it('then the scenarios, feature by feature, step by step', () => {
    expect(sceneAfter(13)).toMatchObject({ act: 'scenarios', feature: 'Orders', scenario: null, features: [{ name: 'Orders', file: 'features/orders.feature', scenarios: [] }] })
    expect(sceneAfter(14)).toMatchObject({ scenario: 'An order is placed', step: null, features: [{ scenarios: ['An order is placed'] }] })
    expect(sceneAfter(15).step).toEqual({ index: 0, status: 'running' })
    expect(sceneAfter(18).step).toEqual({ index: 1, status: 'failed', ms: 3, error: 'expected 201, got 500' })
  })

  it('folds the per-step run state from the same events', () => {
    const { run } = foldScene(recorded().events, 19)
    expect(run.scStatus).toEqual({ 'An order is placed': 'failed' })
    expect(run.stepStatus['An order is placed\u00000']).toBe('passed')
    expect(run.stepError['An order is placed\u00001']).toBe('expected 201, got 500')
    // Scrubbed back to before the second step, nothing is known of it.
    expect(foldScene(recorded().events, 16).run.stepStatus['An order is placed\u00001']).toBeUndefined()
  })

  it('ends on the report with the count', () => {
    const s = sceneAfter(feed.length)
    expect(s).toMatchObject({ act: 'report', summary: { total: 1, passed: 0, failed: 1, skipped: 0 }, finished: { status: 'failed', at: 21000 } })
  })

  it('records a failure outside the scenarios where it happened', () => {
    const events = [
      { type: 'run_started', at: 0 }, { type: 'run_phase', phase: 'app', at: 1 },
      { type: 'app_status', name: 'orderly', status: 'starting', at: 2 },
      { type: 'run_error', phase: 'app', status: 'failed', output: 'port 8099 is already in use', logs: ['last line'], at: 3 },
      { type: 'run_finished', status: 'failed', at: 4 },
    ]
    const { scene } = foldScene(events, events.length)
    expect(scene.error).toEqual({ phase: 'app', message: 'port 8099 is already in use', logs: ['last line'] })
    expect(scene.finished.status).toBe('failed')
  })

  it('goes on from the previous fold when the events only grew', () => {
    const events = recorded().events
    const first = foldScene(events, 10)
    const more = foldScene(events, 15, first)
    expect(more.scene).toEqual(foldScene(events, 15).scene)
    expect(more.run).toEqual(foldScene(events, 15).run)
    // A new run's events are other objects, so the old fold is not reused.
    const other = feed.slice(0, 15).map((m, i) => ({ ...m, at: i }))
    expect(foldScene(other, 15, first).scene).toEqual(foldScene(other, 15).scene)
  })
})

describe('beats and acts', () => {
  it('skips the events that ride along with others', () => {
    const events = recorded().events
    const beats = beatsOf(events)
    const skipped = events.filter((_, i) => !beats.includes(i)).map((e) => e.type)
    expect(skipped).toEqual(['run_created', 'feature_finished', 'run_summary'])
  })

  it('opens each act on its first beat', () => {
    const events = recorded().events
    const beats = beatsOf(events)
    const acts = actsOf(events, beats)
    expect(acts.map((a) => a.key)).toEqual(['deps', 'app', 'scenarios', 'report'])
    expect(acts.map((a) => events[beats[a.beat]].type)).toEqual(['run_phase', 'run_phase', 'run_phase', 'run_finished'])
    expect(events[beats[acts[1].beat]].phase).toBe('app')
  })

  it('only lists the acts a run has reached', () => {
    const events = recorded().events.slice(0, 9)
    expect(actsOf(events, beatsOf(events)).map((a) => a.key)).toEqual(['deps', 'app'])
  })

  it('holds a title longer than a step', () => {
    expect(dwellOf({ type: 'feature_started' })).toBeGreaterThan(dwellOf({ type: 'step_finished' }))
    expect(dwellOf({ type: 'scenario_failed' })).toBeGreaterThan(dwellOf({ type: 'scenario_passed' }))
    expect(dwellOf(undefined)).toBeGreaterThan(0)
  })
})

describe('fromLog', () => {
  it('paces a past run by the times its log knows', () => {
    const reel = fromLog(feed)
    expect(reel.source).toBe('log')
    const at = Object.fromEntries(reel.events.map((e) => [`${e.type}:${e.status ?? e.phase ?? ''}`, e.at]))
    expect(at['container_status:starting']).toBe(0)
    expect(at['container_status:ready']).toBe(2300)
    expect(at['app_status:listening']).toBe(3200)
    // The app's time is from when it started, not on top of its listening.
    expect(at['app_status:ready']).toBe(8400)
    expect(reel.events.filter((e) => e.type === 'step_finished').map((e) => e.at)).toEqual([8412, 8415])
    // A scenario ends when its own duration says, which includes what ran
    // around its steps.
    expect(at['scenario_failed:failed']).toBe(8415)
    expect(reel.events.map((e) => e.type)).not.toContain('run_output')
  })
})
