import { Fragment, useCallback, useMemo, useState } from 'react'
import { analyze, parseMismatchText, segments } from '../lib/mismatch.js'
import { inlineSpan } from '../lib/linediff.js'

const MODE_NOTE = {
  subset: 'compared as a JSON subset: keys only the actual value has are allowed',
  json: 'compared as exact JSON: every key on both sides',
  text: 'compared as text',
}

// What each changed cell is, in words and a sign as well as a colour.
const MARK = {
  err: { sign: { l: '−', r: '+' }, label: { l: 'expected, not found', r: 'differs from expected' } },
  extra: { sign: { r: '·' }, label: { r: 'only in actual; a subset match allows it' } },
  ok: { sign: { l: '≈', r: '≈' }, label: { l: 'differs, but the comparison accepts it', r: 'differs, but the comparison accepts it' } },
}

// useMismatch is the analysis of a failed step: from the runner's mismatch
// when it sent one, else from an error in the "expected:/actual:" shape.
// null when the error is neither.
export function useMismatch(error, mismatch) {
  return useMemo(() => {
    try {
      return analyze(mismatch ?? parseMismatchText(error))
    } catch {
      return null
    }
  }, [error, mismatch])
}

function CopyButton({ text, label }) {
  const [copied, setCopied] = useState(false)
  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      return
    }
    setCopied(true)
    setTimeout(() => setCopied(false), 1200)
  }, [text])
  return (
    <button
      className="btn btn--ghost btn--sm"
      type="button"
      data-copied={copied ? 'true' : undefined}
      aria-label={`Copy the ${label} value`}
      title={`Copy the ${label} value`}
      onClick={copy}
    >
      {copied ? 'copied' : 'copy'}
    </button>
  )
}

// A cell carries both line numbers: side by side each shows its own, and
// stacked an unchanged line shows where it sits in both documents.
function Cell({ side, n, nl, nr, text, mark, other }) {
  if (n == null) return <div className={`mm-cell mm-cell--${side}`} data-empty="true" />
  const m = mark && MARK[mark]
  let body = text
  // On a paired error row, mark just the part of the line that differs.
  if (mark === 'err' && other != null) {
    const { pre, suf } = inlineSpan(text, other)
    const mid = text.slice(pre, text.length - suf)
    if (mid) body = <>{text.slice(0, pre)}<mark className="mm-hl">{mid}</mark>{text.slice(text.length - suf)}</>
  }
  return (
    <div className={`mm-cell mm-cell--${side}`} data-mark={mark || undefined} title={m?.label[side[0]]}>
      <span className="mm-n mm-n--l">{nl != null ? nl + 1 : ''}</span>
      <span className="mm-n mm-n--r">{nr != null ? nr + 1 : ''}</span>
      <span className="mm-sign" aria-hidden="true">{m?.sign[side[0]] ?? ''}</span>
      {m && <span className="sr-only">{m.label[side[0]]}: </span>}
      <span className="mm-text">{body}</span>
    </div>
  )
}

function Row({ r, a }) {
  const lt = r.left != null ? a.left[r.left] : null
  const rt = r.right != null ? a.right[r.right] : null
  const paired = r.ls === 'err' && r.rs === 'err'
  return (
    <div className="mm-row" data-same={r.same ? 'true' : undefined}>
      <Cell side="left" n={r.left} nl={r.left} nr={r.same ? r.right : null} text={lt} mark={r.ls} other={paired ? rt : null} />
      <Cell side="right" n={r.right} nl={r.same ? r.left : null} nr={r.right} text={rt} mark={r.rs} other={paired ? lt : null} />
    </div>
  )
}

// MismatchView draws a failed comparison: what is wrong on one line, then
// expected and actual side by side with the differing lines marked and the
// unchanged ones folded. It stacks the two sides when its box is narrow.
export function MismatchView({ a, error }) {
  const [expanded, setExpanded] = useState(false)
  const [open, setOpen] = useState(() => new Set())
  const segs = useMemo(() => (a.rows ? segments(a, expanded) : []), [a, expanded])
  const leftText = a.left.join('\n')
  const rightText = a.right.join('\n')
  const count = a.problems.length
  const errorText = String(error ?? '')

  return (
    <div className="mm" data-mode={a.mode}>
      <div className="mm-summary">
        <span className="mm-badge">{count === 1 ? '1 difference' : `${count} differences`}</span>
        <span className="mm-what" title={a.problems.join('\n')}>{a.summary}</span>
      </div>
      <div className="mm-bar">
        <span className="mm-mode">
          {a.what ? `${a.what} ` : ''}{MODE_NOTE[a.mode]}
          {a.mode === 'subset' && a.extras.length > 0 && ` (${a.extras.length} here)`}
        </span>
        {a.rows && (
          <button className="btn btn--ghost btn--sm" type="button" aria-pressed={expanded} onClick={() => setExpanded((v) => !v)}>
            {expanded ? 'Collapse' : 'Expand'}
          </button>
        )}
      </div>
      {a.note && <div className="mm-note">{a.note}</div>}

      {a.rows ? (
        <div className={`mm-diff${expanded ? ' mm-diff--full' : ''}`} role="group" aria-label="Expected and actual, side by side">
          <div className="mm-row mm-head">
            <div className="mm-cell mm-cell--left"><span className="mm-side">expected</span><CopyButton text={leftText} label="expected" /></div>
            <div className="mm-cell mm-cell--right"><span className="mm-side">actual</span><CopyButton text={rightText} label="actual" /></div>
          </div>
          {segs.map((s, i) => (s.fold && !open.has(i) ? (
            <button
              key={i}
              className="mm-fold"
              type="button"
              onClick={() => setOpen((o) => new Set(o).add(i))}
            >
              {s.fold.length} unchanged lines
            </button>
          ) : (
            <Fragment key={i}>{(s.fold ?? s.rows).map((r, j) => <Row key={j} r={r} a={a} />)}</Fragment>
          )))}
        </div>
      ) : (
        <div className="mm-plain">
          <div className="mm-plain-head"><span className="mm-side">expected</span><CopyButton text={leftText} label="expected" /></div>
          <pre className="docstring mm-pre">{leftText}</pre>
          <div className="mm-plain-head"><span className="mm-side">actual</span><CopyButton text={rightText} label="actual" /></div>
          <pre className="docstring mm-pre">{rightText}</pre>
        </div>
      )}

      {errorText && (
        <details className="mm-raw">
          <summary>error text</summary>
          <pre className="mm-pre">{errorText}</pre>
        </details>
      )}
    </div>
  )
}

// FailureBody is the body of a failed step's callout: the diff when the
// failure was a comparison, the error text otherwise.
export function FailureBody({ error, mismatch }) {
  const a = useMismatch(error, mismatch)
  if (!a) return <div className="callout-body">{error}</div>
  return <MismatchView a={a} error={error} />
}
