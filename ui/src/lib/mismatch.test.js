import { describe, expect, it } from 'vitest'
import { analyze, checkMatcher, parseMismatchText, pathDiffs, printJSON, segments, HUGE } from './mismatch.js'

const kinds = (r) => r.rows.filter((x) => !x.same).map((x) => [
  x.left == null ? null : r.left[x.left].trim(), x.ls ?? null,
  x.right == null ? null : r.right[x.right].trim(), x.rs ?? null,
])

describe('subset mode', () => {
  it('extra keys in actual are not differences', () => {
    const r = analyze({
      what: 'response JSON', mode: 'subset',
      expected: '{"sku": "TMT-01", "qty": 2}',
      actual: '{"id":"1","sku":"TMT-01","qty":2,"email":"ada@example.com"}',
    })
    expect(pathDiffs({ sku: 'TMT-01', qty: 2 }, { id: '1', sku: 'TMT-01', qty: 2 }, 'subset').diffs).toEqual([])
    expect(r.extras.map((d) => d.path)).toEqual(['$.email', '$.id'])
    // The extra keys show, dimmed, and nothing is an error.
    expect(kinds(r)).toEqual([
      [null, null, '"email": "ada@example.com",', 'extra'],
      [null, null, '"id": "1",', 'extra'],
    ])
  })

  it('points at one wrong value deep in a nested object, not at the extras', () => {
    const r = analyze({
      mode: 'subset',
      expected: '{"order": {"lines": {"first": {"status": "created"}}}}',
      actual: '{"id": 7, "order": {"total": 3, "lines": {"first": {"sku": "A", "status": "pending"}}}}',
    })
    expect(r.problems).toEqual(['$.order.lines.first.status: expected "created", got "pending"'])
    expect(r.summary).toBe('$.order.lines.first.status: expected "created", got "pending"')
    const errs = kinds(r).filter((k) => k[1] === 'err' || k[3] === 'err')
    expect(errs).toEqual([['"status": "created"', 'err', '"status": "pending"', 'err']])
    expect(kinds(r).filter((k) => k[3] === 'extra').map((k) => k[2])).toEqual(['"id": 7,', '"sku": "A",', '"total": 3'])
  })

  it('names a missing key with its path', () => {
    const r = analyze({ mode: 'subset', expected: '{"user": {"email": "ada@example.com", "name": "Ada"}}', actual: '{"user": {"name": "Ada"}}' })
    expect(r.problems).toEqual(['$.user.email: missing, expected "ada@example.com"'])
    expect(kinds(r)).toEqual([['"email": "ada@example.com",', 'err', null, null]])
  })

  it('honours matchers', () => {
    const { diffs } = pathDiffs({ id: '@number', name: '@string', at: '@regex:^2026-' }, { id: 1, name: 7, at: '2026-10-17' }, 'subset')
    expect(diffs).toEqual([{ path: '$.name', kind: 'matcher', expected: '@string', actual: 7, why: 'not a string' }])
    expect(checkMatcher('@len:2', [1, 2])).toBe(null)
    expect(checkMatcher('@gt:5', 3)).toBe('out of range')
    expect(checkMatcher('@nope', 3)).toBe('unknown matcher')
  })
})

describe('arrays', () => {
  it('reports a length difference and wrong elements by index', () => {
    const r = analyze({
      mode: 'subset',
      expected: '{"items": [{"sku": "A"}, {"sku": "B"}]}',
      actual: '{"items": [{"sku": "A", "qty": 1}, {"sku": "C"}, {"sku": "D"}]}',
    })
    expect(r.problems).toEqual([
      '$.items: expected 2 items, got 3',
      '$.items[1].sku: expected "B", got "C"',
    ])
    expect(r.extras.map((d) => d.path)).toEqual(['$.items[0].qty'])
  })
})

