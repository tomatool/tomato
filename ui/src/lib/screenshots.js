// The screenshots the browser resources leave behind, as the UI reads them:
// the list /api/screenshots serves, and which step of this run took each one.
//
// A step's screenshots arrive on its step_finished, recorded by the resource
// that wrote the file (internal/resource) and relayed from the runner's
// step_end. So a picture is matched to its step by the path itself — never
// guessed from the file's name, which carries the scenario but not the step,
// and which an earlier run may have left behind under the same name.

import { splitKey, stepKey } from './run.js'

// normPath makes two spellings of one path compare equal: the runner's
// `.tomato/x.png` and a `./.tomato/x.png`, or a doubled slash.
export function normPath(p) {
  return String(p ?? '').replace(/\\/g, '/').replace(/^(\.\/)+/, '').replace(/\/{2,}/g, '/')
}

export const samePath = (a, b) => normPath(a) === normPath(b)

// screenshotURL is where the server serves the file at `path`: each segment
// escaped on its own so the slashes stay, and an absolute path without its
// leading slash (command/ui_screenshots.go reads both back).
export function screenshotURL(path) {
  const segments = normPath(path).replace(/^\//, '').split('/').map(encodeURIComponent)
  return `/api/screenshots/${segments.join('/')}`
}

// shotName is the file's name without its extension, as the list shows it.
export const shotName = (path) => normPath(path).split('/').pop().replace(/\.png$/i, '')

// shotPlace says where a screenshot sits in a few words: `run a1b2c3d4` for a
// run's own directory under .tomato/runs, else the directory as configured.
export function shotPlace(dir) {
  const m = normPath(dir).match(/^\.tomato\/runs\/([^/]+)\/screenshots$/)
  return m ? `run ${m[1].split('_').pop()}` : normPath(dir)
}

// newestFirst sorts a copy by modification time, newest first; two taken in
// the same instant keep a stable order by path.
export function newestFirst(shots) {
  const at = (s) => {
    const t = Date.parse(s.modTime)
    return Number.isNaN(t) ? 0 : t
  }
  return [...(shots ?? [])].sort((a, b) => (at(b) - at(a)) || String(a.path).localeCompare(String(b.path)))
}

// stepScreenshots are the screenshots pickle step `index` of `scenario` took
// in this run, [{resource, path}]; empty when it took none.
export function stepScreenshots(runState, scenario, index) {
  if (scenario == null || index == null) return []
  return runState?.stepShots?.[stepKey(scenario, index)] ?? []
}

// takenBy says which step of this run left the screenshot at `path`:
// { scenario, index, resource, status }, or null when no step of this run did
// — a file an earlier run left, or one with a name no step produced.
export function takenBy(runState, path) {
  for (const [k, list] of Object.entries(runState?.stepShots ?? {})) {
    const hit = (list ?? []).find((s) => samePath(s.path, path))
    if (!hit) continue
    const [scenario, index] = splitKey(k)
    return { scenario, index, resource: hit.resource, status: runState.stepStatus?.[k] ?? '' }
  }
  return null
}
