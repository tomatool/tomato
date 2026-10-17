import { describe, expect, it } from 'vitest'
import { emptyRun, reduceRun } from './run.js'
import {
  newestFirst, normPath, samePath, screenshotURL, shotName, shotPlace, stepScreenshots, takenBy,
} from './screenshots.js'

const feed = (events, from = emptyRun()) => events.reduce(reduceRun, from)
const started = (scenario) => ({ type: 'scenario_running', scenario })
// step_finished as `tomato ui` relays the runner's step_end: the pickle index
// and, when the step left pictures behind, what the resource recorded.
const done = (scenario, stepIndex, status, stepScreenshots, output) => ({
  type: 'step_finished', scenario, stepIndex, status, durationMs: 1,
  ...(stepScreenshots ? { stepScreenshots } : {}), ...(output ? { output } : {}),
})

const SAVED = 'A screenshot of the page'
const master = { resource: 'alice', path: '.tomato/screenshots/master.png' }

describe('a saved screenshot', () => {
  const run = feed([
    { type: 'run_started' }, started(SAVED),
    done(SAVED, 0, 'passed'), done(SAVED, 1, 'passed'), done(SAVED, 2, 'passed'),
    done(SAVED, 3, 'passed', [master]), done(SAVED, 4, 'passed'),
  ])

  it('belongs to the step that saved it and to no other', () => {
    expect(stepScreenshots(run, SAVED, 3)).toEqual([master])
    expect(stepScreenshots(run, SAVED, 2)).toEqual([])
    expect(stepScreenshots(run, SAVED, 4)).toEqual([])
    expect(stepScreenshots(run, null, 3)).toEqual([])
  })

  it('is found from its path, however the path is spelled', () => {
    const by = { scenario: SAVED, index: 3, resource: 'alice', status: 'passed' }
    expect(takenBy(run, '.tomato/screenshots/master.png')).toEqual(by)
    expect(takenBy(run, './.tomato/screenshots/master.png')).toEqual(by)
  })
})

describe('a failure screenshot', () => {
  const FAILED = 'The order is cancelled'
  const alice = { resource: 'alice', path: '.tomato/browser-fixture/failed-The-order-is-cancelled-alice.png' }
  const bob = { resource: 'bob', path: '.tomato/browser-fixture/failed-The-order-is-cancelled-bob.png' }
  const run = feed([
    { type: 'run_started' }, started(FAILED),
    done(FAILED, 0, 'passed'),
    done(FAILED, 1, 'failed', [alice, bob], '"alice" does not see "Order cancelled"; the page shows: "Order confirmed"'),
    done(FAILED, 2, 'skipped'),
    { type: 'scenario_failed', scenario: FAILED, status: 'failed' },
  ])

  it('is matched to the step that failed, one per person with a page open', () => {
    expect(stepScreenshots(run, FAILED, 1)).toEqual([alice, bob])
    expect(stepScreenshots(run, FAILED, 0)).toEqual([])
    expect(stepScreenshots(run, FAILED, 2)).toEqual([])
    expect(takenBy(run, bob.path)).toEqual({ scenario: FAILED, index: 1, resource: 'bob', status: 'failed' })
  })

  it('is forgotten when the next run starts', () => {
    const next = reduceRun(run, { type: 'run_started' })
    expect(stepScreenshots(next, FAILED, 1)).toEqual([])
    expect(takenBy(next, alice.path)).toBeNull()
  })
})

describe('two scenarios with the same step text', () => {
  // Both fail on `"alice" sees "Saved"` as their second step; the file name
  // alone would tell the scenarios apart but not the steps, and the step
  // text would tell neither. The path recorded on each step does both.
  const A = 'Saving a clause'
  const B = 'Saving a country clause'
  const shotA = { resource: 'alice', path: '.tomato/runs/r1/screenshots/failed-Saving-a-clause-alice.png' }
  const shotB = { resource: 'alice', path: '.tomato/runs/r1/screenshots/failed-Saving-a-country-clause-alice.png' }
  const run = feed([
    { type: 'run_started' },
    started(A), done(A, 0, 'passed'), done(A, 1, 'failed', [shotA], 'does not see "Saved"'),
    started(B), done(B, 0, 'passed'), done(B, 1, 'failed', [shotB], 'does not see "Saved"'),
  ])

  it('gives each scenario its own picture', () => {
    expect(stepScreenshots(run, A, 1)).toEqual([shotA])
    expect(stepScreenshots(run, B, 1)).toEqual([shotB])
    expect(takenBy(run, shotA.path)).toMatchObject({ scenario: A, index: 1 })
    expect(takenBy(run, shotB.path)).toMatchObject({ scenario: B, index: 1 })
  })
})

describe('a name that matches nothing', () => {
  const run = feed([
    { type: 'run_started' }, started(SAVED), done(SAVED, 3, 'passed', [master]),
  ])

  it('is nobody’s: an earlier run’s file, or one saved by hand', () => {
    expect(takenBy(run, '.tomato/screenshots/after-apply.png')).toBeNull()
    expect(takenBy(run, '.tomato/runs/r0/screenshots/master.png')).toBeNull()
    expect(takenBy(emptyRun(), master.path)).toBeNull()
    expect(takenBy({}, master.path)).toBeNull()
  })
})

describe('ordering', () => {
  const shot = (path, modTime) => ({ name: shotName(path), path, modTime })

  it('is newest first, with a tie broken by path', () => {
    const shots = [
      shot('.tomato/screenshots/a.png', '2025-01-01T10:00:00Z'),
      shot('.tomato/screenshots/c.png', '2025-01-01T12:00:00.000000123Z'),
      shot('.tomato/screenshots/b.png', '2025-01-01T12:00:00.000000123Z'),
      shot('.tomato/runs/r1/screenshots/d.png', '2025-01-01T11:00:00Z'),
    ]
    expect(newestFirst(shots).map((s) => s.name)).toEqual(['b', 'c', 'd', 'a'])
    // A copy: the server's list is left alone.
    expect(shots.map((s) => s.name)).toEqual(['a', 'c', 'b', 'd'])
    expect(newestFirst(undefined)).toEqual([])
  })
})

describe('paths', () => {
  it('compares spellings of one path as equal', () => {
    expect(normPath('./.tomato//screenshots/x.png')).toBe('.tomato/screenshots/x.png')
    expect(samePath('./shots/a.png', 'shots/a.png')).toBe(true)
    expect(samePath('shots/a.png', 'shots/b.png')).toBe(false)
  })

  it('builds the URL the server serves a file at', () => {
    expect(screenshotURL('.tomato/screenshots/after apply#1.png')).toBe('/api/screenshots/.tomato/screenshots/after%20apply%231.png')
    expect(screenshotURL('/abs/dir/x.png')).toBe('/api/screenshots/abs/dir/x.png')
  })

  it('names and places a screenshot', () => {
    expect(shotName('.tomato/screenshots/failed-Next-alice.PNG')).toBe('failed-Next-alice')
    expect(shotPlace('.tomato/runs/2025-01-01_000000_abcd1234/screenshots')).toBe('run abcd1234')
    expect(shotPlace('reports/shots')).toBe('reports/shots')
  })
})
