import { describe, expect, it } from 'vitest'
import { runIdOf, shotsOfRun } from './runs.js'

describe('runs', () => {
  it('reads the id off a run directory', () => {
    expect(runIdOf('2026-10-23_050417_216b61f6')).toBe('216b61f6')
    expect(runIdOf(undefined)).toBe('')
  })

  it("keeps a run's screenshots: under its directory, or taken by its steps", () => {
    const shots = [
      { path: '.tomato/runs/2026-10-23_050417_216b61f6/screenshots/failed-a.png' },
      { path: '.tomato/runs/2026-10-23_050220_4d586811/screenshots/failed-b.png' },
      { path: '.tomato/screenshots/master.png' },
      { path: 'shots\\other.png' },
    ]
    const run = '2026-10-23_050417_216b61f6'
    expect(shotsOfRun(shots, run, null).map((s) => s.path)).toEqual([shots[0].path])
    const live = { stepShots: { 'S\u00002': [{ resource: 'alice', path: '.tomato/screenshots/master.png' }] }, stepStatus: {} }
    expect(shotsOfRun(shots, run, live).map((s) => s.path)).toEqual([shots[0].path, shots[2].path])
  })
})
