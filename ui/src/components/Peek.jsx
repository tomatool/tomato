import { fmtDur } from '../lib/format'

// The exact thing a step did, as far as tomato knows it: the step and its
// payload, not bytes captured on the wire.
export function Peek({ msg }) {
  if (!msg) return null
  return (
    <div className="peek" data-status={msg.status || undefined}>
      <div className="peek-head">
        <span className="msg-num">{msg.n}</span>
        <span className="peek-verb">{msg.verb}</span>
        <span className="peek-target">{msg.target}</span>
      </div>
      <div className="peek-kv">
        <span className="peek-k">step</span>
        <span className="peek-v">{`${msg.kw} ${msg.text}`}</span>
        {msg.dur != null && (
          <>
            <span className="peek-k">took</span>
            <span className="peek-v">{fmtDur(msg.dur)}</span>
          </>
        )}
      </div>
      {msg.body && <pre className="docstring peek-body">{msg.body}</pre>}
      <div className="peek-foot">
        <i className="pip" data-status={msg.status} />
        <span>{msg.err || (msg.status ? msg.status : 'not run yet')}</span>
      </div>
    </div>
  )
}
