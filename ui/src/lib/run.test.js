import { describe, expect, it } from 'vitest'
import {
  backgroundScenario, backgroundStatus, emptyRun, failedBackgroundSteps,
  pickleIndex, reduceRun, stepRun,
} from './run.js'

// A scenario's rows as the cards draw them: its own steps offset by the
// Background, each read back through stepRun.
const rows = (feature, scenario, run) => scenario.steps.map((step, i) => (
  stepRun(run, scenario.name, pickleIndex(feature.background, i), step)
))
const bgRows = (feature, scenarioName, run) => (feature.background ?? []).map((step, i) => (
  stepRun(run, scenarioName, i, step)
))

const feed = (events, from = emptyRun()) => events.reduce(reduceRun, from)
const steps = (...texts) => texts.map((text) => ({ keyword: 'Given ', text }))

// step_finished as `tomato ui` relays it from the runner's step_end: the
// pickle index, never a per-card counter.
const done = (scenario, stepIndex, status, durationMs, output) => (
  { type: 'step_finished', scenario, stepIndex, status, durationMs, ...(output ? { output } : {}) }
)
const started = (scenario) => ({ type: 'scenario_running', scenario })
const ended = (scenario, status, durationMs) => (
  { type: `scenario_${status}`, scenario, status, durationMs }
)

describe('without a Background', () => {
  const feature = {
    scenarios: [{ name: 'Passing', steps: steps('"shell" runs "echo hello"', '"shell" succeeds', '"shell" stdout contains "hello"') }],
  }

  it('scenario row i is pickle step i', () => {
    const run = feed([
      { type: 'run_started' }, started('Passing'),
      done('Passing', 0, 'passed', 12), done('Passing', 1, 'passed', 0), done('Passing', 2, 'failed', 3, 'stdout does not contain "hello"'),
      ended('Passing', 'failed', 15),
    ])
    expect(rows(feature, feature.scenarios[0], run)).toEqual([
      { status: 'passed', dur: 12, error: undefined },
      { status: 'passed', dur: 0, error: undefined },
      { status: 'failed', dur: 3, error: 'stdout does not contain "hello"' },
    ])
    expect(backgroundScenario(feature, run)).toBe('Passing')
    expect(failedBackgroundSteps(feature.background, 'Passing', run)).toEqual([])
    expect(backgroundStatus(feature.background, 'Passing', run)).toBe('')
  })

  it('an unmatched step reads as undefined before anything ran', () => {
    const run = emptyRun()
    expect(stepRun(run, 'Passing', 0, { text: 'x', unmatched: true }).status).toBe('undefined')
    expect(stepRun(run, 'Passing', 0, { text: 'x' }).status).toBe('')
    expect(stepRun(run, null, 0, { text: 'x', unmatched: true }).status).toBe('undefined')
  })
})

// The feature from #212: a six-step Background and a scenario of seven steps
// whose third step fails, which is pickle step 8.
const issue = {
  background: steps(
    '"db" table "clause" has values:', '"db" table "country_clause" has values:',
    '"alice" opens "/master"', '"alice" sees "7.2 Zinsen werden monatlich gezahlt." within "5s"',
    '"alice" types " und Gebühren" at the end of "7.2"', '"alice" sees "Saved" within "5s"',
  ),
  scenarios: [{
    name: 'A change scoped to selected countries reaches only those',
    steps: steps(
      '"alice" clicks "Apply"', '"alice" chooses "Selected countries"', '"alice" checks "Austria"',
      '"alice" checks "Netherlands"', '"alice" clicks "Confirm"',
      '"alice" sees "Change applied to 2 countries" within "5s"', '"db" query "SELECT ..." returns:',
    ),
  }],
}
const S = issue.scenarios[0].name
const austria = '"alice" cannot check "Austria": no checkbox "Austria" on the page'
const issueEvents = [
  { type: 'run_started' }, started(S),
  done(S, 0, 'passed', 0), done(S, 1, 'passed', 1), done(S, 2, 'passed', 111),
  done(S, 3, 'passed', 45), done(S, 4, 'passed', 128), done(S, 5, 'passed', 404),
  done(S, 6, 'passed', 231), done(S, 7, 'passed', 635), done(S, 8, 'failed', 5094, austria),
  done(S, 9, 'skipped', 0), done(S, 10, 'skipped', 0), done(S, 11, 'skipped', 0), done(S, 12, 'skipped', 0),
  ended(S, 'failed', 6660),
]

