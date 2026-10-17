// What a failed comparison got wrong, worked out from both sides of it.
//
// A step that compares a whole value — `stdout is:`, `response json
// contains:`, a Kafka message — fails with a `mismatch` on its step_end
// (internal/formatter): what was compared, the expected and actual text, and
// the mode. Older runners only sent the text, "<what> mismatch:\nexpected:
// …\nactual: …", which parseMismatchText reads back.
//
// The mode decides what the mistake is. In a subset match ("subset") keys
// that only the actual value has are allowed, so they are context, not
// differences; in an exact match ("json") they are differences; and a text
// comparison ("text") counts every difference, down to key order.

import { collapse, diffLines, sideBySide } from './linediff.js'

// HUGE is the size above which a value is shown as plain text: a line diff
// of two documents that big is slow and unreadable anyway.
export const HUGE = 200 * 1024

const MODES = new Set(['text', 'json', 'subset'])

// parseMismatchText reads an error in the "<what> mismatch[:]\nexpected:
// X\nactual: Y" shape, or returns null. Only the websocket's "JSON mismatch"
// compared JSON; every other site of this shape compared text.
export function parseMismatchText(error) {
  const m = /^([^\n]*?) ?mismatch:?\nexpected: ([\s\S]*?)\nactual: ([\s\S]*)$/.exec(String(error ?? ''))
  if (!m) return null
  return { what: m[1], expected: m[2], actual: m[3], mode: m[1] === 'JSON' ? 'json' : 'text' }
}

