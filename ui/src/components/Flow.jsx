import { ResourceIcon } from '../lib/icons'
import { clip } from '../lib/format'

export function Flow({ model, cursor, hot, pinned, onHot, onPin }) {
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
            data-hot={hot === m.idx || pinned === m.idx ? 'true' : undefined}
            data-pinned={pinned === m.idx ? 'true' : undefined}
            onMouseEnter={() => onHot(m.idx)}
            onMouseLeave={() => onHot(null)}
            onClick={() => onPin(m.idx)}
          >
            <div className="msg-arrow" data-from={m.a} data-to={m.b}>
              <span className="msg-label">
                <span className="msg-num">{m.n}</span>
                {/* Nowrap by design; .msg-label ellipsises what does not fit. */}
                <span title={`${m.kw} ${m.text}`}>{clip(m.label, 48)}</span>
              </span>
              <span className="msg-line" />
            </div>
          </div>
        ))}
      </div>

      <p className="muted" style={{ fontSize: 11, marginTop: 8 }}>
        Derived from the scenario&apos;s steps. Hover a line for its detail below, click to keep it.
      </p>
    </div>
  )
}