describe('a Background with a failing scenario step', () => {
  const run = feed(issueEvents)

  it('puts the failed step on the scenario row that failed, with its error', () => {
    const r = rows(issue, issue.scenarios[0], run)
    expect(r.map((x) => x.status)).toEqual(['passed', 'passed', 'failed', 'skipped', 'skipped', 'skipped', 'skipped'])
    expect(r[2].error).toBe(austria)
    expect(r.filter((x) => x.error)).toHaveLength(1)
  })

  it('shows every Background row as passed for that scenario', () => {
    expect(bgRows(issue, S, run).map((x) => x.status)).toEqual(['passed', 'passed', 'passed', 'passed', 'passed', 'passed'])
    expect(backgroundStatus(issue.background, S, run)).toBe('passed')
    expect(failedBackgroundSteps(issue.background, S, run)).toEqual([])
  })

  it('keeps the steps after the failure skipped, not blank', () => {
    const r = rows(issue, issue.scenarios[0], run)
    expect(r.slice(3).every((x) => x.status === 'skipped')).toBe(true)
  })

  it('lands each duration on the row that took it', () => {
    expect(bgRows(issue, S, run).map((x) => x.dur)).toEqual([0, 1, 111, 45, 128, 404])
    expect(rows(issue, issue.scenarios[0], run).map((x) => x.dur)).toEqual([231, 635, 5094, 0, 0, 0, 0])
  })

  it('records the scenario itself as failed with its duration', () => {
    expect(run.scStatus[S]).toBe('failed')
    expect(run.scDur[S]).toBe(6660)
  })
})

describe('a failing Background step', () => {
  const noSaved = '"alice" does not see "Saved" within "5s"'
  const run = feed([
    { type: 'run_started' }, started(S),
    done(S, 0, 'passed', 0), done(S, 1, 'passed', 1), done(S, 2, 'passed', 90), done(S, 3, 'passed', 40),
    done(S, 4, 'passed', 100), done(S, 5, 'failed', 5002, noSaved),
    ...[6, 7, 8, 9, 10, 11, 12].map((i) => done(S, i, 'skipped', 0)),
    ended(S, 'failed', 5300),
  ])

  it('marks the Background row that failed, for the scenario that ran it', () => {
    expect(bgRows(issue, S, run).map((x) => x.status)).toEqual(['passed', 'passed', 'passed', 'passed', 'passed', 'failed'])
    expect(bgRows(issue, S, run)[5].error).toBe(noSaved)
    expect(backgroundStatus(issue.background, S, run)).toBe('failed')
  })

  it('hands the scenario card the failed Background step to show', () => {
    const failed = failedBackgroundSteps(issue.background, S, run)
    expect(failed).toHaveLength(1)
    expect(failed[0].index).toBe(5)
    expect(failed[0].step.text).toBe('"alice" sees "Saved" within "5s"')
    expect(failed[0].error).toBe(noSaved)
  })

  it('leaves every one of the scenario’s own steps skipped', () => {
    expect(rows(issue, issue.scenarios[0], run).map((x) => x.status)).toEqual(Array(7).fill('skipped'))
  })

  it('shows the Background card as this scenario’s run', () => {
    expect(backgroundScenario(issue, run)).toBe(S)
  })
})