// parseDoc parses text as a JSON object or array; scalars stay text.
export function parseDoc(text) {
  const t = String(text ?? '').trim()
  if (!/^[[{]/.test(t)) return { ok: false }
  try {
    return { ok: true, value: JSON.parse(t) }
  } catch {
    return { ok: false }
  }
}

const isObj = (v) => v !== null && typeof v === 'object' && !Array.isArray(v)
const has = (o, k) => Object.prototype.hasOwnProperty.call(o, k)

// childPath is the JSONPath of key k under path: $.a.b, or $["a b"].
export const childPath = (path, k) => (/^[A-Za-z_$][\w$]*$/.test(k) ? `${path}.${k}` : `${path}[${JSON.stringify(k)}]`)

// under says whether path is p or inside it.
export const under = (path, p) => path === p || path.startsWith(`${p}.`) || path.startsWith(`${p}[`)

// printJSON pretty-prints value with keys sorted, two spaces deep, as lines
// that each know the JSONPath of the value they belong to.
export function printJSON(value) {
  const lines = []
  const walk = (v, path, depth, prefix, comma) => {
    const pad = '  '.repeat(depth)
    const open = (o, c, entries) => {
      if (!entries.length) { lines.push({ text: `${pad}${prefix}${o}${c}${comma}`, path }); return }
      lines.push({ text: `${pad}${prefix}${o}`, path })
      entries.forEach(([p, val, pre], i) => walk(val, p, depth + 1, pre, i < entries.length - 1 ? ',' : ''))
      lines.push({ text: `${pad}${c}${comma}`, path })
    }
    if (Array.isArray(v)) open('[', ']', v.map((x, i) => [`${path}[${i}]`, x, '']))
    else if (isObj(v)) open('{', '}', Object.keys(v).sort().map((k) => [childPath(path, k), v[k], `${JSON.stringify(k)}: `]))
    else lines.push({ text: `${pad}${prefix}${JSON.stringify(v)}${comma}`, path })
  }
  walk(value, '$', 0, '', '')
  return lines
}

// checkMatcher applies one of tomato's "@" matchers (internal/jsonmatch) to
// actual: null when it matches, else why not. undefined when `m` is not a
// matcher at all.
export function checkMatcher(m, actual) {
  const str = typeof actual === 'string'
  const num = typeof actual === 'number'
  const arg = (p) => (m.startsWith(p) ? m.slice(p.length) : null)
  let v
  if ((v = arg('@regex:')) != null) {
    if (!str) return 'not a string'
    try { return new RegExp(v).test(actual) ? null : 'does not match' } catch { return 'invalid pattern' }
  }
  if ((v = arg('@contains:')) != null) return !str ? 'not a string' : actual.includes(v) ? null : 'does not contain it'
  if ((v = arg('@startswith:')) != null) return !str ? 'not a string' : actual.startsWith(v) ? null : 'does not start with it'
  if ((v = arg('@endswith:')) != null) return !str ? 'not a string' : actual.endsWith(v) ? null : 'does not end with it'
  for (const [p, ok] of [['@gte:', (a, b) => a >= b], ['@gt:', (a, b) => a > b], ['@lte:', (a, b) => a <= b], ['@lt:', (a, b) => a < b]]) {
    if ((v = arg(p)) != null) return !num ? 'not a number' : ok(actual, Number(v)) ? null : 'out of range'
  }
  if ((v = arg('@len:')) != null) {
    const len = str || Array.isArray(actual) ? actual.length : isObj(actual) ? Object.keys(actual).length : null
    return len == null ? 'has no length' : len === Number(v) ? null : `length ${len}`
  }
  const empty = (a) => a == null || a === '' || (Array.isArray(a) && !a.length) || (isObj(a) && !Object.keys(a).length)
  switch (m) {
    case '@string': return str ? null : 'not a string'
    case '@number': return num ? null : 'not a number'
    case '@boolean': return typeof actual === 'boolean' ? null : 'not a boolean'
    case '@array': return Array.isArray(actual) ? null : 'not an array'
    case '@object': return isObj(actual) ? null : 'not an object'
    case '@any': return null
    case '@null': return actual === null ? null : 'not null'
    case '@notnull': return actual !== null ? null : 'null'
    case '@empty': return empty(actual) ? null : 'not empty'
    case '@notempty': return !empty(actual) ? null : 'empty'
    default: return m.startsWith('@') ? 'unknown matcher' : undefined
  }
}

// Scalars compare the way jsonmatch.Compare does — by their printed form, so
// 2 and "2" match — except in a text comparison, where they must be equal.
const sameScalar = (e, a, mode) => {
  if (mode === 'text' || e === null || a === null) return e === a
  return String(e) === String(a)
}

// pathDiffs lists where actual differs from expected under mode, as
// { path, kind, expected, actual }. kind is 'changed', 'missing',
// 'unexpected', 'type', 'length' or 'matcher'. In subset mode a key only
// actual has is not a difference; it goes in `extras` instead.
export function pathDiffs(expected, actual, mode) {
  const diffs = []
  const extras = []
  const walk = (e, a, path) => {
    if (mode !== 'text' && typeof e === 'string' && e.startsWith('@')) {
      const why = checkMatcher(e, a)
      if (why) diffs.push({ path, kind: 'matcher', expected: e, actual: a, why })
      return
    }
    if (isObj(e)) {
      if (!isObj(a)) { diffs.push({ path, kind: 'type', expected: e, actual: a }); return }
      // Both key sets, sorted, so differences come out in the order the
      // printed documents show them.
      for (const k of [...new Set([...Object.keys(e), ...Object.keys(a)])].sort()) {
        const p = childPath(path, k)
        if (!has(a, k)) diffs.push({ path: p, kind: 'missing', expected: e[k] })
        else if (has(e, k)) walk(e[k], a[k], p)
        else if (mode === 'subset') extras.push({ path: p, kind: 'unexpected', actual: a[k] })
        else diffs.push({ path: p, kind: 'unexpected', actual: a[k] })
      }
      return
    }
    if (Array.isArray(e)) {
      if (!Array.isArray(a)) { diffs.push({ path, kind: 'type', expected: e, actual: a }); return }
      if (e.length !== a.length) diffs.push({ path, kind: 'length', expected: e, actual: a })
      for (let i = 0; i < Math.min(e.length, a.length); i++) walk(e[i], a[i], `${path}[${i}]`)
      return
    }
    if (isObj(a) || Array.isArray(a)) diffs.push({ path, kind: 'type', expected: e, actual: a })
    else if (!sameScalar(e, a, mode)) diffs.push({ path, kind: 'changed', expected: e, actual: a })
  }
  walk(expected, actual, '$')
  return { diffs, extras }
}

const clip = (s, n = 48) => (s.length > n ? `${s.slice(0, n - 1)}…` : s)
const kindOf = (v) => (Array.isArray(v) ? 'an array' : isObj(v) ? 'an object' : v === null ? 'null' : `a ${typeof v}`)
export const fmtValue = (v) => (Array.isArray(v) ? `[${v.length} items]` : isObj(v) ? `{${Object.keys(v).length} keys}` : clip(JSON.stringify(v) ?? String(v)))

// describe is one difference as a line: `$.status: expected "created", got "pending"`.
export function describe(d) {
  switch (d.kind) {
    case 'missing': return `${d.path}: missing, expected ${fmtValue(d.expected)}`
    case 'unexpected': return `${d.path}: not expected, got ${fmtValue(d.actual)}`
    case 'type': return `${d.path}: expected ${kindOf(d.expected)}, got ${kindOf(d.actual)}`
    case 'length': return `${d.path}: expected ${d.expected.length} items, got ${d.actual.length}`
    case 'matcher': return `${d.path}: expected ${d.expected}, got ${fmtValue(d.actual)} (${d.why})`
    default: return `${d.path}: expected ${fmtValue(d.expected)}, got ${fmtValue(d.actual)}`
  }
}

// summarize joins the first few differences into one line.
export function summarize(lines, max = 3) {
  if (!lines.length) return ''
  const shown = lines.slice(0, max).join('; ')
  return lines.length > max ? `${shown}; and ${lines.length - max} more` : shown
}

// A line's comparison key: its text without the trailing comma, so the last
// key of an object does not differ from itself just because the other side
// has one more key after it.
const lineKey = (l) => l.text.replace(/,$/, '')

// textProblems describes each changed row of a text diff, by line number.
function textProblems(rows, eLines, aLines) {
  const q = (s) => clip(JSON.stringify(s))
  return rows.filter((r) => !r.same).map((r) => {
    if (r.left != null && r.right != null) return `line ${r.left + 1}: expected ${q(eLines[r.left])}, got ${q(aLines[r.right])}`
    if (r.left != null) return `line ${r.left + 1}: missing ${q(eLines[r.left])}`
    return `actual line ${r.right + 1}: not expected ${q(aLines[r.right])}`
  })
}

// analyze is everything the view needs for one failed comparison:
//   mode     — 'text' | 'json' | 'subset'
//   json     — both sides shown as key-sorted JSON
//   huge     — too big for a diff: shown as plain text, with the summary
//   problems — one line per difference that made the step fail
//   summary  — the first few of those on one line
//   extras   — subset only: keys only the actual value has, which are fine
//   left, right — the lines shown on each side (strings)
//   rows     — side-by-side rows { same, left, right, ls, rs }; ls / rs is
//              'err', 'extra' (subset: only in actual, allowed) or 'ok'
//              (differs, but the comparison accepted it) on a changed row
//   note     — anything a reader should know about how it was compared
export function analyze(m) {
  if (!m) return null
  const mode = MODES.has(m.mode) ? m.mode : 'text'
  const expected = String(m.expected ?? '')
  const actual = String(m.actual ?? '')
  const huge = expected.length > HUGE || actual.length > HUGE
  const e = parseDoc(expected)
  const a = parseDoc(actual)
  const base = { what: m.what ?? '', mode, huge, expected, actual, extras: [], note: '' }

  if (e.ok && a.ok) {
    const { diffs, extras } = pathDiffs(e.value, a.value, mode)
    const problems = diffs.map(describe)
    if (mode === 'text' && !diffs.length) {
      // Same document, different text: key order, spacing or how a number
      // is written. Pretty-printing would hide that, so diff the text.
      return textResult({ ...base, note: 'Both sides are the same JSON, but this step compares text: the difference is key order, spacing or how a value is written.' })
    }
    const left = printJSON(e.value)
    const right = printJSON(a.value)
    const note = mode === 'text' ? 'This step compares text; both sides are JSON, so they are shown key-sorted and pretty-printed.' : ''
    const result = {
      ...base, json: true, extras, note,
      problems: problems.length ? problems : ['The values differ, but no JSON path does: see both sides.'],
      left: left.map((l) => l.text), right: right.map((l) => l.text),
    }
    result.summary = summarize(result.problems)
    if (huge) return result

    const extraPaths = extras.map((d) => d.path)
    const errPaths = diffs.map((d) => d.path)
    const isExtra = (j) => extraPaths.some((p) => under(right[j].path, p))
    const isErr = (path) => !diffs.length || errPaths.some((p) => under(path, p))
    const ops = diffLines(left.map(lineKey), right.map(lineKey))
    result.rows = sideBySide(ops, mode === 'subset' ? isExtra : () => false).map((r) => {
      if (r.same) return r
      const ls = r.left == null ? null : isErr(left[r.left].path) ? 'err' : 'ok'
      const rs = r.right == null ? null : (mode === 'subset' && isExtra(r.right)) ? 'extra' : isErr(right[r.right].path) ? 'err' : 'ok'
      return { ...r, ls, rs }
    })
    return result
  }
  return textResult(base)
}

function textResult(base) {
  const left = base.expected.split('\n')
  const right = base.actual.split('\n')
  const result = { ...base, json: false, left, right }
  if (base.huge) {
    let i = 0
    while (i < left.length && i < right.length && left[i] === right[i]) i++
    result.problems = [`first difference at line ${i + 1}`]
  } else {
    result.rows = sideBySide(diffLines(left, right)).map((r) => (r.same ? r : { ...r, ls: r.left == null ? null : 'err', rs: r.right == null ? null : 'err' }))
    result.problems = textProblems(result.rows, left, right)
    if (!result.problems.length) result.problems = ['The texts differ only in whitespace at their ends.']
  }
  result.summary = summarize(result.problems)
  return result
}

// segments is what the diff view draws: every row when expanded, else the
// rows with unchanged runs folded (linediff.collapse).
export const segments = (result, expanded) => (expanded || !result.rows ? [{ rows: result.rows ?? [] }] : collapse(result.rows))
