import { takenBy } from './screenshots.js'

// A run's id is the last part of its directory name: 2026-10-23_050417_216b61f6.
export const runIdOf = (name) => String(name ?? '').split('_').pop()

// shotsOfRun is the screenshots a run took: those under its run directory,
// and, for the run in the page, any its steps recorded wherever they wrote
// them (a browser's own screenshots directory).
export function shotsOfRun(shots, runName, runState) {
  return (shots ?? []).filter((s) => {
    const path = String(s.path ?? '').replaceAll('\\', '/')
    if (path.includes(`/runs/${runName}/`) || path.startsWith(`runs/${runName}/`)) return true
    return Boolean(runState && takenBy(runState, s.path))
  })
}