describe('two scenarios sharing one Background', () => {
  const feature = {
    background: steps('"shell" runs "echo setup"', '"shell" succeeds'),
    scenarios: [
      { name: 'Passing', steps: steps('"shell" runs "echo hello"', '"shell" stdout contains "hello"') },
      { name: 'Failing on purpose', steps: steps('"shell" runs "echo hello"', '"shell" stdout contains "nope"') },
    ],
  }
  const first = [
    { type: 'run_started' }, started('Passing'),
    done('Passing', 0, 'passed', 5), done('Passing', 1, 'passed', 0), done('Passing', 2, 'passed', 6), done('Passing', 3, 'passed', 1),
    ended('Passing', 'passed', 12),
  ]
  const second = [
    started('Failing on purpose'),
    done('Failing on purpose', 0, 'passed', 7), done('Failing on purpose', 1, 'failed', 2, 'exit 1'),
    done('Failing on purpose', 2, 'skipped', 0), done('Failing on purpose', 3, 'skipped', 0),
    ended('Failing on purpose', 'failed', 9),
  ]

  it('follows the scenario that is running, then the last one that ran', () => {
    const afterFirst = feed(first)
    expect(backgroundScenario(feature, afterFirst)).toBe('Passing')
    const secondRunning = feed([started('Failing on purpose'), done('Failing on purpose', 0, 'passed', 7)], afterFirst)
    expect(backgroundScenario(feature, secondRunning)).toBe('Failing on purpose')
    const both = feed(second, afterFirst)
    expect(backgroundScenario(feature, both)).toBe('Failing on purpose')
    const finished = reduceRun(both, { type: 'run_finished' })
    expect(backgroundScenario(feature, finished)).toBe('Failing on purpose')
  })

  it('keeps each scenario’s Background run apart', () => {
    const run = feed([...first, ...second])
    expect(bgRows(feature, 'Passing', run).map((x) => x.status)).toEqual(['passed', 'passed'])
    expect(bgRows(feature, 'Failing on purpose', run).map((x) => x.status)).toEqual(['passed', 'failed'])
    expect(failedBackgroundSteps(feature.background, 'Passing', run)).toEqual([])
    expect(failedBackgroundSteps(feature.background, 'Failing on purpose', run).map((r) => r.index)).toEqual([1])
    expect(bgRows(feature, 'Passing', run).map((x) => x.dur)).toEqual([5, 0])
    expect(bgRows(feature, 'Failing on purpose', run).map((x) => x.dur)).toEqual([7, 2])
  })

  it('does not let one scenario’s rows show the other’s steps', () => {
    const run = feed([...first, ...second])
    expect(rows(feature, feature.scenarios[0], run).map((x) => x.status)).toEqual(['passed', 'passed'])
    expect(rows(feature, feature.scenarios[1], run).map((x) => x.status)).toEqual(['skipped', 'skipped'])
    expect(rows(feature, feature.scenarios[0], run).map((x) => x.dur)).toEqual([6, 1])
  })

  it('ignores another feature’s scenarios when picking whose run to show', () => {
    const other = { background: steps('"shell" runs "true"'), scenarios: [{ name: 'Elsewhere', steps: [] }] }
    const run = feed([...first, started('Elsewhere'), done('Elsewhere', 0, 'passed', 1), ended('Elsewhere', 'passed', 1)])
    expect(backgroundScenario(feature, run)).toBe('Passing')
    expect(backgroundScenario(other, run)).toBe('Elsewhere')
    expect(backgroundScenario({ scenarios: [{ name: 'Never ran', steps: [] }] }, run)).toBeNull()
  })
})

describe('the run itself', () => {
  it('starts empty, and a new run forgets the last one', () => {
    const run = feed(issueEvents)
    expect(reduceRun(run, { type: 'run_started' })).toEqual(emptyRun())
  })

  it('marks the running step, and clears a scenario that never reported when the run ends', () => {
    const run = feed([{ type: 'run_started' }, started(S), { type: 'step_started', scenario: S, stepIndex: 7 }])
    expect(stepRun(run, S, 7).status).toBe('running')
    expect(run.scStatus[S]).toBe('running')
    const ended = reduceRun(run, { type: 'run_error', status: 'failed' })
    expect(ended.scStatus[S]).toBeUndefined()
    expect(ended.scOrder).toEqual([S])
  })

  it('returns the same state for a message it does not read', () => {
    const run = feed(issueEvents)
    expect(reduceRun(run, { type: 'run_output', output: 'x' })).toBe(run)
    expect(reduceRun(run, { type: 'step_finished', status: 'passed' })).toBe(run)
    expect(reduceRun(run, { type: 'run_finished' })).toBe(run)
  })
})

describe('a failed comparison', () => {
  it('keeps the mismatch the runner sent with the step', () => {
    const mismatch = { what: 'stdout', expected: 'a', actual: 'b', mode: 'text' }
    const step = { keyword: 'Then ', text: '"shell" stdout is:' }
    const run = feed([
      { type: 'run_started' }, started('S'),
      { ...done('S', 0, 'failed', 3, 'stdout mismatch\nexpected: a\nactual: b'), mismatch },
    ])
    expect(stepRun(run, 'S', 0, step)).toMatchObject({ status: 'failed', mismatch })
    expect(stepRun(feed([{ type: 'run_started' }, done('S', 0, 'failed', 1, 'boom')]), 'S', 0, step).mismatch).toBeUndefined()
  })
})

describe('network on a step', () => {
  it('keeps what network resources did while the step ran, and the faults on when it ended', () => {
    const network = {
      events: [{ at: '2026-10-21T07:03:23.120Z', resource: 'postgres-network', kind: 'applied', detail: 'resets connections for 3s' }],
      active: ['postgres-network resets connections (step 1, 2.9s left)'],
    }
    const run = reduceRun(emptyRun(), { type: 'step_finished', scenario: 'S', stepIndex: 0, status: 'passed', stepNetwork: network })
    expect(stepRun(run, 'S', 0).network).toEqual(network)
    expect(stepRun(run, 'S', 1).network).toBeUndefined()
  })
})
