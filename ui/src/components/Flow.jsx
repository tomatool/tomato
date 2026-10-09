import { ResourceIcon } from '../lib/icons'
import { clip } from '../lib/format'
import { Peek } from './Peek.jsx'

export function Flow({ model, cursor, hot, onHot }) {
  const { lanes, msgs } = model
  const stateOf = (i) => (cursor < 0 ? undefined : i < cursor ? 'past' : i === cursor ? 'current' : 'future')

  return (
    <div className="seq" style={{ '--lanes': lanes.length }}>
      <div className="seq-heads">
        {lanes.map((l) => (
          <div className="seq-head" key={l.name} data-runner={l.runner ? 'true' : undefined}>
            <ResourceIcon type={l.icon} />
            <span className="node-name">{l.name}</span>
            <span className="node-type">{l.type}</span>
          </div>
        ))}
      </div>

      <div className="seq-body">
        <div className="seq-lines">{lanes.map((l) => <i className="seq-line" key={l.name} />)}</div>
        {msgs.map((m) => (
          <div
            className="msg"
            key={m.n}
            data-kind={m.kind}
            data-dir={m.dir}
            data-state={stateOf(m.idx)}
            data-status={m.status || undefined}
            data-hot={hot === m.idx ? 'true' : undefined}
            data-up={m.n >= msgs.length - 1 ? 'true' : undefined}
            onMouseEnter={() => onHot(m.idx)}
            onMouseLeave={() => onHot(null)}
          >
            <div className="msg-arrow" data-from={m.a} data-to={m.b}>
              <span className="msg-label">
                <span className="msg-num">{m.n}</span>
                {/* Nowrap by design; .msg-label ellipsises what does not fit. */}
                <span title={`${m.kw} ${m.text}`}>{clip(m.label, 48)}</span>
              </span>
              <span className="msg-line" />
            </div>
            {hot === m.idx && <div className="peek-anchor"><Peek msg={m} /></div>}
          </div>
        ))}
      </div>

      <p className="muted" style={{ fontSize: 11, marginTop: 8 }}>
        Derived from the scenario&apos;s steps. Hover a line to see the step and its payload.
      </p>
    </div>
  )
}
