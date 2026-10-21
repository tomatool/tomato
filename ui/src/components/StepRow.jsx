import { tokenizeStep } from '../lib/steps.jsx'
import { stepRun } from '../lib/run.js'
import { stepKind, KIND_LABEL } from '../lib/kind'
import { fmtDur } from '../lib/format'
import { screenshotURL } from '../lib/screenshots.js'
import { FailureBody } from './Mismatch.jsx'

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

// The pictures a step took — `saves screenshot`, or the page when it failed
// — as thumbnails that open full size. They come from the run, matched to
// the step by the runner, so a passed step shows what it saved and a failed
// one what each person with a page open was looking at.
function StepShots({ shots, onOpen }) {
  if (!shots?.length || !onOpen) return null
  return (
    <div className="shot-chips">
      {shots.map((s) => (
        <button className="shot-chip" type="button" key={s.path} title={s.path} onClick={() => onOpen(s.path)}>
          <img className="shot-chip-img" src={screenshotURL(s.path)} alt="" loading="lazy" decoding="async" />
          <span className="shot-chip-label"><i className="ico-shot" />{s.resource}</span>
        </button>
      ))}
    </div>
  )
}

// What the network was like while the step ran. A step that ran while a
// fault was on gets a badge naming the networks, with the faults on hover; a
// step during which something changed on the network — a fault applied or
// healed, a connection failed — lists it with the time it happened. A step
// that failed on a bad network says which faults in its error, too.
const fmtAt = (at) => {
  const d = new Date(at)
  return Number.isNaN(d.getTime()) ? '' : d.toTimeString().slice(0, 8) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

// The active lines that name a fault, not the connection counts.
const faultLines = (network) => (network?.active ?? []).filter((l) => !/ has seen \d/.test(l))

function NetworkBadge({ network }) {
  const lines = faultLines(network)
  if (!lines.length) return null
  const names = [...new Set(lines.map((l) => l.split(' ')[0]))]
  return (
    <span className="net-badge" title={(network.active ?? []).join('\n')}>
      <i className="net-ico" aria-hidden="true">⇄</i>{names.join(', ')}
    </span>
  )
}

function StepNetwork({ network }) {
  const events = network?.events ?? []
  if (!events.length && !faultLines(network).length) return null
  return (
    <div className="step-attach net">
      <NetworkBadge network={network} />
      {events.length > 0 && <ul className="net-events">
        {events.map((e, i) => (
          <li key={i} data-kind={e.kind}>
            <span className="net-at">{fmtAt(e.at)}</span>
            <span className="net-res">{e.resource}</span>
            <span className="net-kind">{e.kind}</span>
            <span className="net-detail">{e.detail}</span>
          </li>
        ))}
      </ul>}
    </div>
  )
}

// `index` is the step's position in the pickle, which is how the runner
// reports it: a scenario's own step sits after the Background (lib/run.js).
// `onScreenshot` opens one of the step's screenshots at full size.
export function StepRow({ step, scenarioName, index, runState, background, onScreenshot }) {
  const kind = stepKind(step, { background })
  const { status, dur, error, mismatch, screenshots, network } = stepRun(runState, scenarioName, index, step)
  const isAnd = /^(and|but)$/i.test(String(step.keyword).trim())
  const failedWithError = status === 'failed' && error

  return (
    <div
      className="step"
      data-phase={step.phase || undefined}
      data-kind={kind || undefined}
      data-and={isAnd ? 'true' : undefined}
      data-status={status || undefined}
      aria-current={status === 'running' ? 'true' : undefined}
      data-network={faultLines(network).length ? 'faulty' : undefined}
    >
      <i className="pip" data-status={status} />
      <span className="step-kw">{String(step.keyword).trim()}</span>
      <span className="step-text">{tokenizeStep(step.text)}</span>
      <span className="step-res">
        {dur != null && <span className="step-dur">{fmtDur(dur)}</span>}
        {kind && <span className="kind" data-kind={kind}>{KIND_LABEL[kind]}</span>}
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

      {failedWithError && (
        <div className="step-attach callout" data-status="failed">
          <span className="callout-title">step failed</span>
          <FailureBody error={error} mismatch={mismatch} />
          <StepShots shots={screenshots} onOpen={onScreenshot} />
        </div>
      )}

      {!failedWithError && screenshots?.length > 0 && (
        <div className="step-attach"><StepShots shots={screenshots} onOpen={onScreenshot} /></div>
      )}

      <StepNetwork network={network} />

      {status === 'undefined' && (
        <div className="step-attach callout" data-status="undefined">
          <span className="callout-title">no step definition matches this text</span>
          <div className="callout-body">Run <code>tomato steps</code> to list the available steps.</div>
        </div>
      )}
    </div>
  )
}