describe('exact modes', () => {
  it('exact JSON counts keys only in actual as differences', () => {
    const r = analyze({ what: 'JSON', mode: 'json', expected: '{"a":1}', actual: '{"a":1,"b":2}' })
    expect(r.problems).toEqual(['$.b: not expected, got 2'])
    // "a": 1 gains a comma on the actual side; that is not a difference.
    expect(kinds(r)).toEqual([[null, null, '"b": 2', 'err']])
  })

  it('a text comparison of JSON lists every difference', () => {
    const r = analyze({ what: 'message', mode: 'text', expected: '{"status": "created", "qty": 2}', actual: '{"status": "pending", "qty": 2, "id": 1}' })
    expect(r.json).toBe(true)
    expect(r.problems).toEqual(['$.id: not expected, got 1', '$.status: expected "created", got "pending"'])
  })

  it('text that is the same JSON in another order is diffed as text', () => {
    const r = analyze({ mode: 'text', expected: '{"a":1,"b":2}', actual: '{"b":2,"a":1}' })
    expect(r.json).toBe(false)
    expect(r.note).toMatch(/key order/)
    expect(r.problems).toEqual(['line 1: expected "{\\"a\\":1,\\"b\\":2}", got "{\\"b\\":2,\\"a\\":1}"'])
  })

  it('a text mismatch shows each changed line', () => {
    const r = analyze({ what: 'stdout', mode: 'text', expected: 'one\ntwo\nthree', actual: 'one\n2\nthree\nfour' })
    expect(r.problems).toEqual(['line 2: expected "two", got "2"', 'actual line 4: not expected "four"'])
    expect(r.summary).toBe('line 2: expected "two", got "2"; actual line 4: not expected "four"')
  })
})

describe('fallbacks', () => {
  it('non-JSON values fall back to a line diff', () => {
    const r = analyze({ mode: 'json', expected: 'hello', actual: '{"a": 1}' })
    expect(r.json).toBe(false)
    expect(r.rows).toEqual([{ same: false, left: 0, right: 0, ls: 'err', rs: 'err' }])
  })

  it('huge values are plain text with a summary and no rows', () => {
    const big = `{"a": "${'x'.repeat(HUGE)}", "b": 1}`
    const r = analyze({ mode: 'subset', expected: '{"b": 2}', actual: big })
    expect(r.huge).toBe(true)
    expect(r.rows).toBeUndefined()
    expect(r.summary).toBe('$.b: expected 2, got 1')
    const t = analyze({ mode: 'text', expected: 'a\nb', actual: `a\nc${'x'.repeat(HUGE)}` })
    expect(t.problems).toEqual(['first difference at line 2'])
  })

  it('reads the error text of a runner that sent no mismatch', () => {
    const err = 'message mismatch:\nexpected: {"sku": "TMT-01", "qty": 2, "status": "created"}\nactual: {"id":"1","sku":"TMT-01","qty":2,"status":"pending"}'
    const m = parseMismatchText(err)
    expect(m).toEqual({
      what: 'message', mode: 'text',
      expected: '{"sku": "TMT-01", "qty": 2, "status": "created"}',
      actual: '{"id":"1","sku":"TMT-01","qty":2,"status":"pending"}',
    })
    expect(analyze(m).problems).toEqual(['$.id: not expected, got "1"', '$.status: expected "created", got "pending"'])
    expect(parseMismatchText('stdout mismatch\nexpected: a\nb\nactual: c')).toEqual({ what: 'stdout', mode: 'text', expected: 'a\nb', actual: 'c' })
    expect(parseMismatchText('JSON mismatch:\nexpected: {}\nactual: []').mode).toBe('json')
    expect(parseMismatchText('stdout does not contain "x"')).toBe(null)
  })
})

describe('printing and folding', () => {
  it('sorts keys and gives every line its path', () => {
    expect(printJSON({ b: [1, {}], a: { 'x y': null } })).toEqual([
      { text: '{', path: '$' },
      { text: '  "a": {', path: '$.a' },
      { text: '    "x y": null', path: '$.a["x y"]' },
      { text: '  },', path: '$.a' },
      { text: '  "b": [', path: '$.b' },
      { text: '    1,', path: '$.b[0]' },
      { text: '    {}', path: '$.b[1]' },
      { text: '  ]', path: '$.b' },
      { text: '}', path: '$' },
    ])
  })

  it('folds unchanged runs unless expanded', () => {
    const doc = (s) => JSON.stringify(Object.fromEntries(Array.from({ length: 30 }, (_, i) => [`k${String(i).padStart(2, '0')}`, i === 15 ? s : i])))
    const r = analyze({ mode: 'json', expected: doc('a'), actual: doc('b') })
    const folded = segments(r, false)
    expect(folded.map((s) => (s.fold ? `fold ${s.fold.length}` : `rows ${s.rows.length}`))).toEqual(['fold 13', 'rows 7', 'fold 12'])
    expect(segments(r, true)).toEqual([{ rows: r.rows }])
  })
})
