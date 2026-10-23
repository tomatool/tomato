import { fmtBytes, when } from '../lib/format'

export function Runs({ runs, openRun, onOpenRun, onOpenLog, onWatch }) {
  if (!runs.length) {
    return (
      <div className="empty">
        <h3>No runs yet</h3>
        <p>Runs are written to <code>.tomato/runs</code>. Press <span className="kbd">R</span> to start one.</p>
      </div>
    )
  }
  return (
    <div className="runs">
      {runs.map((r) => {
        const selected = openRun === r.name
        return (
          <div
            className="run"
            key={r.name}
            aria-selected={selected ? 'true' : undefined}
            onClick={() => onOpenRun(selected ? null : r.name)}
          >
            <i className="pip" data-status={r.status ?? ''} />
            <div>
              <div className="run-id">{String(r.name).split('_').pop()}</div>
              <div className="run-when">{when(r.timestamp)}</div>
            </div>
            <span className="tally" />
            {selected && r.logs?.length > 0 && (
              <div className="run-logs">
                {onWatch && (
                  <button
                    className="log-link" type="button" title="Watch this run again as a film"
                    onClick={(e) => { e.stopPropagation(); onWatch(r.name) }}
                  >
                    <i className="ico-play" />watch
                  </button>
                )}
                {r.logs.map((l) => (
                  <button
                    className="log-link"
                    type="button"
                    key={l.name}
                    onClick={(e) => { e.stopPropagation(); onOpenLog(r.name, l.name, l.absPath) }}
                  >
                    {l.name} <small>{fmtBytes(l.size)}</small>
                  </button>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
