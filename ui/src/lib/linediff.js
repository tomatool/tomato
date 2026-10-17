// Line diff, side-by-side pairing and folding for the failed-comparison view
// (components/Mismatch.jsx). Pure functions over arrays of strings, so the
// shape of a diff is tested without a browser (linediff.test.js).

// Above this many edits the diff stops looking for the shortest script and
// reports the rest as one replaced block: two unrelated documents are not
// worth an O(D²) search, and what differs is then obvious anyway.
const MAX_EDITS = 1000

// diffLines is the shortest edit script from a to b (Myers), as
// [{ op: 'same' | 'del' | 'add', i, j }] where i indexes a and j indexes b.
export function diffLines(a, b) {
  let lo = 0
  while (lo < a.length && lo < b.length && a[lo] === b[lo]) lo++
  let ea = a.length
  let eb = b.length
  while (ea > lo && eb > lo && a[ea - 1] === b[eb - 1]) { ea--; eb-- }

  const ops = []
  for (let i = 0; i < lo; i++) ops.push({ op: 'same', i, j: i })
  ops.push(...myers(a, b, lo, ea, lo, eb))
  for (let k = 0; ea + k < a.length; k++) ops.push({ op: 'same', i: ea + k, j: eb + k })
  return ops
}

function myers(a, b, a0, a1, b0, b1) {
  const n = a1 - a0
  const m = b1 - b0
  const replaced = () => [
    ...Array.from({ length: n }, (_, x) => ({ op: 'del', i: a0 + x })),
    ...Array.from({ length: m }, (_, y) => ({ op: 'add', j: b0 + y })),
  ]
  if (!n || !m) return replaced()

  const limit = Math.min(n + m, MAX_EDITS)
  const off = limit + 1
  const v = new Int32Array(2 * limit + 3)
  const trace = []
  for (let d = 0; d <= limit; d++) {
    trace.push(v.slice(off - d - 1, off + d + 2))
    for (let k = -d; k <= d; k += 2) {
      let x = (k === -d || (k !== d && v[off + k - 1] < v[off + k + 1])) ? v[off + k + 1] : v[off + k - 1] + 1
      let y = x - k
      while (x < n && y < m && a[a0 + x] === b[b0 + y]) { x++; y++ }
      v[off + k] = x
      if (x >= n && y >= m) return backtrack(trace, n, m, a0, b0)
    }
  }
  return replaced()
}

function backtrack(trace, n, m, a0, b0) {
  const ops = []
  let x = n
  let y = m
  for (let d = trace.length - 1; d >= 0; d--) {
    const row = trace[d] // v[k] for k in -d-1 .. d+1
    const at = (k) => row[k + d + 1]
    const k = x - y
    const pk = (k === -d || (k !== d && at(k - 1) < at(k + 1))) ? k + 1 : k - 1
    const px = at(pk)
    const py = px - pk
    while (x > px && y > py) { ops.push({ op: 'same', i: a0 + x - 1, j: b0 + y - 1 }); x--; y-- }
    if (d > 0) {
      if (x === px) ops.push({ op: 'add', j: b0 + y - 1 })
      else ops.push({ op: 'del', i: a0 + x - 1 })
    }
    x = px
    y = py
  }
  return ops.reverse()
}

// sideBySide turns an edit script into rows of { same, left, right }, where
// left is an index into a (or null) and right an index into b. Within a block
// of edits, deletions and additions are paired in order; `unpaired(j)` keeps
// an added line on a row of its own — the subset view uses it so a key only
// the actual value has is never lined up against an expected key it has
// nothing to do with.
export function sideBySide(ops, unpaired = () => false) {
  const rows = []
  for (let p = 0; p < ops.length;) {
    if (ops[p].op === 'same') {
      rows.push({ same: true, left: ops[p].i, right: ops[p].j })
      p++
      continue
    }
    const dels = []
    const adds = []
    while (p < ops.length && ops[p].op !== 'same') {
      if (ops[p].op === 'del') dels.push(ops[p].i)
      else adds.push(ops[p].j)
      p++
    }
    let d = 0
    for (const j of adds) {
      if (unpaired(j) || d >= dels.length) rows.push({ same: false, left: null, right: j })
      else rows.push({ same: false, left: dels[d++], right: j })
    }
    for (; d < dels.length; d++) rows.push({ same: false, left: dels[d], right: null })
  }
  return rows
}

// collapse folds runs of unchanged rows, keeping `context` rows next to each
// change. It returns segments: { rows } to show and { fold: rows } to show as
// "N unchanged lines". A run is folded only when that hides two or more rows,
// and nothing is folded when nothing changed.
export function collapse(rows, context = 3) {
  if (!rows.some((r) => !r.same)) return [{ rows }]
  const out = []
  const push = (kind, rs) => {
    if (!rs.length) return
    const last = out[out.length - 1]
    if (kind === 'rows' && last?.rows) last.rows.push(...rs)
    else out.push(kind === 'rows' ? { rows: [...rs] } : { fold: rs })
  }
  for (let p = 0; p < rows.length;) {
    if (!rows[p].same) {
      push('rows', [rows[p]])
      p++
      continue
    }
    let q = p
    while (q < rows.length && rows[q].same) q++
    const run = rows.slice(p, q)
    const head = p === 0 ? 0 : context
    const tail = q === rows.length ? 0 : context
    if (run.length - head - tail >= 2) {
      push('rows', run.slice(0, head))
      push('fold', run.slice(head, run.length - tail))
      push('rows', run.slice(run.length - tail))
    } else {
      push('rows', run)
    }
    p = q
  }
  return out
}

// inlineSpan is where two versions of one line differ: the lengths of their
// common prefix and suffix, so the view can mark only the middle.
export function inlineSpan(a, b) {
  let pre = 0
  while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++
  let suf = 0
  while (suf < a.length - pre && suf < b.length - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++
  return { pre, suf }
}
