import { tokenizeStep, stepKey } from '../lib/steps.jsx'
import { fmtDur } from '../lib/format'

function DataTable({ rows }) {
  const [head, ...body] = rows
  return (
    <table className="dtable">
      <tbody>
        <tr>{head.map((c, i) => <th key={i}>{c}</th>)}</tr>
        {body.map((r, i) => (
          <tr key={i}>{r.map((c, j) => <td key={j}>{tokenizeStep(c)}</td>)}</tr>
        ))}
      </tbody>
    </table>
  )
}

export function StepRow({ step, scenarioName, index, runState }) {
  const k = stepKey(scenarioName, index)
  // `unmatched` is known from parsing alone, so a step with no definition
  // reads as undefined before the suite has ever run.
  const status = runState.stepStatus[k] ?? (step.unmatched ? 'undefined' : '')
  const dur = runState.stepDur[k]
  const error = runState.stepError[k]
  const isAnd = /^(and|but)$/i.test(String(step.keyword).trim())

  return (
    <div
      className="step"
      data-phase={step.phase || undefined}
      data-and={isAnd ? 'true' : undefined}
      data-status={status || undefined}
      aria-current={status === 'running' ? 'true' : undefined}
    >
      <i className="pip" data-status={status} />
      <span className="step-kw">{String(step.keyword).trim()}</span>
      <span className="step-text">{tokenizeStep(step.text)}</span>
      <span className="step-res">
        {dur != null && <span className="step-dur">{fmtDur(dur)}</span>}
        {step.resource && (
          <span className="res">
            <span className="res-name">{step.resource}</span>
            {step.resourceType && <span className="res-type">{step.resourceType}</span>}
          </span>
        )}
      </span>

      {step.description && <div className="step-desc">{step.description}</div>}

      {step.docString && (
        <div className="step-attach">
          <pre className="docstring" data-lang={step.docStringLang || undefined}>{step.docString}</pre>
        </div>
      )}

      {step.table?.length > 0 && (
        <div className="step-attach"><DataTable rows={step.table} /></div>
      )}

      {status === 'failed' && error && (
        <div className="step-attach callout" data-status="failed">
          <span className="callout-title">step failed</span>
          <div className="callout-body">{error}</div>
        </div>
      )}

      {status === 'undefined' && (
        <div className="step-attach callout" data-status="undefined">
          <span className="callout-title">no step definition matches this text</span>
          <div className="callout-body">Run <code>tomato steps</code> to list the available steps.</div>
        </div>
      )}
    </div>
  )
}
