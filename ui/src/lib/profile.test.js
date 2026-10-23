import { describe, expect, it } from 'vitest'
import { costliest, fmtBytes, laneMax, linePath, profileFacts, samplesIn, scenarioBands, slowestDependency, timelineLanes } from './profile.js'
import { emptyRun, maxSamples, reduceRun } from './run.js'

const MB = 1 << 20
const profile = {
  start: '2026-10-22T07:00:00.000Z', end: '2026-10-22T07:00:02.000Z', durationMs: 2000,
  app: { cpuMs: 450, peakRss: 180 * MB, rssStart: 100 * MB, rssEnd: 140 * MB },
  network: {
    'postgres-network': { container: 'postgres', waitMs: 1400, roundTrips: 412 },
    'kafka-network': { container: 'kafka', waitMs: 100, roundTrips: 1 },
  },
  logs: { errors: 2, warnings: 1, examples: ['ERROR declined'] },
}

describe('scenario facts', () => {
  it('names CPU, memory, its growth, the slowest dependency and logged errors', () => {
    const facts = Object.fromEntries(profileFacts(profile).map((f) => [f.key, f]))
    expect(facts.cpu.value).toBe('450ms')
    expect(facts.mem.value).toBe('180 MB')
    expect(facts.delta).toMatchObject({ value: '+40 MB', warn: true })
    expect(facts.dep).toMatchObject({ label: 'postgres', value: '1.4s · 412 round trips' })
    expect(facts.err).toMatchObject({ value: '2 errors', warn: true, title: 'ERROR declined' })
  })
  it('leaves out what did not happen', () => {
    const quiet = { app: { cpuMs: 3, peakRss: 50 * MB, rssStart: 50 * MB, rssEnd: 50 * MB } }
    expect(profileFacts(quiet).map((f) => f.key)).toEqual(['cpu', 'mem'])
    expect(profileFacts(undefined)).toEqual([])
    expect(slowestDependency(quiet)).toBeNull()
  })
})

describe('timeline', () => {
  const samples = [
    { at: '2026-10-22T06:59:59.000Z', app: { cpu: 5, rss: 90 * MB } },
    { at: '2026-10-22T07:00:01.000Z', app: { cpu: 80, rss: 160 * MB }, containers: { postgres: { cpu: 40 }, kafka: { cpu: 3 } } },
    { at: '2026-10-22T07:00:03.000Z', app: { cpu: 2, rss: 140 * MB }, containers: { postgres: { cpu: 1 }, kafka: { cpu: 2 } } },
  ]
  it('keeps the samples inside a scenario', () => {
    expect(samplesIn(samples, profile).map((s) => s.app.cpu)).toEqual([80])
  })
  it('has the app first and the busiest container next', () => {
    expect(timelineLanes(samples).map((l) => l.key)).toEqual(['app-cpu', 'app-mem', 'ctr-postgres', 'ctr-kafka'])
  })
  it('draws against a scale that a quiet lane does not fill', () => {
    expect(laneMax({ unit: '%', points: [[0, 3]] })).toBe(100)
    expect(laneMax({ unit: '%', points: [[0, 180]] })).toBe(200)
    expect(laneMax({ unit: 'bytes', points: [[0, MB]] })).toBe(64 * MB)
  })
  it('draws a line across the width', () => {
    expect(linePath([[0, 0], [10, 50], [20, 100]], { from: 0, to: 20, max: 100, width: 200, height: 10 })).toBe('M0.0,10.0L100.0,5.0L200.0,0.0')
    expect(linePath([], { from: 0, to: 1, max: 1, width: 1, height: 1 })).toBe('')
  })
  it('orders bands by when they ran and the table by cost', () => {
    const later = { ...profile, start: '2026-10-22T07:00:05.000Z', end: '2026-10-22T07:00:06.000Z', app: { cpuMs: 900 } }
    expect(scenarioBands({ b: later, a: profile }).map((b) => b.name)).toEqual(['a', 'b'])
    expect(costliest({ a: profile, b: later }).map((c) => c.name)).toEqual(['b', 'a'])
  })
  it('formats bytes', () => {
    expect(fmtBytes(512)).toBe('512 B')
    expect(fmtBytes(3 * (1 << 30))).toBe('3.0 GB')
  })
})

describe('run state', () => {
  it('keeps each scenario\'s profile and the run\'s samples', () => {
    let run = reduceRun(emptyRun(), { type: 'step_finished', scenario: 'S', stepIndex: 0, status: 'passed', samples: [{ at: 'a' }] })
    run = reduceRun(run, { type: 'scenario_passed', scenario: 'S', durationMs: 5, profile, samples: [{ at: 'b' }] })
    expect(run.scProfile.S).toBe(profile)
    expect(run.samples.map((s) => s.at)).toEqual(['a', 'b'])
  })
  it('keeps no more than maxSamples', () => {
    const many = Array.from({ length: maxSamples + 5 }, (_, i) => ({ at: i }))
    const run = reduceRun(emptyRun(), { type: 'scenario_passed', scenario: 'S', samples: many })
    expect(run.samples).toHaveLength(maxSamples)
    expect(run.samples[0].at).toBe(5)
  })
})
