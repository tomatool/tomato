import { describe, expect, it } from 'vitest'
import { collapse, diffLines, inlineSpan, sideBySide } from './linediff.js'

// Apply an edit script to a and return what it produces; it must be b.
const apply = (a, b, ops) => ops.filter((o) => o.op !== 'del').map((o) => (o.op === 'same' ? a[o.i] : b[o.j]))

describe('diffLines', () => {
  it('finds the shortest script', () => {
    const a = ['{', '"a": 1', '"b": 2', '"c": 3', '}']
    const b = ['{', '"a": 1', '"b": 9', '"c": 3', '"d": 4', '}']
    const ops = diffLines(a, b)
    expect(apply(a, b, ops)).toEqual(b)
    expect(ops.filter((o) => o.op !== 'same')).toEqual([
      { op: 'del', i: 2 }, { op: 'add', j: 2 }, { op: 'add', j: 4 },
    ])
  })

  it('handles empty sides and identical input', () => {
    expect(diffLines([], ['x'])).toEqual([{ op: 'add', j: 0 }])
    expect(diffLines(['x'], [])).toEqual([{ op: 'del', i: 0 }])
    expect(diffLines(['x', 'y'], ['x', 'y']).every((o) => o.op === 'same')).toBe(true)
  })

  it('round-trips unrelated inputs', () => {
    const a = 'the quick brown fox jumps over the lazy dog'.split(' ')
    const b = 'a lazy cat sleeps over the quick brown dog'.split(' ')
    expect(apply(a, b, diffLines(a, b))).toEqual(b)
  })
})

describe('sideBySide', () => {
  it('pairs deletions with additions and keeps leftovers on their own rows', () => {
    const rows = sideBySide(diffLines(['a', 'b', 'c'], ['a', 'B', 'X', 'c']))
    expect(rows).toEqual([
      { same: true, left: 0, right: 0 },
      { same: false, left: 1, right: 1 },
      { same: false, left: null, right: 2 },
      { same: true, left: 2, right: 3 },
    ])
  })

  it('leaves an unpaired addition alone', () => {
    const rows = sideBySide(diffLines(['a', 'b'], ['a', 'x', 'B']), (j) => j === 1)
    expect(rows.slice(1)).toEqual([
      { same: false, left: null, right: 1 },
      { same: false, left: 1, right: 2 },
    ])
  })
})

describe('collapse', () => {
  const same = (n) => Array.from({ length: n }, (_, i) => ({ same: true, left: i, right: i }))
  const change = { same: false, left: 99, right: 99 }

  it('folds long unchanged runs, keeping context around each change', () => {
    const rows = [...same(10), change, ...same(10), change, ...same(2)]
    const segs = collapse(rows, 3)
    expect(segs.map((s) => (s.fold ? `fold ${s.fold.length}` : `rows ${s.rows.length}`))).toEqual([
      'fold 7', 'rows 7', 'fold 4', 'rows 6',
    ])
    // Nothing is lost: the segments are the rows, in order.
    expect(segs.flatMap((s) => s.fold ?? s.rows)).toEqual(rows)
  })

  it('does not fold a single line, or anything when nothing changed', () => {
    expect(collapse([...same(4), change, ...same(4)], 3).every((s) => !s.fold)).toBe(true)
    expect(collapse(same(50), 3)).toEqual([{ rows: same(50) }])
  })
})

it('inlineSpan marks the middle that differs', () => {
  expect(inlineSpan('  "status": "created",', '  "status": "pending",')).toEqual({ pre: 13, suf: 2 })
  expect(inlineSpan('abc', 'abc')).toEqual({ pre: 3, suf: 0 })
})
